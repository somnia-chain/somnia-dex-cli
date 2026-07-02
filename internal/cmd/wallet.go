package cmd

import (
	"fmt"

	"github.com/njayp/ophis"
	"github.com/spf13/cobra"
)

// walletCmd returns the "wallet" parent command for wallet-level queries across markets.
func (a *app) walletCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wallet",
		Short: "Query wallet balances, volume, and smart wallets",
		Long:  "Query a wallet's balances and traded volume across all markets, and resolve its smart wallets. Commands default to your own wallet when no address is given.",
	}
	cmd.AddCommand(
		a.walletBalanceCmd(),
		a.walletVolumeCmd(),
		a.walletSmartWalletsCmd(),
	)
	return cmd
}

// resolveWallet authenticates the caller, then returns the positional wallet arg or,
// when absent, the key-derived address. Wallet queries require an authenticated request.
func (a *app) resolveWallet(cmd *cobra.Command, args []string) (string, error) {
	if err := a.requireAuth(cmd); err != nil {
		return "", err
	}
	if len(args) == 1 {
		return args[0], nil
	}
	if a.eth == nil {
		return "", fmt.Errorf("pass a wallet address (no key configured to derive your address)")
	}
	return a.eth.Address().Hex(), nil
}

// walletBalanceCmd shows a wallet's balances across markets.
func (a *app) walletBalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance [wallet]",
		Short: "Show wallet balances across markets",
		Long:  "Show wallet and vault balances per currency for every market. Use --block to pin the read to a block number.",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get wallet balances",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			wallet, err := a.resolveWallet(cmd, args)
			if err != nil {
				return err
			}
			block, _ := cmd.Flags().GetInt64("block")
			bal, err := a.client.GetWalletBalance(wallet, block)
			if err != nil {
				return err
			}
			return printResult(cmd, bal)
		},
	}
	cmd.Flags().Int64("block", 0, "pin balance reads to this block number")
	return cmd
}

// walletVolumeCmd shows a wallet's traded volume per market.
func (a *app) walletVolumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume [wallet]",
		Short: "Show wallet trading volume across markets",
		Long:  "Show a wallet's traded volume per market over an optional [--since, --until) window (unix ms).",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get wallet volume",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			wallet, err := a.resolveWallet(cmd, args)
			if err != nil {
				return err
			}
			since, _ := cmd.Flags().GetInt64("since")
			until, _ := cmd.Flags().GetInt64("until")
			vol, err := a.client.GetWalletVolume(wallet, since, until)
			if err != nil {
				return err
			}
			return printResult(cmd, vol)
		},
	}
	cmd.Flags().Int64("since", 0, "inclusive lower bound of the window (unix ms)")
	cmd.Flags().Int64("until", 0, "exclusive upper bound of the window (unix ms)")
	return cmd
}

// walletSmartWalletsCmd resolves the smart wallets provisioned for an EOA.
func (a *app) walletSmartWalletsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "smart-wallets [wallet]",
		Short: "Resolve smart wallets for an EOA",
		Long:  "List the smart wallets provisioned for an externally-owned account.",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Resolve smart wallets",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			wallet, err := a.resolveWallet(cmd, args)
			if err != nil {
				return err
			}
			sw, err := a.client.GetSmartWallets(wallet)
			if err != nil {
				return err
			}
			return printResult(cmd, sw)
		},
	}
}
