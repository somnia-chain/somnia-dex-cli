package cmd

import (
	"fmt"

	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

// builderCmd returns the "builder" parent command for managing builder-fee approvals.
func (a *app) builderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builder",
		Short: "Manage builder-fee approvals",
		Long: `Manage builder approvals, which let a builder charge a per-fill fee on orders you
submit tagged with that builder code.

Approve a builder once (bounded by the protocol-wide cap), then place orders with
--builder and --builder-fee. Approvals are keyed on the signing wallet.`,
	}
	cmd.AddCommand(
		a.builderApprovalCmd(),
		a.builderMaxFeeCmd(),
		a.builderApproveCmd(),
	)
	return cmd
}

// builderApprovalCmd shows a wallet's builder approval for a market.
func (a *app) builderApprovalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approval <symbol>",
		Short: "Show a wallet's builder approval",
		Long:  "Show the builder-fee approval a wallet has granted a builder on a market. Defaults to your wallet if --wallet is unset.",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get builder approval",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireAuth(cmd); err != nil {
				return err
			}
			builder, _ := cmd.Flags().GetString("builder")
			wallet, _ := cmd.Flags().GetString("wallet")
			if wallet == "" {
				if a.eth == nil {
					return fmt.Errorf("set --wallet (no key configured to derive your address)")
				}
				wallet = a.eth.Address().Hex()
			}
			approval, err := a.client.GetBuilderApproval(args[0], wallet, builder)
			if err != nil {
				return err
			}
			return printResult(cmd, approval)
		},
	}
	cmd.Flags().String("builder", "", "builder address to query (required)")
	cmd.Flags().String("wallet", "", "wallet address (derived from key if unset)")
	cmd.MarkFlagRequired("builder")
	return cmd
}

// builderMaxFeeCmd shows the protocol-wide builder fee cap for a market.
func (a *app) builderMaxFeeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "max-fee <symbol>",
		Short: "Show the protocol builder fee cap",
		Long:  "Show the protocol-wide cap on builder approvals, in BPS_TIMES_1K units (0 disables builder codes).",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get builder fee cap",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireAuth(cmd); err != nil {
				return err
			}
			max, err := a.client.GetBuilderMaxFee(args[0])
			if err != nil {
				return err
			}
			if isJSON(cmd) {
				return printJSON(struct {
					MaxFeeBpsTimes1k int64 `json:"maxFeeBpsTimes1k"`
				}{max})
			}
			fmt.Printf("maxFeeBpsTimes1k: %d\n", max)
			return nil
		},
	}
}

// builderApproveCmd prepares, signs, and sends a builder approval. --max-fee 0 revokes.
func (a *app) builderApproveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve <symbol>",
		Short: "Approve (or revoke) a builder fee permission",
		Long:  "Grant a builder permission to charge a per-fill fee up to --max-fee (BPS_TIMES_1K). Set --max-fee 0 to revoke.",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Approve builder",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireEth(cmd); err != nil {
				return err
			}
			builder, _ := cmd.Flags().GetString("builder")
			maxFee, _ := cmd.Flags().GetInt64("max-fee")

			tx, err := a.client.PrepareBuilderApproval(args[0], &api.BuilderApprovalRequest{
				Builder:          builder,
				MaxFeeBpsTimes1k: maxFee,
			})
			if err != nil {
				return err
			}
			_, err = a.eth.SignAndSend(tx, "Builder approval")
			return err
		},
	}
	cmd.Flags().String("builder", "", "builder address (required)")
	cmd.Flags().Int64("max-fee", 0, "max fee in BPS_TIMES_1K (0 revokes)")
	cmd.MarkFlagRequired("builder")
	return cmd
}
