# DreamDEX CLI - LLM Skill Reference

You are interacting with `dreamdex`, a non-custodial trading CLI for DreamDEX on the Somnia blockchain. This document describes every command, its arguments, and expected outputs so you can use the CLI effectively.

## Prerequisites

- Keys are stored in an encrypted keystore (`~/.config/dreamdex/keystore/`). Run `dreamdex login` to import a key.
- For headless/MCP use, set `DREAMDEX_PRIVATE_KEY` (hex-encoded, with or without `0x` prefix) to bypass the keystore, or set `DREAMDEX_PASSWORD` to unlock it non-interactively.
- All commands support `--json` for structured JSON output. Always use `--json` when you need to parse results programmatically.
- Commands that accept an optional `[symbol]` default to all markets when omitted. Symbols look like `SOMI:SOMUSD`, `WETH:SOMUSD`, `WBTC:SOMUSD`.
- All write commands (order place/cancel/reduce, stoporder place/cancel, vault deposit/withdraw/approve) always wait for on-chain confirmation before returning.

## Concepts

**Markets** - A trading pair (e.g. `SOMI:SOMUSD`) with its own order book, tick size, lot size, and minimum quantity. Use `dreamdex markets` to discover available pairs.

**Orders** - Instructions to buy or sell tokens. Market orders execute immediately as limit IOC orders priced from the order book with a slippage tolerance (default 0.5%). Limit orders rest on the book at a specified price. Sub-types: `normalOrder` (default, rests until filled), `fillOrKill` (fill entirely or cancel), `immediateOrCancel` (fill what you can, cancel rest), `postOnly` (only accepted if it rests on the book).

**Stop orders** - Conditional orders that activate when the market price crosses a trigger. Use `--trigger-operator lte` for stop-loss (sell when price drops) or `--trigger-operator gte` for breakout entry (buy when price rises). States: `pending`, `triggered`, `cancelled`, `failed`.

**Vault** - On-chain escrow for pre-funding trades. Workflow: approve -> deposit -> trade with `--funding-source vault` -> withdraw. Optional - orders default to `--funding-source wallet`.

## Commands

### Market data (no auth required)

- `dreamdex markets` - List all trading pairs with tick/lot sizes and minimums.
- `dreamdex currencies` - List supported currencies with code, name, decimals, address.
- `dreamdex orderbook <symbol> [--depth N]` - Show order book. `--depth` limits levels per side.
- `dreamdex ticker [symbol]` - 24-hour OHLCV statistics.
- `dreamdex trades [symbol] [--limit N]` - Recent trades (default 20).
- `dreamdex candles <symbol> [--interval 1m|5m|15m|1h|4h|1d] [--limit N]` - OHLCV candles (default 1h, 20).

### Authentication

- `dreamdex login` - Import key into keystore and authenticate via SIWE.

### Orders (auth required)

- `dreamdex order place <symbol> --side buy|sell --amount <n> [--type market|limit] [--price <n>] [--order-type normalOrder|fillOrKill|immediateOrCancel|postOnly] [--funding-source wallet|vault] [--slippage <pct>]` - Place an order. `--price` required for limit. Auto-submits token approval if needed.
- `dreamdex order list [symbol] [--status open|closed|canceled|expired|rejected]` - List orders.
- `dreamdex order get <symbol> <order-id>` - Get single order details.
- `dreamdex order cancel <symbol> <order-id>` - Cancel an open order on-chain.
- `dreamdex order reduce <symbol> <order-id> --quantity <new-remaining>` - Reduce remaining quantity on-chain.

### Stop orders (auth required)

- `dreamdex stoporder place <symbol> --side buy|sell --amount <n> --trigger-price <n> --trigger-operator gte|lte [--type market|limit] [--price <n>]` - Place a conditional stop order. `--price` required for limit type.
- `dreamdex stoporder list [symbol] [--status pending|triggered|cancelled|failed]` - List stop orders.
- `dreamdex stoporder cancel <symbol> <id>` - Cancel a pending stop order on-chain.

### Vault (auth required)

- `dreamdex vault balance [symbol] [--wallet <address>]` - Show vault balances. Derives address from key if omitted.
- `dreamdex vault approve <symbol> --currency <code> --amount <n>` - Approve token spending for deposits.
- `dreamdex vault deposit <symbol> --currency <code> --amount <n>` - Deposit tokens into vault.
- `dreamdex vault withdraw <symbol> --currency <code> --amount <n>` - Withdraw tokens from vault.

### Live streaming (WebSocket)

- `dreamdex watch orderbook [symbol]` - Stream order book updates.
- `dreamdex watch trades [symbol]` - Stream trade executions.
- `dreamdex watch candles [symbol] [--interval 1m|5m|15m|1h|4h|1d]` - Stream candle updates (default 1m).
- `dreamdex watch order <order-id>` - Watch a specific order for status changes.

All `watch` commands support `--timeout <duration>` (e.g. `30s`, `5m`) to auto-terminate.

## Global flags

`--json` structured JSON output | `--log-level debug|info|warn|error` | `--api-url <url>` | `--rpc-url <url>`

## Common workflows

```sh
# Check price and market buy
dreamdex ticker SOMI:SOMUSD --json
dreamdex order place SOMI:SOMUSD --side buy --amount 100

# Limit sell
dreamdex order place SOMI:SOMUSD --side sell --type limit --amount 50 --price 0.20

# Vault deposit and trade
dreamdex vault approve SOMI:SOMUSD --currency SOMUSD --amount 1000
dreamdex vault deposit SOMI:SOMUSD --currency SOMUSD --amount 500
dreamdex order place SOMI:SOMUSD --side buy --amount 100 --funding-source vault

# Monitor and cancel
dreamdex order list SOMI:SOMUSD --status open --json
dreamdex order cancel SOMI:SOMUSD <order-id>

# Stream trades
dreamdex watch trades SOMI:SOMUSD --timeout 5m
```
