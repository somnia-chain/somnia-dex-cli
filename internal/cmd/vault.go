package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

// vaultCmd returns the "vault" parent command for managing vault balances.
func (a *app) vaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage vault balances",
	}
	cmd.AddCommand(
		a.vaultBalanceCmd(),
		a.vaultApproveCmd(),
		a.vaultDepositCmd(),
		a.vaultWithdrawCmd(),
	)
	return cmd
}

// vaultBalanceCmd returns the "vault balance" command, which shows vault balances.
func (a *app) vaultBalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance [symbol]",
		Short: "Show vault balances (all markets if no symbol given)",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get vault balances",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			wallet, _ := cmd.Flags().GetString("wallet")
			if wallet == "" {
				key, err := privateKey()
				if err != nil {
					return fmt.Errorf("set --wallet or DREAMDEX_PRIVATE_KEY")
				}
				wallet = crypto.PubkeyToAddress(key.PublicKey).Hex()
			}

			symbols, err := a.resolveSymbols(args)
			if err != nil {
				return err
			}
			var all []api.VaultBalance
			for _, sym := range symbols {
				balances, err := a.client.GetVaultBalance(sym, wallet)
				if err != nil {
					return err
				}
				all = append(all, balances...)
			}
			if isJSON(cmd) {
				return printJSON(all)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "CURRENCY\tBALANCE")
			for _, b := range all {
				fmt.Fprintf(w, "%s\t%s\n", b.Currency, b.Amount)
			}
			return w.Flush()
		},
	}
	cmd.Flags().String("wallet", "", "wallet address (derived from DREAMDEX_PRIVATE_KEY if unset)")
	return cmd
}

// vaultApproveCmd returns the "vault approve" command for token spending approval.
func (a *app) vaultApproveCmd() *cobra.Command {
	return a.vaultActionCmd("approve", "Approve token spending for vault deposits",
		func(c *api.Client, symbol string, req *api.VaultActionRequest) (*api.Transaction, error) {
			return c.PrepareApproval(symbol, req)
		})
}

// vaultDepositCmd returns the "vault deposit" command for depositing tokens.
func (a *app) vaultDepositCmd() *cobra.Command {
	return a.vaultActionCmd("deposit", "Deposit tokens into vault",
		func(c *api.Client, symbol string, req *api.VaultActionRequest) (*api.Transaction, error) {
			return c.PrepareDeposit(symbol, req)
		})
}

// vaultWithdrawCmd returns the "vault withdraw" command for withdrawing tokens.
func (a *app) vaultWithdrawCmd() *cobra.Command {
	return a.vaultActionCmd("withdraw", "Withdraw tokens from vault",
		func(c *api.Client, symbol string, req *api.VaultActionRequest) (*api.Transaction, error) {
			return c.PrepareWithdraw(symbol, req)
		})
}

// prepareFunc is the signature for vault action API calls (approve, deposit, withdraw).
type prepareFunc func(*api.Client, string, *api.VaultActionRequest) (*api.Transaction, error)

// vaultActionCmd is a shared constructor for vault approve, deposit, and withdraw commands.
func (a *app) vaultActionCmd(use, short string, prepare prepareFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use + " <symbol>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: short,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := privateKey()
			if err != nil {
				return err
			}
			addr := crypto.PubkeyToAddress(key.PublicKey)
			currency, _ := cmd.Flags().GetString("currency")
			amount, _ := cmd.Flags().GetString("amount")

			tx, err := prepare(a.client, args[0], &api.VaultActionRequest{
				WalletAddress: addr.Hex(),
				Currency:      currency,
				Amount:        amount,
			})
			if err != nil {
				return err
			}

			wait, _ := cmd.Flags().GetBool("wait")
			label := strings.Title(use) //nolint:staticcheck
			return a.signAndSend(cmd, key, tx, wait, label)
		},
	}
	cmd.Flags().String("currency", "", "currency code, e.g. SOM or USDC (required)")
	cmd.Flags().String("amount", "", "amount (required)")
	cmd.Flags().Bool("wait", false, "wait for transaction confirmation")
	cmd.MarkFlagRequired("currency")
	cmd.MarkFlagRequired("amount")
	return cmd
}
