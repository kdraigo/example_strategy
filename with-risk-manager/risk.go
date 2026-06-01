package main

import (
	"log"

	"github.com/kdraigo/flow_v1/dev_sdk/types"
)

type RiskParams struct {
	MaxRiskPerTradePct float64
	StopLossPct        float64
	TakeProfitPct      float64
	InitialEquity      float64
}

type position struct {
	entryPrice float64
	quantity   float64
	stop       float64
	take       float64
}

type RiskManager struct {
	params RiskParams
	equity float64
	pos    *position
	wins   int
	losses int
}

func NewRiskManager(p RiskParams) *RiskManager {
	return &RiskManager{params: p, equity: p.InitialEquity}
}

func (r *RiskManager) OnSignal(ctx *types.Context, sig Signal, c *types.Candle) {
	switch {
	case sig == SignalLong && r.pos == nil:
		r.openLong(ctx, c)
	case sig == SignalExit && r.pos != nil:
		r.closePos(ctx, c.Close, "signal_exit")
	case r.pos != nil:
		r.checkStops(ctx, c)
	}
}

func (r *RiskManager) openLong(ctx *types.Context, c *types.Candle) {
	stopPrice := c.Close * (1 - r.params.StopLossPct)
	takePrice := c.Close * (1 + r.params.TakeProfitPct)
	riskAmount := r.equity * r.params.MaxRiskPerTradePct
	perUnitRisk := c.Close - stopPrice
	if perUnitRisk <= 0 {
		return
	}
	qty := riskAmount / perUnitRisk

	_, err := ctx.PlaceOrder(ctx.Ctx, &types.OrderRequest{
		Exchange: exchange,
		Asset:    quoteAsset,
		Pair:     pair,
		Side:     types.OrderSideBuy,
		Type:     types.OrderTypeMarket,
		Quantity: qty,
		Reason: map[string]any{
			"signal":    "long_entry",
			"stop":      stopPrice,
			"take":      takePrice,
			"risk_usd":  riskAmount,
			"qty":       qty,
			"entry":    c.Close,
		},
	})
	if err != nil {
		log.Printf("[risk] open long: %v", err)
		return
	}
	r.pos = &position{entryPrice: c.Close, quantity: qty, stop: stopPrice, take: takePrice}
}

func (r *RiskManager) checkStops(ctx *types.Context, c *types.Candle) {
	if r.pos == nil {
		return
	}
	if c.Low <= r.pos.stop {
		r.closePos(ctx, r.pos.stop, "stop_loss")
	} else if c.High >= r.pos.take {
		r.closePos(ctx, r.pos.take, "take_profit")
	}
}

func (r *RiskManager) closePos(ctx *types.Context, exitPrice float64, reason string) {
	if r.pos == nil {
		return
	}
	pnl := (exitPrice - r.pos.entryPrice) * r.pos.quantity
	r.equity += pnl
	if pnl >= 0 {
		r.wins++
	} else {
		r.losses++
	}

	_, err := ctx.PlaceOrder(ctx.Ctx, &types.OrderRequest{
		Exchange: exchange,
		Asset:    quoteAsset,
		Pair:     pair,
		Side:     types.OrderSideSell,
		Type:     types.OrderTypeMarket,
		Quantity: r.pos.quantity,
		Reason: map[string]any{
			"signal":   reason,
			"entry":    r.pos.entryPrice,
			"exit":     exitPrice,
			"pnl_usd":  pnl,
			"equity":   r.equity,
		},
	})
	if err != nil {
		log.Printf("[risk] close pos: %v", err)
	}
	r.pos = nil
}

func (r *RiskManager) OnOrderUpdate(_ *types.Context, _ *types.Order) {
	// hook for future use (slippage tracking, partial fills)
}

func (r *RiskManager) PrintSummary() {
	total := r.wins + r.losses
	winRate := 0.0
	if total > 0 {
		winRate = float64(r.wins) / float64(total)
	}
	log.Printf("[summary] equity=%.2f trades=%d wins=%d losses=%d win_rate=%.2f%%",
		r.equity, total, r.wins, r.losses, winRate*100)
}
