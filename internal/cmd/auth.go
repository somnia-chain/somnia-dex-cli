package cmd

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"
)

// loginCmd returns the "login" command, which imports a key into the keystore and authenticates.
func (a *app) loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Set up keystore and authenticate",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := keystoreDir()
			ks := keystore.NewKeyStore(dir, keystore.StandardScryptN, keystore.StandardScryptP)

			if len(ks.Accounts()) == 0 {
				if a.eth == nil {
					return fmt.Errorf("set DREAMDEX_PRIVATE_KEY to import into keystore")
				}
				fmt.Fprintln(os.Stderr, "Importing key into keystore...")
				pass, err := readNewPassword()
				if err != nil {
					return err
				}
				if _, err := ks.ImportECDSA(a.eth.key, pass); err != nil {
					return fmt.Errorf("import key: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Key stored in %s\n", dir)
			}

			if err := a.requireEth(cmd); err != nil {
				return err
			}
			apiURL, _ := cmd.Flags().GetString("api-url")
			addr, _, err := a.authenticate(apiURL)
			if err != nil {
				return err
			}
			fmt.Printf("Authenticated as %s\n", addr)
			return nil
		},
	}
}

// authenticate performs SIWE auth and sets client.Token in memory.
func (a *app) authenticate(apiURL string) (addr string, expiresAt int64, err error) {
	if a.eth == nil {
		return "", 0, fmt.Errorf("no key available")
	}

	nonce, err := a.client.GetNonce()
	if err != nil {
		return "", 0, fmt.Errorf("get nonce: %w", err)
	}

	address := a.eth.Address()
	message := buildSIWE(apiURL, address.Hex(), nonce)

	sig, err := a.eth.SignPersonal(message)
	if err != nil {
		return "", 0, fmt.Errorf("sign message: %w", err)
	}

	token, exp, err := a.client.Login(message, sig)
	if err != nil {
		return "", 0, fmt.Errorf("login: %w", err)
	}

	a.client.Token = token
	return address.Hex(), exp, nil
}

// requireAuth ensures the client has a valid JWT, loading the key and authenticating if needed.
func (a *app) requireAuth(cmd *cobra.Command) error {
	if a.client.Token != "" {
		return nil
	}
	if err := a.requireEth(cmd); err != nil {
		return err
	}
	if a.client.Token == "" {
		apiURL, _ := cmd.Flags().GetString("api-url")
		if _, _, err := a.authenticate(apiURL); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}
	return nil
}

// buildSIWE constructs an ERC-4361 Sign-In with Ethereum message.
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
