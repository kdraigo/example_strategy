package main

import (
	"context"
	"log"
	"os"
	"time"

	sdk "github.com/kdraigo/dev_sdk"
	"github.com/kdraigo/dev_sdk/types"
)

const (
	exchange    = "binance"
	pair        = "BTC/USDT"
	quoteAsset  = "USDT"
	rsiPeriod   = 14
	rsiOverbuy  = 70.0
	rsiOversell = 30.0
	tradeQty    = 0.01
)

func main() {
	keyID := os.Getenv("KDRAIGO_KEY_ID")
	privateKey := os.Getenv("KDRAIGO_PRIVATE_KEY")
	if keyID == "" || privateKey == "" {
		log.Fatal("KDRAIGO_KEY_ID and KDRAIGO_PRIVATE_KEY must be set")
	}

	endpoint := os.Getenv("KDRAIGO_BACKTESTER_URL")
	if endpoint == "" {
		endpoint = "http://localhost:4000"
	}

	endTime := time.Now().UTC()
	startTime := endTime.AddDate(0, 0, -30)

	cfg := &types.Config{
		Environment: types.EnvBacktest,
		Timeframes:  []types.Timeframe{types.Timeframe1h},
		Credentials: types.Credentials{KeyID: keyID, PrivateKey: privateKey},
		Backtest: &types.BacktestOptions{
			Endpoint:           endpoint,
			SessionName:        "rsi-template",
			RequestedExchanges: []string{exchange},
			Assets:             []string{pair},
			Wallets:            map[string]float64{quoteAsset: 10000},
			StartTime:          startTime,
			EndTime:            endTime,
		},
	}

	s, err := sdk.New(cfg)
	if err != nil {
		log.Fatalf("sdk.New: %v", err)
	}

	// Track whether we're currently long so we don't re-buy.
	long := false

	s.SetOnCandleFor(types.Timeframe1h, func(ctx *types.Context, c *types.Candle) {
		rsi, err := s.IndicatorManagerFor(types.Timeframe1h).RSI(exchange, pair, "close", rsiPeriod)
		if err != nil {
			return // not enough data yet
		}
		if len(rsi) == 0 {
			return
		}
		latest := rsi[len(rsi)-1]

		switch {
		case !long && latest < rsiOversell:
			placeOrder(ctx, types.OrderSideBuy, c.Close, latest, "rsi_oversold")
			long = true
		case long && latest > rsiOverbuy:
			placeOrder(ctx, types.OrderSideSell, c.Close, latest, "rsi_overbought")
			long = false
		}
	})

	s.SetOnOrderUpdate(func(_ *types.Context, o *types.Order) {
		log.Printf("[order] %s %s status=%s qty=%.4f price=%.2f", o.Side, o.Type, o.Status, o.FilledQty, o.AveragePrice)
	})

	s.SetOnComplete(func() {
		log.Println("[done] backtest finished")
	})

	if err := s.Start(context.Background()); err != nil {
		log.Fatalf("sdk.Start: %v", err)
	}
}

func placeOrder(ctx *types.Context, side types.OrderSide, price, rsi float64, reason string) {
	_, err := ctx.PlaceOrder(&types.OrderRequest{
		Exchange: exchange,
		Symbol:   pair,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: tradeQty,
		Reason:   map[string]any{"signal": reason, "rsi": rsi, "close": price},
	})
	if err != nil {
		log.Printf("[err] place %s: %v", side, err)
	}
}
