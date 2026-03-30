package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(vaultCmd)
	vaultCmd.AddCommand(vaultBalanceCmd, vaultApproveCmd, vaultDepositCmd, vaultWithdrawCmd)

	vaultBalanceCmd.Flags().String("wallet", "", "wallet address (derived from DREAMDEX_PRIVATE_KEY if unset)")

	for _, c := range []*cobra.Command{vaultApproveCmd, vaultDepositCmd, vaultWithdrawCmd} {
		c.Flags().String("currency", "", "currency code, e.g. SOM or USDC (required)")
		c.Flags().String("amount", "", "amount (required)")
		c.Flags().Bool("wait", false, "wait for transaction confirmation")
		c.MarkFlagRequired("currency")
		c.MarkFlagRequired("amount")
	}
}

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage vault balances",
}

var vaultBalanceCmd = &cobra.Command{
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

		symbols, err := resolveSymbols(args)
		if err != nil {
			return err
		}
		var all []api.VaultBalance
		for _, sym := range symbols {
			balances, err := client.GetVaultBalance(sym, wallet)
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

var vaultApproveCmd = &cobra.Command{
	Use:   "approve <symbol>",
	Short: "Approve token spending for vault deposits",
	Args:  cobra.ExactArgs(1),
	Annotations: map[string]string{
		ophis.AnnotationTitle: "Approve vault spending",
	},
	RunE: vaultAction(func(c *api.Client, symbol string, req *api.VaultActionRequest) (*api.Transaction, error) {
		return c.PrepareApproval(symbol, req)
	}),
}

var vaultDepositCmd = &cobra.Command{
	Use:   "deposit <symbol>",
	Short: "Deposit tokens into vault",
	Args:  cobra.ExactArgs(1),
	Annotations: map[string]string{
		ophis.AnnotationTitle: "Deposit to vault",
	},
	RunE: vaultAction(func(c *api.Client, symbol string, req *api.VaultActionRequest) (*api.Transaction, error) {
		return c.PrepareDeposit(symbol, req)
	}),
}

var vaultWithdrawCmd = &cobra.Command{
	Use:   "withdraw <symbol>",
	Short: "Withdraw tokens from vault",
	Args:  cobra.ExactArgs(1),
	Annotations: map[string]string{
		ophis.AnnotationTitle: "Withdraw from vault",
	},
	RunE: vaultAction(func(c *api.Client, symbol string, req *api.VaultActionRequest) (*api.Transaction, error) {
		return c.PrepareWithdraw(symbol, req)
	}),
}

type prepareFunc func(*api.Client, string, *api.VaultActionRequest) (*api.Transaction, error)

func vaultAction(prepare prepareFunc) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		key, err := privateKey()
		if err != nil {
			return err
		}
		addr := crypto.PubkeyToAddress(key.PublicKey)
		currency, _ := cmd.Flags().GetString("currency")
		amount, _ := cmd.Flags().GetString("amount")

		tx, err := prepare(client, args[0], &api.VaultActionRequest{
			WalletAddress: addr.Hex(),
			Currency:      currency,
			Amount:        amount,
		})
		if err != nil {
			return err
		}

		wait, _ := cmd.Flags().GetBool("wait")
		return signAndSend(cmd, key, tx, wait)
	}
}
