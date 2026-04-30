package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"
	"github.com/njayp/ophis"
	"github.com/rodaine/table"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

// Exit codes:
//
//	0       success
//	1       user input error (bad flags, missing arguments)
//	2       authentication error (no key, bad passphrase, unauthorized)
//	3       network error (API unreachable, RPC connection failed)
//	4       chain error (tx revert, nonce, gas estimation, signing)
//	101     order not placed (e.g. IOC/FOK with no fills)
//	102     transaction reverted on-chain (receipt status 0)
const (
	ExitInput    = 1
	ExitAuth     = 2
	ExitNetwork  = 3
	ExitChain    = 4
	ExitNoFill   = 101
	ExitReverted = 102
)

// ExitError wraps an error with a specific process exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// app holds shared state for all commands.
type app struct {
	client *api.Client
	eth    *ethClient
	log    *slog.Logger
}

// Execute runs the root command and exits on error.
func Execute() {
	a := &app{}
	root := a.rootCmd()
	if err := root.Execute(); err != nil {
		code := exitCode(err)
		if isJSON(root) {
			json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		os.Exit(code)
	}
}

// exitCode determines the process exit code for an error.
func exitCode(err error) int {
	// Explicit exit code takes priority.
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}

	// API errors: auth vs other.
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Name == "unauthorized" {
			return ExitAuth
		}
		return ExitInput
	}

	msg := err.Error()

	// Auth errors.
	for _, s := range []string{
		"no key available", "wrong passphrase", "not logged in",
		"DREAMDEX_PRIVATE_KEY", "DREAMDEX_PASSWORD", "no keystore found",
		"no accounts in keystore", "passphrase",
	} {
		if strings.Contains(msg, s) {
			return ExitAuth
		}
	}

	// Chain errors.
	for _, s := range []string{
		"transaction would revert", "sign tx", "get nonce",
		"get gas price", "send tx", "wait for receipt",
		"invalid chain ID", "invalid tx value",
	} {
		if strings.Contains(msg, s) {
			return ExitChain
		}
	}

	// Network errors.
	for _, s := range []string{
		"connect to RPC", "connect:", "read response",
	} {
		if strings.Contains(msg, s) {
			return ExitNetwork
		}
	}

	return ExitInput
}

// rootCmd builds the top-level command with persistent flags and all subcommands.
func (a *app) rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dreamdex",
		Short: "Trade on Somnia's dreamDEX",
		Long: `dreamDEX CLI — a non-custodial trading client for dreamDEX on Somnia.

Keys are stored in an encrypted keystore (~/.config/dreamdex/keystore/).
Run "dreamdex login" to import a key. For headless/CI use, set DREAMDEX_PRIVATE_KEY.
To skip key/SIWE auth entirely, pass --token or set DREAMDEX_TOKEN with a JWT.

Environment variables:
  DREAMDEX_API_URL       API base URL (default: staging)
  DREAMDEX_RPC_URL       Somnia JSON-RPC URL
  DREAMDEX_PRIVATE_KEY   Hex-encoded private key (headless fallback)
  DREAMDEX_PASSWORD      Keystore passphrase (headless fallback)
  DREAMDEX_TOKEN         JWT bearer token (bypasses key/SIWE auth)`,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Silence usage after arg validation so only input errors show help.
			cmd.SilenceUsage = true

			levelName, _ := cmd.Flags().GetString("log-level")
			a.log = slog.New(tint.NewHandler(os.Stderr, &tint.Options{
				Level: parseLogLevel(levelName),
			}))

			apiURL, _ := cmd.Flags().GetString("api-url")
			a.client = api.NewClient(apiURL)
			a.client.Log = a.log

			// Explicit token bypasses key loading and SIWE auth entirely.
			if tok, _ := cmd.Flags().GetString("token"); tok != "" {
				a.client.Token = tok
			} else if cmd.Annotations[ophis.AnnotationReadOnly] != "true" {
				rpcURL, _ := cmd.Flags().GetString("rpc-url")
				key, err := loadKeyFromEnv()
				if err != nil {
					key, err = loadKeyFromKeystore(apiURL)
				}
				if err == nil {
					a.eth = newEthClient(key, rpcURL, a.log)
					a.authenticate(apiURL) //nolint:errcheck // best-effort
				}
			}
			return nil
		},
	}

	cmd.PersistentFlags().String("api-url", envOr("DREAMDEX_API_URL", "https://stg.api.dreamdex.io"), "API base URL")
	cmd.PersistentFlags().String("rpc-url", envOr("DREAMDEX_RPC_URL", "https://dream-rpc.somnia.network"), "Somnia RPC URL")
	cmd.PersistentFlags().String("token", os.Getenv("DREAMDEX_TOKEN"), "JWT bearer token (bypasses key/SIWE auth)")
	cmd.PersistentFlags().String("log-level", "warn", "log level: debug, info, warn, error")
	cmd.PersistentFlags().Bool("json", false, "output as JSON")

	cmd.AddCommand(
		a.marketsCmd(),
		a.currenciesCmd(),
		a.orderbookCmd(),
		a.tickerCmd(),
		a.tradesCmd(),
		a.candlesCmd(),
		a.loginCmd(),
		a.orderCmd(),
		a.stopOrderCmd(),
		a.vaultCmd(),
		a.watchCmd(),
		skillCmd(),
		ophis.Command(&ophis.Config{
			DefaultEnv: map[string]string{
				"PATH":          os.Getenv("PATH"),
				"DREAMDEX_JSON": "1",
			},
		}),
	)

	return cmd
}

// envOr returns the environment variable value or fallback if unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// isJSON returns true when the --json flag is set or when running as an MCP subprocess.
func isJSON(cmd *cobra.Command) bool {
	if v, _ := cmd.Flags().GetBool("json"); v {
		return true
	}
	return os.Getenv("DREAMDEX_JSON") == "1"
}

// printJSON writes v to stdout as indented JSON.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printResult renders v as JSON or a table depending on the --json flag.
// It uses type assertions to check if v implements JSON() or Table().
func printResult(cmd *cobra.Command, v any) error {
	if isJSON(cmd) {
		if j, ok := v.(interface{ JSON() any }); ok {
			return printJSON(j.JSON())
		}
		return printJSON(v)
	}
	if t, ok := v.(interface{ Table() [][]string }); ok {
		rows := t.Table()
		if len(rows) == 0 {
			return nil
		}
		headers := make([]any, len(rows[0]))
		for i, h := range rows[0] {
			headers[i] = h
		}
		tbl := table.New(headers...)
		for _, row := range rows[1:] {
			vals := make([]any, len(row))
			for i, v := range row {
				vals[i] = v
			}
			tbl.AddRow(vals...)
		}
		tbl.Print()
	}
	return nil
}

// resolveSymbols returns args if non-empty, otherwise fetches all market symbols.
func (a *app) resolveSymbols(args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	markets, err := a.client.GetMarkets()
	if err != nil {
		return nil, fmt.Errorf("fetch markets: %w", err)
	}
	symbols := make([]string, len(markets))
	for i, m := range markets {
		symbols[i] = m.Symbol
	}
	return symbols, nil
}
