# DreamDEX CLI

A non-custodial trading client for [DreamDEX](https://dreamdex.somnia.host) on the [Somnia](https://somnia.network) blockchain.

## Install

Via [mise](https://mise.jdx.dev):

```sh
mise use -g go:github.com/somnia-chain/somnia-dex-cli/cmd/dreamdex
```

Via `go install`:

```sh
go install github.com/somnia-chain/somnia-dex-cli/cmd/dreamdex@latest
```

Or build from source:

```sh
git clone https://github.com/somnia-chain/somnia-dex-cli.git
cd somnia-dex-cli
go build -o dreamdex ./cmd/dreamdex/
```

## Configuration

| Variable | Description | Default |
|---|---|---|
| `DREAMDEX_API_URL` | API base URL | `https://stg.dreamdex.somnia.host` |
| `DREAMDEX_RPC_URL` | Somnia JSON-RPC URL | `https://dream-rpc.somnia.network` |
| `DREAMDEX_PRIVATE_KEY` | Hex-encoded private key (headless/CI fallback) | — |
| `DREAMDEX_PASSWORD` | Keystore passphrase (headless/CI fallback) | — |
| `DREAMDEX_JSON` | Force JSON output (useful for MCP/scripting) | — |

## Key management

Private keys are stored in an encrypted keystore using the [Web3 Secret Storage](https://ethereum.org/en/developers/docs/data-structures-and-encoding/web3-secret-storage/) format (the same format used by geth). The keystore is namespaced by API host under your OS config directory:

| OS | Path |
|---|---|
| Linux | `~/.config/dreamdex/keystore/<host>/` |
| macOS | `~/Library/Application Support/dreamdex/keystore/<host>/` |
| Windows | `%AppData%\dreamdex\keystore\<host>\` |

**Your private key is never stored in plaintext.** It is encrypted with your passphrase using scrypt + AES-128-CTR before being written to disk. The plaintext key is held in memory only for the duration of the command and is never logged, even at debug log level.

### First-time setup

Import your private key into the keystore:

```sh
export DREAMDEX_PRIVATE_KEY=0x...
dreamdex login
```

This prompts for a passphrase, encrypts the key, and stores it. **Unset `DREAMDEX_PRIVATE_KEY` immediately after** - subsequent commands will read from the encrypted keystore and prompt for your passphrase.

### Headless / MCP usage

For CI, scripts, or MCP servers where no terminal is available, set the env vars directly:

```sh
DREAMDEX_PRIVATE_KEY=0x...   # bypasses keystore entirely
DREAMDEX_PASSWORD=...        # unlocks keystore without a prompt
```

## Usage

### Market data

```sh
dreamdex markets                          # list all trading pairs
dreamdex currencies                       # list supported tokens
dreamdex orderbook                        # order book for all markets
dreamdex orderbook SOMI:SOMUSD --depth 5  # single market, 5 levels
dreamdex ticker                           # 24h stats for all markets
dreamdex trades WETH:SOMUSD --limit 10    # recent trades
dreamdex candles WBTC:SOMUSD --interval 1h --limit 50
```

### Orders

```sh
dreamdex order place SOMI:SOMUSD --side buy --amount 100
dreamdex order place SOMI:SOMUSD --side sell --type limit --amount 50 --price 0.20
dreamdex order list                       # all markets
dreamdex order list SOMI:SOMUSD --status open
dreamdex order get SOMI:SOMUSD <order-id>
dreamdex order cancel SOMI:SOMUSD <order-id>
dreamdex order reduce SOMI:SOMUSD <order-id> --quantity 25
```

### Stop orders

```sh
dreamdex stoporder place SOMI:SOMUSD --side sell --amount 1 --trigger-price 0.17 --trigger-operator lte
dreamdex stoporder place SOMI:SOMUSD --side buy --type limit --amount 50 --price 0.20 \
  --trigger-price 0.19 --trigger-operator gte
dreamdex stoporder list                  # all markets
dreamdex stoporder list SOMI:SOMUSD --status pending
dreamdex stoporder cancel SOMI:SOMUSD <id>
```

### Vault

```sh
dreamdex vault balance                    # all markets
dreamdex vault balance SOMI:SOMUSD
dreamdex vault approve SOMI:SOMUSD --currency SOM --amount 1000
dreamdex vault deposit SOMI:SOMUSD --currency SOM --amount 500
dreamdex vault withdraw SOMI:SOMUSD --currency SOM --amount 200
```

### Live streaming

```sh
dreamdex watch orderbook SOMI:SOMUSD          # stream order book updates
dreamdex watch trades                          # stream trades for all markets
dreamdex watch candles WBTC:SOMUSD --interval 5m
dreamdex watch order <order-id>                # watch a specific order
dreamdex watch trades --timeout 5m             # auto-terminate after 5 minutes
```

### JSON output

Pass `--json` to any command for structured JSON output, useful for scripting and LLM agents:

```sh
dreamdex markets --json
dreamdex order list --json
```

### Debug logging

Pass `--log-level debug` to see all HTTP, RPC, and WebSocket traffic:

```sh
dreamdex order get SOMI:SOMUSD 123 --log-level debug
dreamdex watch trades --log-level debug
```

### MCP server

The CLI includes an [MCP](https://modelcontextprotocol.io) server via [ophis](https://github.com/njayp/ophis), allowing LLM agents to interact with DreamDEX as tool calls. The MCP server is currently designed for local (stdio) usage only.

```sh
dreamdex mcp start
```

#### IDE integration

Register the MCP server with your editor in one command:

```sh
dreamdex mcp claude   # Claude Desktop
dreamdex mcp cursor   # Cursor
dreamdex mcp vscode   # VS Code
```

#### Claude Code / OpenClaw

Via the CLI:

```sh
claude mcp add --transport stdio \
  -e DREAMDEX_JSON=1 \
  -e DREAMDEX_PRIVATE_KEY=0x... \
  dreamdex -- dreamdex mcp start
```

Or add to your `.mcp.json` (project-level) or `~/.claude.json` (global):

```json
{
  "mcpServers": {
    "dreamdex": {
      "command": "dreamdex",
      "args": ["mcp", "start"],
      "env": {
        "DREAMDEX_JSON": "1",
        "DREAMDEX_PRIVATE_KEY": "0x..."
      }
    }
  }
}
```

If you prefer to use the encrypted keystore instead of a raw key, set `DREAMDEX_PASSWORD` in place of `DREAMDEX_PRIVATE_KEY` (after running `dreamdex login` to import the key).

#### Inspect available tools

```sh
dreamdex mcp tools          # human-readable
dreamdex mcp tools --json   # JSON schema for each tool
```

### LLM skill reference

The `skill` command prints a structured command reference designed for LLM agents. It describes every command, flag, and common workflow in a format optimized for machine consumption.

```sh
dreamdex skill              # print the reference to stdout
dreamdex skill | pbcopy     # copy to clipboard (macOS)
dreamdex skill | xclip      # copy to clipboard (Linux)
```

This is useful when you want to give an LLM context about the CLI without the MCP server. For example, you can paste the output into a chat session, or pipe it into a prompt:

```sh
echo "Place a limit buy for 100 SOMI at 0.15. $(dreamdex skill)" | llm
```

The same reference is also available as [SKILL.md](SKILL.md) in the repository.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | User input error (bad flags, missing arguments) |
| 2 | Authentication error (no key, bad passphrase, unauthorized) |
| 3 | Network error (API unreachable, RPC connection failed) |
| 4 | Chain error (tx revert, nonce, gas estimation, signing) |
| 101 | Order not placed (e.g. IOC/FOK with no available fills) |
| 102 | Transaction reverted on-chain (receipt status 0) |

## Architecture

- **Non-custodial**: the API returns unsigned EVM transactions; the CLI signs them locally with your private key and broadcasts via RPC.
- **Somnia blockchain**: Chain ID 50312.
- **SIWE authentication**: [ERC-4361](https://eips.ethereum.org/EIPS/eip-4361) Sign-In with Ethereum for JWT-based API auth.

## License

See [LICENSE](LICENSE).
