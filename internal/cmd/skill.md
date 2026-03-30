# DreamDEX CLI — LLM Skill Reference

You are interacting with `dreamdex`, a non-custodial trading CLI for DreamDEX on the Somnia blockchain. This document describes every command, its arguments, and expected outputs so you can use the CLI effectively.

## Prerequisites

- `DREAMDEX_PRIVATE_KEY` must be set as a hex-encoded Ethereum private key (with or without `0x` prefix). Commands that require signing will auto-authenticate using this key.
- All commands support `--json` for structured JSON output. Always use `--json` when you need to parse results programmatically.
- Commands that accept an optional `[symbol]` default to all markets when omitted. Symbols look like `SOMI:SOMUSD`, `WETH:SOMUSD`, `WBTC:SOMUSD`.

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
Authenticate via Sign-In with Ethereum and cache the JWT token to disk. Other commands auto-authenticate in memory when `DREAMDEX_PRIVATE_KEY` is set, so explicit login is rarely needed.

### Orders (auth required)

#### `dreamdex order place <symbol> --side buy|sell --amount <amount> [--type market|limit] [--price <price>] [--order-type normalOrder|fillOrKill|immediateOrCancel|postOnly] [--funding-source wallet|vault] [--wait] [--json]`
Place a new order. `--side` and `--amount` are required. `--price` is required for limit orders. `--type` defaults to `market`. `--wait` blocks until the transaction is confirmed on-chain.

If token approval is needed, the CLI will print instructions instead of submitting the order. Run `dreamdex vault approve` first, then retry.

#### `dreamdex order list [symbol] [--status open|closed|canceled|expired|rejected] [--json]`
List orders. Optionally filter by symbol and/or status.

#### `dreamdex order get <symbol> <order-id> [--json]`
Get details for a single order.

#### `dreamdex order cancel <symbol> <order-id> [--json]`
Cancel an open order. This is an API-side cancellation (no transaction signing).

#### `dreamdex order reduce <symbol> <order-id> --quantity <new-remaining> [--wait] [--json]`
Reduce an open order's remaining quantity. Signs and sends a transaction.

### Vault (auth required)

#### `dreamdex vault balance [symbol] [--wallet <address>] [--json]`
Show vault balances. If `--wallet` is omitted, derives the address from `DREAMDEX_PRIVATE_KEY`.

#### `dreamdex vault approve <symbol> --currency <code> --amount <amount> [--wait] [--json]`
Approve token spending for vault deposits. Required before the first deposit of a token.

#### `dreamdex vault deposit <symbol> --currency <code> --amount <amount> [--wait] [--json]`
Deposit tokens into the vault.

#### `dreamdex vault withdraw <symbol> --currency <code> --amount <amount> [--wait] [--json]`
Withdraw tokens from the vault.

## Global flags

| Flag | Description |
|---|---|
| `--json` | Output structured JSON instead of human-readable tables |
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
dreamdex order cancel SOMI:SOMUSD <order-id>
```
