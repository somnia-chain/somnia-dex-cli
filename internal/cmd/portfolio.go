package cmd

import (
	"github.com/njayp/ophis"
	"github.com/spf13/cobra"
)

// portfolioCmd returns the "portfolio" command, which shows trading performance metrics.
func (a *app) portfolioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Show portfolio metrics for the authenticated wallet",
		Long: `Show trading performance for the authenticated wallet: PnL, money-weighted return,
traded volume, and estimated fees saved versus a centralized exchange.`,
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get portfolio metrics",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.requireAuth(cmd); err != nil {
				return err
			}
			timeframe, _ := cmd.Flags().GetString("timeframe")
			sessionSince, _ := cmd.Flags().GetInt64("session-since")
			p, err := a.client.GetPortfolio(timeframe, sessionSince)
			if err != nil {
				return err
			}
			return printResult(cmd, p)
		},
	}
	cmd.Flags().String("timeframe", "", "time window: 24h, 7d, 30d, all")
	cmd.Flags().Int64("session-since", 0, "add a session volume figure from this time (unix ms)")
	return cmd
}
