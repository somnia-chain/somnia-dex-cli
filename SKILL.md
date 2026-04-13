# DreamDEX CLI — LLM Skill Reference

You are interacting with `dreamdex`, a non-custodial trading CLI for DreamDEX on the Somnia blockchain. This document describes every command, its arguments, and expected outputs so you can use the CLI effectively.

## Prerequisites

- Keys are stored in an encrypted keystore (`~/.config/dreamdex/keystore/`). Run `dreamdex login` to import a key.
- For headless/MCP use, set `DREAMDEX_PRIVATE_KEY` (hex-encoded, with or without `0x` prefix) to bypass the keystore, or set `DREAMDEX_PASSWORD` to unlock it non-interactively.
- All commands support `--json` for structured JSON output. Always use `--json` when you need to parse results programmatically.
- Commands that accept an optional `[symbol]` default to all markets when omitted. Symbols look like `SOMI:SOMUSD`, `WETH:SOMUSD`, `WBTC:SOMUSD`.

## Concepts

### Markets

A market is a trading pair (e.g. `SOMI:SOMUSD`) representing the base token traded against a quote currency. Each market has its own order book, tick size (minimum price increment), lot size (minimum quantity increment), and minimum order quantity. Use `dreamdex markets` to discover available pairs and their parameters before trading.

### Orders

Orders are instructions to buy or sell tokens at a given price. DreamDEX supports two core order types:

- **Market orders** execute immediately at the best available price. They are implemented as limit IOC (immediate-or-cancel) orders priced from the current order book with a slippage tolerance (default 0.5%). Use these when you want to trade now and care more about speed than exact price.
- **Limit orders** sit on the order book at a specified price and wait to be filled. Use these when you want to trade at a specific price or better. Sub-types control execution behaviour:
  - `normalOrder` (default) - rests on the book until filled, cancelled, or expired.
  - `fillOrKill` - must fill entirely in one match or the whole order is cancelled.
  - `immediateOrCancel` - fills as much as possible immediately, cancels the remainder.
  - `postOnly` - only accepted if it would rest on the book (no immediate fill); useful for earning maker rebates.

All orders are settled on-chain on the Somnia blockchain. The CLI handles signing, token approval, and transaction submission automatically.

### Stop orders

Stop orders are conditional orders that activate only when the market price crosses a trigger threshold. They are useful for:

- **Stop-loss** - automatically sell if the price drops to a certain level, limiting downside risk. Example: you hold SOMI and want to sell if it falls below $0.15 - place a stop sell with `--trigger-price 0.15 --trigger-operator lte`.
- **Breakout entry** - automatically buy if the price rises above resistance. Example: buy SOMI if it breaks above $0.25 - place a stop buy with `--trigger-price 0.25 --trigger-operator gte`.

Once triggered, the stop order becomes a regular order (market or limit) and is submitted on-chain. A stop order can end up in one of four states: `pending` (waiting for trigger), `triggered` (activated and submitted), `cancelled` (manually cancelled before triggering), or `failed` (triggered but the resulting order failed to submit).

### Vault

The vault is an on-chain escrow that holds tokens earmarked for trading. Depositing into the vault pre-funds your account so that subsequent orders can settle faster without requiring a fresh wallet transfer each time. The workflow is:

1. **Approve** - grant the vault contract permission to transfer a token (one-time per token).
2. **Deposit** - move tokens from your wallet into the vault.
3. **Trade** - place orders with `--funding-source vault` to draw from your vault balance.
4. **Withdraw** - move tokens back to your wallet when done.

Using the vault is optional - orders default to `--funding-source wallet`, which transfers directly from your wallet for each trade.

## Commands

### Market data (read-only, no auth required)

#### `dreamdex markets [--json]`
List all trading pairs. Returns symbol, contract address, base/quote tokens, tick size, lot size, and minimum quantity.

#### `dreamdex currencies [--json]`
List all supported currencies. Returns code, name, decimals, and token address.

#### `dreamdex orderbook [symbol] [--depth N] [--json]`
Show the order book. `--depth` limits the number of price levels per side.

#### `dreamdex ticker [symbol] [--json]`
Show 24-hour OHLCV market statistics.

#### `dreamdex trades [symbol] [--limit N] [--json]`
Show recent trades. `--limit` controls the number of results (default 20).

#### `dreamdex candles [symbol] [--interval 1m|5m|15m|1h|4h|1d] [--limit N] [--json]`
Show OHLCV candle data. Default interval is `1h`, default limit is 20.

### Authentication

#### `dreamdex login`
Import a private key into the encrypted keystore and authenticate. On first run with `DREAMDEX_PRIVATE_KEY` set, prompts for a passphrase and stores the encrypted key. Subsequent runs authenticate using the keystore.

