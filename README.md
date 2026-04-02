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

All configuration is via environment variables:

| Variable | Description | Default |
|---|---|---|
| `DREAMDEX_PRIVATE_KEY` | Hex-encoded private key (`0x...` or bare hex) | — |
| `DREAMDEX_API_URL` | API base URL | `https://stg.dreamdex.somnia.host` |
| `DREAMDEX_RPC_URL` | Somnia JSON-RPC URL | `https://dream-rpc.somnia.network` |

## Authentication

Commands that require authentication will automatically sign in using `DREAMDEX_PRIVATE_KEY` if set. To explicitly log in and cache a token:

```sh
export DREAMDEX_PRIVATE_KEY=0x...
dreamdex login
```

The token is cached at `~/.config/dreamdex/token.json` and reused until it expires. Only `login` writes this file; other commands authenticate in-memory.

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

The CLI includes an [MCP](https://modelcontextprotocol.io) server via [ophis](https://github.com/njayp/ophis), allowing LLM agents to interact with DreamDEX as tool calls.

#### stdio (default)

Runs the server over stdin/stdout, the standard transport for most MCP clients:

```sh
dreamdex mcp start
```

#### HTTP (Streamable HTTP)

Runs the server as an HTTP endpoint for remote or multi-client access:

```sh
dreamdex mcp stream --addr :8080
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
claude mcp add --transport stdio dreamdex -- dreamdex mcp start
```

Or add to your `.mcp.json` (project-level) or `~/.claude/claude_desktop_config.json` (global):

```json
{
  "mcpServers": {
    "dreamdex": {
      "command": "dreamdex",
      "args": ["mcp", "start"],
      "env": {
        "DREAMDEX_PRIVATE_KEY": "0x..."
      }
    }
  }
}
```

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

## Architecture

- **Non-custodial**: the API returns unsigned EVM transactions; the CLI signs them locally with your private key and broadcasts via RPC.
- **Somnia blockchain**: Chain ID 50312.
- **SIWE authentication**: [ERC-4361](https://eips.ethereum.org/EIPS/eip-4361) Sign-In with Ethereum for JWT-based API auth.

## License

See [LICENSE](LICENSE).
