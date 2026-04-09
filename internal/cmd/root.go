package cmd

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/lmittmann/tint"
	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

// app holds shared state for all commands.
type app struct {
	client *api.Client
	log    *slog.Logger
}

// Execute runs the root command and exits on error.
func Execute() {
	a := &app{}
	root := a.rootCmd()
	if err := root.Execute(); err != nil {
		if isJSON(root) {
			json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		os.Exit(1)
	}
}

// rootCmd builds the top-level command with persistent flags and all subcommands.
func (a *app) rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dreamdex",
		Short: "Trade on Somnia's DreamDEX",
		Long: `DreamDEX CLI — a non-custodial trading client for DreamDEX on Somnia.

Environment variables:
  DREAMDEX_API_URL       API base URL (default: staging)
  DREAMDEX_RPC_URL       Somnia JSON-RPC URL
  DREAMDEX_PRIVATE_KEY   Hex-encoded private key for signing`,
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
			if token, err := loadToken(); err == nil {
				a.client.Token = token
			} else if os.Getenv("DREAMDEX_PRIVATE_KEY") != "" {
				a.authenticate(apiURL) //nolint:errcheck // best-effort
			}
			return nil
		},
	}

	cmd.PersistentFlags().String("api-url", envOr("DREAMDEX_API_URL", "https://stg.dreamdex.somnia.host"), "API base URL")
	cmd.PersistentFlags().String("rpc-url", envOr("DREAMDEX_RPC_URL", "https://dream-rpc.somnia.network"), "Somnia RPC URL")
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
			DefaultEnv: map[string]string{"PATH": os.Getenv("PATH")},
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

// isJSON returns true when the --json flag is set.
func isJSON(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("json")
	return v
}

// printJSON writes v to stdout as indented JSON. Nil slices emit [] not null.
func printJSON(v any) error {
	if rv := reflect.ValueOf(v); !rv.IsValid() || (rv.Kind() == reflect.Slice && rv.IsNil()) {
		_, err := os.Stdout.WriteString("[]\n")
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// privateKey loads the ECDSA key from the DREAMDEX_PRIVATE_KEY environment variable.
func privateKey() (*ecdsa.PrivateKey, error) {
	raw := os.Getenv("DREAMDEX_PRIVATE_KEY")
	if raw == "" {
		return nil, fmt.Errorf("DREAMDEX_PRIVATE_KEY not set")
	}
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "0x"), "0X")
	return crypto.HexToECDSA(raw)
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

// tokenFile is the on-disk format for cached JWT tokens.
type tokenFile struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// tokenPath returns the path to the cached token file (~/.config/dreamdex/token.json).
func tokenPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "dreamdex", "token.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dreamdex", "token.json")
}

// loadToken reads a cached JWT from disk, returning an error if missing or expired.
func loadToken() (string, error) {
	data, err := os.ReadFile(tokenPath())
	if err != nil {
		return "", err
	}
	var t tokenFile
	if err := json.Unmarshal(data, &t); err != nil {
		return "", err
	}
	if time.Now().UnixMilli() >= t.ExpiresAt {
		return "", fmt.Errorf("token expired")
	}
	return t.Token, nil
}

// saveToken persists a JWT and its expiry to disk.
func saveToken(token string, expiresAt int64) error {
	p := tokenPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, _ := json.Marshal(tokenFile{Token: token, ExpiresAt: expiresAt})
	return os.WriteFile(p, data, 0o600)
}

// signPersonal signs a message using EIP-191 personal_sign and returns the hex signature.
func signPersonal(key *ecdsa.PrivateKey, message string) (string, error) {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	hash := crypto.Keccak256([]byte(prefix + message))
	sig, err := crypto.Sign(hash, key)
	if err != nil {
		return "", err
	}
	sig[64] += 27 // V: 0/1 → 27/28
	return "0x" + hex.EncodeToString(sig), nil
}