### Orders (auth required)

#### `dreamdex order place <symbol> --side buy|sell --amount <amount> [--type market|limit] [--price <price>] [--order-type normalOrder|fillOrKill|immediateOrCancel|postOnly] [--funding-source wallet|vault] [--slippage <percent>] [--wait] [--json]`
Place a new order. `--side` and `--amount` are required. `--price` is required for limit orders. `--type` defaults to `market`. Market orders are sent as limit IOC orders with an orderbook-derived price; `--slippage` controls the tolerance (default 0.5%). `--wait` blocks until the transaction is confirmed on-chain.

If token approval is needed, the CLI automatically submits the approval transaction before the order.

#### `dreamdex order list [symbol] [--status open|closed|canceled|expired|rejected] [--json]`
List orders. Optionally filter by symbol and/or status.

#### `dreamdex order get <symbol> <order-id> [--json]`
Get details for a single order.

#### `dreamdex order cancel <symbol> <order-id> [--wait] [--json]`
Cancel an open order. Signs and sends a cancellation transaction on-chain.

#### `dreamdex order reduce <symbol> <order-id> --quantity <new-remaining> [--wait] [--json]`
Reduce an open order's remaining quantity. Signs and sends a transaction.

### Stop orders (auth required)

#### `dreamdex stop-order place <symbol> --side buy|sell --amount <amount> --trigger-price <price> --trigger-operator gte|lte [--type market|limit] [--price <price>] [--wait] [--json]`
Place a conditional stop order. `--side`, `--amount`, `--trigger-price`, and `--trigger-operator` are required. `--type` defaults to `market`. `--price` is required for limit stop orders. The order activates when the market price crosses the trigger.

#### `dreamdex stop-order list [symbol] [--status pending|triggered|cancelled|failed] [--json]`
List stop orders. Optionally filter by symbol and/or status.

#### `dreamdex stop-order cancel <symbol> <id> [--wait] [--json]`
Cancel a pending stop order. Signs and sends a cancellation transaction on-chain.

### Vault (auth required)

#### `dreamdex vault balance [symbol] [--wallet <address>] [--json]`
Show vault balances. If `--wallet` is omitted, derives the address from the keystore or `DREAMDEX_PRIVATE_KEY`.

#### `dreamdex vault approve <symbol> --currency <code> --amount <amount> [--wait] [--json]`
Approve token spending for vault deposits. Required before the first deposit of a token.

#### `dreamdex vault deposit <symbol> --currency <code> --amount <amount> [--wait] [--json]`
Deposit tokens into the vault.

#### `dreamdex vault withdraw <symbol> --currency <code> --amount <amount> [--wait] [--json]`
Withdraw tokens from the vault.

### Live streaming (WebSocket)

#### `dreamdex watch orderbook [symbol] [--json]`
Stream live order book updates.

#### `dreamdex watch trades [symbol] [--json]`
Stream live trade executions.

#### `dreamdex watch candles [symbol] [--interval 1m|5m|15m|1h|4h|1d] [--json]`
Stream live OHLCV candle updates. Default interval is `1m`.

#### `dreamdex watch order <order-id> [--json]`
Watch a specific order for status changes.

All `watch` commands support `--timeout <duration>` (e.g. `30s`, `5m`) to auto-terminate after a duration.

## Global flags

| Flag | Description |
|---|---|
| `--json` | Output structured JSON instead of human-readable tables |
| `--log-level` | Log verbosity: `debug`, `info`, `warn` (default), `error` |
| `--api-url` | Override the API base URL |
| `--rpc-url` | Override the Somnia RPC URL |

## Common workflows

### Check prices and place a market buy

```sh
dreamdex ticker SOMI:SOMUSD --json
dreamdex order place SOMI:SOMUSD --side buy --amount 100 --wait
```

### Place a limit sell order

```sh
dreamdex order place SOMI:SOMUSD --side sell --type limit --amount 50 --price 0.20 --wait
```

### Deposit to vault and trade from vault

```sh
dreamdex vault approve SOMI:SOMUSD --currency SOMUSD --amount 1000 --wait
dreamdex vault deposit SOMI:SOMUSD --currency SOMUSD --amount 500 --wait
dreamdex order place SOMI:SOMUSD --side buy --amount 100 --funding-source vault --wait
```

### Monitor and cancel an order

```sh
dreamdex order list SOMI:SOMUSD --status open --json
dreamdex order cancel SOMI:SOMUSD <order-id> --wait
```

### Stream live trades

```sh
dreamdex watch trades SOMI:SOMUSD --timeout 5m
```
