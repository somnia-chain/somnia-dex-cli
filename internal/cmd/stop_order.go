package cmd

import (
	"fmt"
	"os"
	"slices"
	"text/tabwriter"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

// stopOrderCmd returns the "stop-order" parent command.
func (a *app) stopOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop-order",
		Short: "Manage stop orders",
	}
	cmd.AddCommand(
		a.stopOrderPlaceCmd(),
		a.stopOrderListCmd(),
		a.stopOrderCancelCmd(),
	)
	return cmd
}

// stopOrderPlaceCmd prepares, signs, and sends a new stop order.
func (a *app) stopOrderPlaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "place <symbol>",
		Short: "Place a new stop order",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Place stop order",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := privateKey()
			if err != nil {
				return err
			}
			addr := crypto.PubkeyToAddress(key.PublicKey)

			side, _ := cmd.Flags().GetString("side")
			typ, _ := cmd.Flags().GetString("type")
			amount, _ := cmd.Flags().GetString("amount")
			triggerPrice, _ := cmd.Flags().GetString("trigger-price")
			triggerOp, _ := cmd.Flags().GetString("trigger-operator")
			price, _ := cmd.Flags().GetString("price")

			if typ == "limit" && price == "" {
				return fmt.Errorf("--price is required for limit stop orders")
			}

			tx, err := a.client.PrepareStopOrder(args[0], &api.PrepareStopOrderRequest{
				Type:            typ,
				Side:            side,
				Amount:          amount,
				TriggerPrice:    triggerPrice,
				TriggerOperator: triggerOp,
				WalletAddress:   addr.Hex(),
				Price:           price,
			})
			if err != nil {
				return fmt.Errorf("prepare stop order: %w", err)
			}

			if tx.OrderID != "" {
				fmt.Printf("Stop Order ID: %s\n", tx.OrderID)
			}
			wait, _ := cmd.Flags().GetBool("wait")
			return a.signAndSend(cmd, key, tx, wait, "Stop order")
		},
	}
	f := cmd.Flags()
	f.String("side", "", "buy or sell (required)")
	f.String("type", "market", "market or limit")
	f.String("amount", "", "order amount (required)")
	f.String("trigger-price", "", "price that activates the order (required)")
	f.String("trigger-operator", "", "gte or lte (required)")
	f.String("price", "", "limit price (required for limit type)")
	f.Bool("wait", false, "wait for transaction confirmation")
	cmd.MarkFlagRequired("side")
	cmd.MarkFlagRequired("amount")
	cmd.MarkFlagRequired("trigger-price")
	cmd.MarkFlagRequired("trigger-operator")
	return cmd
}

// stopOrderListCmd lists stop orders for one or all markets.
func (a *app) stopOrderListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [symbol]",
		Short: "List stop orders (all markets if no symbol given)",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "List stop orders",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			symbols, err := a.resolveSymbols(args)
			if err != nil {
				return err
			}
			status, _ := cmd.Flags().GetString("status")
			var all []api.StopOrder
			for _, sym := range symbols {
				orders, err := a.client.GetStopOrders(sym, status)
				if err != nil {
					return err
				}
				all = append(all, orders...)
			}
			slices.SortFunc(all, func(a, b api.StopOrder) int {
				return int(a.CreatedAt - b.CreatedAt)
			})
			if isJSON(cmd) {
				return printJSON(all)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SYMBOL\tID\tCREATED\tSTATUS\tTYPE\tSIDE\tTRIGGER\tOPERATOR")
			for _, o := range all {
				created := time.UnixMilli(o.CreatedAt).Format("2006-01-02 15:04:05")
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					o.Symbol, o.ID, created, o.Status, o.Type, o.Side, o.TriggerPrice, o.TriggerOperator)
			}
			return w.Flush()
		},
	}
	cmd.Flags().String("status", "", "filter: pending, triggered, cancelled, failed")
	return cmd
}

// stopOrderCancelCmd cancels a pending stop order on-chain.
func (a *app) stopOrderCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <symbol> <id>",
		Short: "Cancel a pending stop order",
		Args:  cobra.ExactArgs(2),
		Annotations: map[string]string{
			ophis.AnnotationTitle:       "Cancel stop order",
			ophis.AnnotationDestructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := privateKey()
			if err != nil {
				return err
			}
			tx, err := a.client.CancelStopOrder(args[0], args[1])
			if err != nil {
				return err
			}
			wait, _ := cmd.Flags().GetBool("wait")
			return a.signAndSend(cmd, key, tx, wait, "Stop order cancel")
		},
	}
	cmd.Flags().Bool("wait", false, "wait for transaction confirmation")
	return cmd
}
