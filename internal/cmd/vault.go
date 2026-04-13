package cmd

import (
	"fmt"
	"strings"

	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

// vaultCmd returns the "vault" parent command for managing vault balances.
func (a *app) vaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage vault balances",
		Long: `Manage the on-chain vault, which holds tokens earmarked for trading.

Depositing into the vault pre-funds your account so orders can settle without a
fresh wallet transfer each time. Approve a token first, then deposit. Trade with
--funding-source vault, and withdraw when done.`,
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
		Use:   "balance <symbol>",
		Short: "Show vault balances",
		Long: `Show token balances held in the vault for a given market. Defaults to your
wallet address if --wallet is not specified.`,
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get vault balances",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			wallet, _ := cmd.Flags().GetString("wallet")
			if wallet == "" {
				if err := a.requireEth(cmd); err != nil {
					return fmt.Errorf("set --wallet or configure a key via: dreamdex login")
				}
				wallet = a.eth.Address().Hex()
			}

			balances, err := a.client.GetVaultBalance(args[0], wallet)
			if err != nil {
				return err
			}
			return printResult(cmd, api.VaultBalances{Balances: balances})
		},
	}
	cmd.Flags().String("wallet", "", "wallet address (derived from DREAMDEX_PRIVATE_KEY if unset)")
	return cmd
}

// vaultApproveCmd returns the "vault approve" command for token spending approval.
func (a *app) vaultApproveCmd() *cobra.Command {
	return a.vaultActionCmd("approve", "Approve token spending for vault deposits",
		"Approve the vault contract to transfer a token on your behalf. Required once per token before the first deposit.",
		func(c *api.Client, symbol string, req *api.VaultActionRequest) (*api.Transaction, error) {
			return c.PrepareApproval(symbol, req)
		})
}

// vaultDepositCmd returns the "vault deposit" command for depositing tokens.
func (a *app) vaultDepositCmd() *cobra.Command {
	return a.vaultActionCmd("deposit", "Deposit tokens into vault",
		"Deposit tokens from your wallet into the vault. Tokens must be approved first via 'vault approve'.",
		func(c *api.Client, symbol string, req *api.VaultActionRequest) (*api.Transaction, error) {
			return c.PrepareDeposit(symbol, req)
		})
}

// vaultWithdrawCmd returns the "vault withdraw" command for withdrawing tokens.
func (a *app) vaultWithdrawCmd() *cobra.Command {
	return a.vaultActionCmd("withdraw", "Withdraw tokens from vault",
		"Withdraw tokens from the vault back to your wallet.",
		func(c *api.Client, symbol string, req *api.VaultActionRequest) (*api.Transaction, error) {
			return c.PrepareWithdraw(symbol, req)
		})
}

// prepareFunc is the signature for vault action API calls (approve, deposit, withdraw).
type prepareFunc func(*api.Client, string, *api.VaultActionRequest) (*api.Transaction, error)

// vaultActionCmd is a shared constructor for vault approve, deposit, and withdraw commands.
func (a *app) vaultActionCmd(use, short, long string, prepare prepareFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use + " <symbol>",
		Short: short,
		Long:  long,
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: short,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireEth(cmd); err != nil {
				return err
			}
			currency, _ := cmd.Flags().GetString("currency")
			amount, _ := cmd.Flags().GetString("amount")

			tx, err := prepare(a.client, args[0], &api.VaultActionRequest{
				WalletAddress: a.eth.Address().Hex(),
				Currency:      currency,
				Amount:        amount,
			})
			if err != nil {
				return err
			}

			wait, _ := cmd.Flags().GetBool("wait")
			label := strings.Title(use) //nolint:staticcheck
			return a.eth.SignAndSend(tx, wait, label)
		},
	}
	cmd.Flags().String("currency", "", "currency code, e.g. SOM or USDC (required)")
	cmd.Flags().String("amount", "", "amount (required)")
	cmd.Flags().Bool("wait", false, "wait for transaction confirmation")
	cmd.MarkFlagRequired("currency")
	cmd.MarkFlagRequired("amount")
	return cmd
}
