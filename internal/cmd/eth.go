package cmd

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// ethClient holds a private key and RPC endpoint for signing and sending transactions.
type ethClient struct {
	key    *ecdsa.PrivateKey
	addr   common.Address
	rpcURL string
	log    *slog.Logger
}

// newEthClient wraps a private key with an RPC endpoint for transaction signing.
func newEthClient(key *ecdsa.PrivateKey, rpcURL string, log *slog.Logger) *ethClient {
	return &ethClient{
		key:    key,
		addr:   crypto.PubkeyToAddress(key.PublicKey),
		rpcURL: rpcURL,
		log:    log,
	}
}

// Address returns the wallet address derived from the private key.
func (e *ethClient) Address() common.Address {
	return e.addr
}

// SignPersonal signs a message using EIP-191 personal_sign and returns the hex signature.
func (e *ethClient) SignPersonal(message string) (string, error) {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	hash := crypto.Keccak256([]byte(prefix + message))
	sig, err := crypto.Sign(hash, e.key)
	if err != nil {
		return "", err
	}
	sig[64] += 27 // V: 0/1 -> 27/28
	return "0x" + hex.EncodeToString(sig), nil
}

// SignAndSend signs an unsigned transaction from the API and broadcasts it via RPC.
func (e *ethClient) SignAndSend(tx *api.Transaction, label string) error {
	e.log.Debug("unsigned tx", "to", tx.To, "value", tx.Value, "chainId", tx.ChainID, "data", tx.Data)

	rc, err := rpc.DialOptions(context.Background(), e.rpcURL,
		rpc.WithHTTPClient(&http.Client{
			Transport: &debugTransport{base: http.DefaultTransport, log: e.log},
		}),
	)
	if err != nil {
		return fmt.Errorf("connect to RPC: %w", err)
	}
	defer rc.Close()
	ec := ethclient.NewClient(rc)

	ctx := context.Background()
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
			From: e.addr, To: &to, Value: value, Data: data,
		})
		if err != nil {
			return fmt.Errorf("transaction would revert: %s", revertReason(err))
		}
	}

	nonce, err := ec.PendingNonceAt(ctx, e.addr)
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

	signed, err := types.SignTx(rawTx, types.NewEIP155Signer(chainID), e.key)
	if err != nil {
		return fmt.Errorf("sign tx: %w", err)
	}

	if err := ec.SendTransaction(ctx, signed); err != nil {
		return fmt.Errorf("send tx: %w", err)
	}

	fmt.Printf("%s sent: %s\n", label, signed.Hash().Hex())
	fmt.Printf("Waiting for %s confirmation...", strings.ToLower(label))
	receipt, err := waitForReceipt(ctx, ec, signed.Hash())
	if err != nil {
		return fmt.Errorf("\nwait for receipt: %w", err)
	}
	fmt.Printf(" confirmed in block %s (status: %d)\n", receipt.BlockNumber, receipt.Status)
	if tx.OrderID != "" {
		fmt.Printf("Order ID: %s\n", tx.OrderID)
	}
	for i, el := range receipt.Logs {
		e.log.Debug("event", "index", i, "address", el.Address.Hex())
		for j, topic := range el.Topics {
			e.log.Debug("  topic", "index", j, "value", topic.Hex())
		}
		if len(el.Data) > 0 {
			e.log.Debug("  data", "hex", "0x"+hex.EncodeToString(el.Data))
		}
	}

	return nil
}

// keystoreDir returns the keystore directory namespaced by API host.
func keystoreDir(apiURL string) string {
	host := "default"
	if u, err := url.Parse(apiURL); err == nil && u.Host != "" {
		host = u.Host
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "dreamdex", "keystore", host)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dreamdex", "keystore", host)
}

// loadKeyFromEnv loads a private key from the DREAMDEX_PRIVATE_KEY environment variable.
func loadKeyFromEnv() (*ecdsa.PrivateKey, error) {
	raw := os.Getenv("DREAMDEX_PRIVATE_KEY")
	if raw == "" {
		return nil, fmt.Errorf("DREAMDEX_PRIVATE_KEY not set")
	}
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "0x"), "0X")
	return crypto.HexToECDSA(raw)
}

// loadKeyFromKeystore decrypts the first account in the keystore directory.
func loadKeyFromKeystore(apiURL string) (*ecdsa.PrivateKey, error) {
	dir := keystoreDir(apiURL)
	host := "default"
	if u, err := url.Parse(apiURL); err == nil && u.Host != "" {
		host = u.Host
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("no keystore found for %s; run: dreamdex login --api-url %s", host, apiURL)
	}
	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no accounts in keystore for %s; run: dreamdex login --api-url %s", host, apiURL)
	}

	keyjson, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	if err != nil {
		return nil, err
	}

	pass, err := readPassword("Passphrase: ")
	if err != nil {
		return nil, err
	}

	key, err := keystore.DecryptKey(keyjson, pass)
	if err != nil {
		return nil, fmt.Errorf("wrong passphrase")
	}
	return key.PrivateKey, nil
}

// readPassword reads a password from DREAMDEX_PASSWORD env var or an interactive terminal prompt.
func readPassword(prompt string) (string, error) {
	if pass := os.Getenv("DREAMDEX_PASSWORD"); pass != "" {
		return pass, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("set DREAMDEX_PASSWORD (no terminal for interactive prompt)")
	}
	fmt.Fprint(os.Stderr, prompt)
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	return string(pass), nil
}

// readNewPassword prompts for a new passphrase with confirmation.
func readNewPassword() (string, error) {
	pass, err := readPassword("New passphrase: ")
	if err != nil {
		return "", err
	}
	if pass == "" {
		return "", fmt.Errorf("passphrase must not be empty")
	}
	confirm, err := readPassword("Confirm passphrase: ")
	if err != nil {
		return "", err
	}
	if pass != confirm {
		return "", fmt.Errorf("passphrases do not match")
	}
	return pass, nil
}

// requireEth ensures a.eth is initialized, trying the keystore if the env var path failed.
// Also does best-effort SIWE auth when loading from keystore.
func (a *app) requireEth(cmd *cobra.Command) error {
	if a.eth != nil {
		return nil
	}
	apiURL, _ := cmd.Flags().GetString("api-url")
	key, err := loadKeyFromKeystore(apiURL)
	if err != nil {
		return fmt.Errorf("no key available: %w", err)
	}
	rpcURL, _ := cmd.Flags().GetString("rpc-url")
	a.eth = newEthClient(key, rpcURL, a.log)
	if a.client.Token == "" {
		apiURL, _ := cmd.Flags().GetString("api-url")
		a.authenticate(apiURL) //nolint:errcheck // best-effort
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
