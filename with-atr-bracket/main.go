package main

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"

	sdk "github.com/kdraigo/dev_sdk"
	"github.com/kdraigo/dev_sdk/params"
	"github.com/kdraigo/dev_sdk/types"
)

// The run's settings, declared with dev_sdk/params: the website shows them as
// a form, and the backtest can be run again with other values without
// changing the code. Names, defaults and options stay literals so the
// platform can read them.
var (
	exchange = params.String("EXCHANGE", "binance", params.Help("exchange to backtest on"))
	pair     = params.String("PAIR", "BTC/USDT", params.Help("pair to trade, BASE/QUOTE"))
	capital  = params.Float("CAPITAL", 10000, params.Range(100, 10000000), params.Help("starting balance in the quote asset"))
	start    = params.Date("BT_START", "today-90d", params.Help("first day of the backtest (UTC)"))
	end      = params.Date("BT_END", "today", params.Help("day the backtest stops (UTC)"))

	fastEMA      = params.Int("EMA_FAST", 20, params.Range(2, 200), params.Help("fast EMA length, in bars"))
	slowEMA      = params.Int("EMA_SLOW", 50, params.Range(3, 400), params.Help("slow EMA length, in bars"))
	atrPeriod    = params.Int("ATR_PERIOD", 14, params.Range(2, 100), params.Help("ATR length, in bars"))
	stopATR      = params.Float("STOP_ATR", 2, params.Range(0.5, 10), params.Help("stop distance below entry, in ATRs"))
	takeATR      = params.Float("TAKE_ATR", 4, params.Range(0.5, 20), params.Help("target distance above entry, in ATRs"))
	riskPerTrade = params.Float("RISK_PER_TRADE", 0.01, params.Range(0.001, 0.05), params.Help("share of the balance lost if the stop fills"))
)