// signAndSend signs an unsigned transaction from the API and broadcasts it via RPC.
func (a *app) signAndSend(cmd *cobra.Command, key *ecdsa.PrivateKey, tx *api.Transaction, wait bool, labels ...string) error {
	label := "Transaction"
	if len(labels) > 0 {
		label = labels[0]
	}
	a.log.Debug("unsigned tx", "to", tx.To, "value", tx.Value, "chainId", tx.ChainID, "data", tx.Data)

	rpcURL, _ := cmd.Flags().GetString("rpc-url")
	rc, err := rpc.DialOptions(context.Background(), rpcURL,
		rpc.WithHTTPClient(&http.Client{
			Transport: &debugTransport{base: http.DefaultTransport, log: a.log},
		}),
	)
	if err != nil {
		return fmt.Errorf("connect to RPC: %w", err)
	}
	defer rc.Close()
	ec := ethclient.NewClient(rc)

	ctx := context.Background()
	from := crypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress(tx.To)
	data := common.FromHex(tx.Data)

	value := new(big.Int)
	if tx.Value != "" {
		if _, ok := value.SetString(tx.Value, 10); !ok {
			return fmt.Errorf("invalid tx value: %s", tx.Value)
		}
	}
	chainID := new(big.Int)
	if _, ok := chainID.SetString(tx.ChainID, 10); !ok {
		return fmt.Errorf("invalid chain ID: %s", tx.ChainID)
	}

	var gasLimit uint64
	if tx.GasLimit != "" {
		gasLimit, _ = strconv.ParseUint(tx.GasLimit, 10, 64)
	}
	if gasLimit == 0 {
		gasLimit, err = ec.EstimateGas(ctx, ethereum.CallMsg{
			From: from, To: &to, Value: value, Data: data,
		})
		if err != nil {
			return fmt.Errorf("transaction would revert: %s", revertReason(err))
		}
	}

	nonce, err := ec.PendingNonceAt(ctx, from)
	if err != nil {
		return fmt.Errorf("get nonce: %w", err)
	}
	gasPrice, err := ec.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("get gas price: %w", err)
	}

	rawTx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    value,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	})

	signed, err := types.SignTx(rawTx, types.NewEIP155Signer(chainID), key)
	if err != nil {
		return fmt.Errorf("sign tx: %w", err)
	}

	if err := ec.SendTransaction(ctx, signed); err != nil {
		return fmt.Errorf("send tx: %w", err)
	}

	fmt.Printf("%s sent: %s\n", label, signed.Hash().Hex())

	if wait {
		fmt.Printf("Waiting for %s confirmation...", strings.ToLower(label))
		receipt, err := waitForReceipt(ctx, ec, signed.Hash())
		if err != nil {
			return fmt.Errorf("\nwait for receipt: %w", err)
		}
		fmt.Printf(" confirmed in block %s (status: %d)\n", receipt.BlockNumber, receipt.Status)
		for i, el := range receipt.Logs {
			a.log.Debug("event", "index", i, "address", el.Address.Hex())
			for j, topic := range el.Topics {
				a.log.Debug("  topic", "index", j, "value", topic.Hex())
			}
			if len(el.Data) > 0 {
				a.log.Debug("  data", "hex", "0x"+hex.EncodeToString(el.Data))
			}
		}
	}

	return nil
}

// waitForReceipt polls for a transaction receipt until confirmed or timeout (2 minutes).
func waitForReceipt(ctx context.Context, ec *ethclient.Client, hash common.Hash) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tick.C:
			receipt, err := ec.TransactionReceipt(ctx, hash)
			if err == nil {
				return receipt, nil
			}
		}
	}
}

// debugTransport wraps an http.RoundTripper and logs JSON-RPC requests and responses via slog.
type debugTransport struct {
	base http.RoundTripper
	log  *slog.Logger
}

func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}
	if len(reqBody) > 0 {
		d.log.Debug("rpc >>", "method", req.Method, "url", req.URL.String(), "body", string(reqBody))
	}

	resp, err := d.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	if len(respBody) > 0 {
		d.log.Debug("rpc <<", "status", resp.StatusCode, "body", string(respBody))
	}
	return resp, nil
}

// revertReason extracts a human-readable reason from an EVM revert error.
func revertReason(err error) string {
	var dataErr rpc.DataError
	if !errors.As(err, &dataErr) {
		return err.Error()
	}
	hexData, ok := dataErr.ErrorData().(string)
	if !ok {
		return err.Error()
	}
	data := common.FromHex(hexData)
	if reason, unpackErr := abi.UnpackRevert(data); unpackErr == nil {
		return reason
	}
	return err.Error()
}
