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
	exchange = "binance"
	pair     = "BTC/USDT"
	asset    = "USDT"
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
			SessionName:        "basic-template",
			RequestedExchanges: []string{exchange},
			Assets:             []string{pair},
			Wallets:            map[string]float64{asset: 10000},
			StartTime:          startTime,
			EndTime:            endTime,
		},
	}

	s, err := sdk.New(cfg)
	if err != nil {
		log.Fatalf("sdk.New: %v", err)
	}

	s.SetOnCandle(func(ctx *types.Context, c *types.Candle) {
		log.Printf("[candle] %s %s %s O=%.2f H=%.2f L=%.2f C=%.2f V=%.4f",
			c.CloseTime.Format(time.RFC3339), c.Symbol, c.Timeframe,
			c.Open, c.High, c.Low, c.Close, c.Volume)
	})

	s.SetOnComplete(func() {
		log.Println("[done] backtest finished")
	})

	if err := s.Start(context.Background()); err != nil {
		log.Fatalf("sdk.Start: %v", err)
	}
}
