# example_strategy

Starter templates for the kdraigo trading platform. Cloned by `kdraigo-mcp` at scaffold time.

## Templates

| Name | What you get |
|------|--------------|
| [`basic/`](basic) | Single `main.go` that prints each closed candle. No orders, no indicators. Use this when you want a clean slate to write your own logic. |
| [`with-indicators/`](with-indicators) | RSI-based market-buy / market-sell. Reads `OnCandleFor(1h)`, places one order per signal. Use this when your strategy is indicator-driven. |
| [`with-risk-manager/`](with-risk-manager) | Two-file layout (`main.go` + `risk.go`) with a separate signal layer and a stateful position/PnL manager. Use this when you need stop-loss, take-profit, or position sizing. |

## Configuration

Every template reads two env vars at startup:

```bash
export KDRAIGO_KEY_ID=<your-ed25519-key-id>
export KDRAIGO_PRIVATE_KEY=<hex-encoded-ed25519-private-key>
```

Register a key via the kdraigo dashboard or `users_service/cmd/test_client`.

## Running a template

```bash
cd <template-dir>
go mod tidy
go run .
```

By default templates run against `http://localhost:4000` (a local backtester_engine).
To target the hosted platform, set `KDRAIGO_BACKTESTER_URL=https://kdraigo.com/backtester`.

## Date range

Templates default to a 30-day backtest ending today. Edit `startTime` / `endTime` in `main.go`.
