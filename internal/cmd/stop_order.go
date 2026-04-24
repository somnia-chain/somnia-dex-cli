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
		Use:   "stoporder",
		Short: "Manage stop orders",
		Long: `Place, list, and cancel conditional stop orders.

Stop orders activate only when the market price crosses a trigger threshold. Use
them for stop-loss protection (sell if price drops below a level) or breakout
entries (buy if price rises above resistance). Once triggered, the stop order
becomes a regular market or limit order.`,
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
		Long: `Place a conditional stop order that activates when the market price crosses the
trigger threshold. Requires --trigger-price and --trigger-operator (gte for >=,
lte for <=).

Examples:
  Stop-loss:      --side sell --trigger-operator lte --trigger-price 0.15
  Breakout entry: --side buy  --trigger-operator gte --trigger-price 0.25`,
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

			_, err = a.eth.SignAndSend(tx, "Stop order")
			return err
		},
	}
	f := cmd.Flags()
	f.String("side", "", "buy or sell (required)")
	f.String("type", "market", "market or limit")
	f.String("amount", "", "order amount (required)")
	f.String("trigger-price", "", "price that activates the order (required)")
	f.String("trigger-operator", "", "gte or lte (required)")
	f.String("price", "", "limit price (required for limit type)")
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
		Long: `List stop orders for one or all markets. Optionally filter by status: pending,
triggered, canceled, or failed.`,
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "List stop orders",
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
	cmd.Flags().String("status", "", "filter: pending, triggered, canceled, failed")
	return cmd
}

// stopOrderCancelCmd cancels a pending stop order on-chain.
func (a *app) stopOrderCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <symbol> <id>",
		Short: "Cancel a pending stop order",
		Long:  "Cancel a pending stop order before it triggers. Signs and submits a cancellation transaction on-chain.",
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
			_, err = a.eth.SignAndSend(tx, "Stop order cancel")
			return err
		},
	}
	return cmd
}
