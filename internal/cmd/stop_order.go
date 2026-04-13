package cmd

import (
	"fmt"
	"slices"

	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

// stopOrderCmd returns the "stop-order" parent command.
func (a *app) stopOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop-order",
		Short: "Manage stop orders",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Root().PersistentPreRunE(cmd, args); err != nil {
				return err
			}
			return a.requireAuth(cmd)
		},
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
				WalletAddress:   a.eth.Address().Hex(),
				Price:           price,
			})
			if err != nil {
				return fmt.Errorf("prepare stop order: %w", err)
			}

			if tx.OrderID != "" {
				fmt.Printf("Stop Order ID: %s\n", tx.OrderID)
			}
			wait, _ := cmd.Flags().GetBool("wait")
			return a.eth.SignAndSend(tx, wait, "Stop order")
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
			if all == nil {
				all = []api.StopOrder{}
			}
			return printResult(cmd, api.StopOrders{StopOrders: all})
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
			tx, err := a.client.CancelStopOrder(args[0], args[1])
			if err != nil {
				return err
			}
			wait, _ := cmd.Flags().GetBool("wait")
			return a.eth.SignAndSend(tx, wait, "Stop order cancel")
		},
	}
	cmd.Flags().Bool("wait", false, "wait for transaction confirmation")
	return cmd
}