// A long-only trend starter whose exits come from the market's own range:
// the stop sits STOP_ATR average true ranges below the entry and the target
// TAKE_ATR above it, so both widen in volatile markets and tighten in quiet
// ones. The size is what loses RISK_PER_TRADE of the balance at the stop.
// Both exits rest on the venue as one bracket and fill inside the bar, where
// the price got to.
func main() {
	params.Log()
	keyID := os.Getenv("KDRAIGO_KEY_ID")
	privateKey := os.Getenv("KDRAIGO_PRIVATE_KEY")
	if keyID == "" || privateKey == "" {
		log.Fatal("KDRAIGO_KEY_ID and KDRAIGO_PRIVATE_KEY must be set")
	}

	endpoint := os.Getenv("KDRAIGO_BACKTESTER_URL")
	if endpoint == "" {
		endpoint = "http://localhost:4000"
	}

	cfg := &types.Config{
		Environment: types.EnvBacktest,
		Timeframes:  []types.Timeframe{types.Timeframe1h},
		Credentials: types.Credentials{KeyID: keyID, PrivateKey: privateKey},
		Backtest: &types.BacktestOptions{
			Endpoint:           endpoint,
			SessionName:        "atr-bracket-template",
			RequestedExchanges: []string{exchange},
			Assets:             []string{pair},
			Wallets:            map[string]float64{quoteOf(pair): capital},
			StartTime:          start,
			EndTime:            end,
		},
	}

	s, err := sdk.New(cfg)
	if err != nil {
		log.Fatalf("sdk.New: %v", err)
	}

	// The open position's bracket legs; empty when flat. Order updates arrive
	// on their own goroutine, so the state is shared under a lock.
	var (
		mu             sync.Mutex
		long           bool
		stopID, takeID string
		wasAbove       bool
		seen           bool
	)

	s.SetOnCandleFor(types.Timeframe1h, func(ctx *types.Context, c *types.Candle) {
		im := s.IndicatorManagerFor(types.Timeframe1h)
		fast, err1 := im.EMA(exchange, pair, "close", fastEMA)
		slow, err2 := im.EMA(exchange, pair, "close", slowEMA)
		atr, err3 := im.ATR(exchange, pair, atrPeriod)
		if err1 != nil || err2 != nil || err3 != nil || len(fast) == 0 || len(slow) == 0 || len(atr) == 0 {
			return // not enough data yet
		}
		above := fast[len(fast)-1] > slow[len(slow)-1]
		crossedUp := seen && above && !wasAbove
		wasAbove, seen = above, true

		mu.Lock()
		defer mu.Unlock()
		if long || !crossedUp {
			return
		}
		lastATR := atr[len(atr)-1]
		if lastATR <= 0 {
			return
		}

		balance, err := quoteBalance(ctx.Ctx, s)
		if err != nil {
			log.Printf("[atr] balance: %v", err)
			return
		}
		stopDist := stopATR * lastATR
		qty := riskPerTrade * balance / stopDist
		// A tight stop sizes past what the balance can buy: cap at most of
		// it, leaving room for the fee.
		if most := 0.98 * balance / c.Close; qty > most {
			qty = most
		}

		entry, err := ctx.PlaceOrder(&types.OrderRequest{
			Exchange: exchange,
			Symbol:   pair,
			Side:     types.OrderSideBuy,
			Type:     types.OrderTypeMarket,
			Quantity: qty,
			Reason:   map[string]any{"signal": "ema_cross_up", "atr": lastATR, "stop_dist": stopDist},
		})
		if err != nil {
			log.Printf("[atr] entry: %v", err)
			return
		}
		price, filled := c.Close, qty
		if entry.AveragePrice > 0 {
			price = entry.AveragePrice
		}
		if entry.FilledQty > 0 {
			filled = entry.FilledQty
		}
		long = true

		stop, take := price-stopDist, price+takeATR*lastATR
		legs, err := s.PlaceBracket(ctx.Ctx, &types.BracketRequest{
			Exchange:        exchange,
			Symbol:          pair,
			Side:            types.OrderSideSell,
			Quantity:        filled,
			TakeProfitPrice: take,
			StopPrice:       stop,
			Reason:          map[string]any{"signal": "atr_bracket", "stop": stop, "take": take, "atr": lastATR},
		})
		if err != nil {
			// Never hold a position nobody protects.
			log.Printf("[atr] bracket refused (%v): closing the position", err)
			if _, err := ctx.PlaceOrder(&types.OrderRequest{
				Exchange: exchange, Symbol: pair, Side: types.OrderSideSell, Type: types.OrderTypeMarket,
				Quantity: filled, Reason: map[string]any{"signal": "no_bracket"},
			}); err != nil {
				log.Printf("[atr] close: %v", err)
			}
			long = false
			return
		}
		// Tell the legs apart by the stop: the take-profit's type differs
		// between venues (LIMIT_MAKER in backtests).
		for _, o := range legs {
			if o.Type == types.OrderTypeStopLoss || o.Type == types.OrderTypeStopLossLimit {
				stopID = o.ID
			} else {
				takeID = o.ID
			}
		}
		log.Printf("[atr] long %.6f at %.2f: stop %.2f, target %.2f (ATR %.2f)", filled, price, stop, take, lastATR)
	})

	s.SetOnOrderUpdate(func(_ *types.Context, o *types.Order) {
		mu.Lock()
		defer mu.Unlock()
		// Either leg filling closes the position; the venue cancels the other.
		if !long || o.Status != types.OrderStatusFilled || (o.ID != stopID && o.ID != takeID) {
			return
		}
		leg := "target"
		if o.ID == stopID {
			leg = "stop"
		}
		log.Printf("[atr] %s filled at %.2f", leg, o.AveragePrice)
		long, stopID, takeID = false, "", ""
	})

	s.SetOnComplete(func() {
		log.Println("[done] backtest finished")
	})

	if err := s.Start(context.Background()); err != nil {
		log.Fatalf("sdk.Start: %v", err)
	}
}

// quoteBalance is the free balance of the pair's quote asset.
func quoteBalance(ctx context.Context, s *sdk.SDK) (float64, error) {
	quote := quoteOf(pair)
	acc, err := s.GetAccount(ctx, exchange, quote)
	if err != nil {
		return 0, err
	}
	for _, b := range acc.Balances {
		if b.Asset == quote {
			return b.Free, nil
		}
	}
	return 0, nil
}

// quoteOf is the quote asset of a BASE/QUOTE pair: the wallet's currency.
func quoteOf(pair string) string {
	if i := strings.LastIndex(pair, "/"); i >= 0 {
		return pair[i+1:]
	}
	return "USDT"
}
