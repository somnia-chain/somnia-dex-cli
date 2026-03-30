package cmd

import (
	"fmt"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(loginCmd)
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Sign-In with Ethereum",
	RunE: func(cmd *cobra.Command, _ []string) error {
		apiURL, _ := cmd.Flags().GetString("api-url")
		addr, expiresAt, err := authenticate(apiURL)
		if err != nil {
			return err
		}
		if err := saveToken(client.Token, expiresAt); err != nil {
			return fmt.Errorf("save token: %w", err)
		}
		fmt.Printf("Logged in as %s (expires %s)\n",
			addr, time.UnixMilli(expiresAt).Format(time.RFC3339))
		return nil
	},
}

// authenticate performs SIWE auth and sets client.Token in memory.
func authenticate(apiURL string) (addr string, expiresAt int64, err error) {
	key, err := privateKey()
	if err != nil {
		return "", 0, err
	}
	address := crypto.PubkeyToAddress(key.PublicKey)

	nonce, err := client.GetNonce()
	if err != nil {
		return "", 0, fmt.Errorf("get nonce: %w", err)
	}

	message := buildSIWE(apiURL, address.Hex(), nonce)

	sig, err := signPersonal(key, message)
	if err != nil {
		return "", 0, fmt.Errorf("sign message: %w", err)
	}

	token, exp, err := client.Login(message, sig)
	if err != nil {
		return "", 0, fmt.Errorf("login: %w", err)
	}

	client.Token = token
	return address.Hex(), exp, nil
}

func buildSIWE(apiURL, address, nonce string) string {
	u, _ := url.Parse(apiURL)
	now := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf(`%s wants you to sign in with your Ethereum account:
%s

Sign in to Somnia DEX

URI: %s
Version: 1
Chain ID: 50312
Nonce: %s
Issued At: %s`, u.Host, address, apiURL, nonce, now)
}
