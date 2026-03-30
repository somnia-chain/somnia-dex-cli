package cmd

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

var client *api.Client

var rootCmd = &cobra.Command{
	Use:   "dreamdex",
	Short: "Trade on Somnia's DreamDEX",
	Long: `DreamDEX CLI — a non-custodial trading client for DreamDEX on Somnia.

Environment variables:
  DREAMDEX_API_URL       API base URL (default: staging)
  DREAMDEX_RPC_URL       Somnia JSON-RPC URL
  DREAMDEX_PRIVATE_KEY   Hex-encoded private key for signing`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		apiURL, _ := cmd.Flags().GetString("api-url")
		client = api.NewClient(apiURL)
		if token, err := loadToken(); err == nil {
			client.Token = token
		} else if os.Getenv("DREAMDEX_PRIVATE_KEY") != "" {
			authenticate(apiURL) //nolint:errcheck // best-effort; commands that need auth will fail with a clear message
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("api-url", envOr("DREAMDEX_API_URL", "https://stg.dreamdex.somnia.host"), "API base URL")
	rootCmd.PersistentFlags().String("rpc-url", envOr("DREAMDEX_RPC_URL", "https://dream-rpc.somnia.network"), "Somnia RPC URL")
	rootCmd.PersistentFlags().Bool("json", false, "output as JSON")

	rootCmd.AddCommand(ophis.Command(&ophis.Config{
		DefaultEnv: map[string]string{
			"PATH": os.Getenv("PATH"),
		},
	}))
}

// Helpers

func isJSON(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("json")
	return v
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func privateKey() (*ecdsa.PrivateKey, error) {
	raw := os.Getenv("DREAMDEX_PRIVATE_KEY")
	if raw == "" {
		return nil, fmt.Errorf("DREAMDEX_PRIVATE_KEY not set")
	}
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "0x"), "0X")
	return crypto.HexToECDSA(raw)
}

// Token persistence

type tokenFile struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

func tokenPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "dreamdex", "token.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dreamdex", "token.json")
}

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

func saveToken(token string, expiresAt int64) error {
	p := tokenPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, _ := json.Marshal(tokenFile{Token: token, ExpiresAt: expiresAt})
	return os.WriteFile(p, data, 0o600)
}

// SIWE signing

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

// Transaction signing and broadcasting

func signAndSend(cmd *cobra.Command, key *ecdsa.PrivateKey, tx *api.Transaction, wait bool) error {
	rpcURL, _ := cmd.Flags().GetString("rpc-url")
	ec, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("connect to RPC: %w", err)
	}
	defer ec.Close()

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
			return fmt.Errorf("estimate gas: %w", err)
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

	fmt.Printf("Transaction sent: %s\n", signed.Hash().Hex())

	if wait {
		fmt.Print("Waiting for confirmation...")
		receipt, err := waitForReceipt(ctx, ec, signed.Hash())
		if err != nil {
			return fmt.Errorf("\nwait for receipt: %w", err)
		}
		fmt.Printf("\nConfirmed in block %s (status: %d)\n", receipt.BlockNumber, receipt.Status)
	}

	return nil
}

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
