package main

import (
	"context"
	"log"
	"os"
	"time"

	sdk "github.com/kdraigo/flow_v1/dev_sdk"
	"github.com/kdraigo/flow_v1/dev_sdk/types"
)

const (
	exchange   = "binance"
	pair       = "BTC/USDT"
	quoteAsset = "USDT"
	rsiPeriod  = 14
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
			SessionName:        "risk-managed-template",
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

	risk := NewRiskManager(RiskParams{
		MaxRiskPerTradePct: 0.01, // risk 1% of equity per trade
		StopLossPct:        0.02, // 2% stop loss
		TakeProfitPct:      0.04, // 4% take profit
		InitialEquity:      10000,
	})

	s.SetOnCandleFor(types.Timeframe1h, func(ctx *types.Context, c *types.Candle) {
		rsi, err := s.IndicatorManagerFor(types.Timeframe1h).RSI(exchange, pair, "close", rsiPeriod)
		if err != nil || len(rsi) == 0 {
			return
		}
		latest := rsi[len(rsi)-1]
		signal := classify(latest)
		risk.OnSignal(ctx, signal, c)
	})

	s.SetOnOrderUpdate(func(ctx *types.Context, o *types.Order) {
		risk.OnOrderUpdate(ctx, o)
	})

	s.SetOnComplete(func() {
		risk.PrintSummary()
		log.Println("[done] backtest finished")
	})

	if err := s.Start(context.Background()); err != nil {
		log.Fatalf("sdk.Start: %v", err)
	}
}

type Signal int

const (
	SignalNone Signal = iota
	SignalLong
	SignalExit
)

func classify(rsi float64) Signal {
	switch {
	case rsi < 30:
		return SignalLong
	case rsi > 70:
		return SignalExit
	default:
		return SignalNone
	}
}
