# example_strategy

Starter templates for the kdraigo trading platform. Cloned by `kdraigo-mcp` at scaffold time.

## Templates

| Name | What you get |
|------|--------------|
| [`basic/`](basic) | Single `main.go` that prints each closed candle. No orders, no indicators. Use this when you want a clean slate to write your own logic. |
| [`with-indicators/`](with-indicators) | RSI-based market-buy / market-sell. Reads `OnCandleFor(1h)`, places one order per signal. Use this when your strategy is indicator-driven. |
| [`with-risk-manager/`](with-risk-manager) | Two-file layout (`main.go` + `risk.go`) with a separate signal layer and a stateful position/PnL manager. Use this when you need stop-loss, take-profit, or position sizing. |
| [`with-atr-bracket/`](with-atr-bracket) | EMA-cross longs whose stop and target are multiples of ATR, resting on the venue as one bracket; size is what loses `RISK_PER_TRADE` of the balance at the stop. On dev_sdk v1.8.1, with its settings declared through `dev_sdk/params` (override any of them with an env var of the same name). Use this for volatility-scaled exits. |

## Configuration

Every template reads two env vars at startup:

```bash
export KDRAIGO_KEY_ID=<your-ed25519-key-id>
export KDRAIGO_PRIVATE_KEY=<hex-encoded-ed25519-private-key>
```

Create a key on kdraigo.com under Account → API keys ([kdraigo.com/account/api-keys](https://kdraigo.com/account/api-keys)).

## Running a template

```bash
cd <template-dir>
go mod tidy
go run .
```

By default templates run against `http://localhost:4000` (a local backtester_engine).
To target the hosted platform, set `KDRAIGO_BACKTESTER_URL=https://api.kdraigo.com`.

## Date range

`basic`, `with-indicators` and `with-risk-manager` default to a 30-day backtest ending today: edit `startTime` / `endTime` in `main.go`. `with-atr-bracket` declares `BT_START` and `BT_END` (default: the last 90 days): set them as env vars, e.g. `BT_START=2025-01-01`.
