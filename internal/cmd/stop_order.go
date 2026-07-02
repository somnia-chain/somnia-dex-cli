package cmd

import (
	"fmt"

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
		a.stopOrderAuthorizationCmd(),
		a.stopOrderApproveCmd(),
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
		Args: cobra.ExactArgs(1),
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

			if err := a.ensureStopOrderAuthorized(args[0]); err != nil {
				return err
			}

			tx, err := a.client.PrepareStopOrder(args[0], &api.PrepareStopOrderRequest{
				Type:            typ,
				Side:            side,
				Amount:          amount,
				TriggerPrice:    triggerPrice,
				TriggerOperator: triggerOp,
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
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "List stop orders",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			status, _ := cmd.Flags().GetString("status")
			limit, _ := cmd.Flags().GetInt("limit")
			cursor, _ := cmd.Flags().GetString("cursor")
			orders, next, err := a.client.GetAllStopOrders(args, status, limit, cursor)
			if err != nil {
				return err
			}
			if orders == nil {
				orders = []api.StopOrder{}
			}
			printCursorHint(cmd, next)
			return printResult(cmd, api.StopOrders{StopOrders: orders})
		},
	}
	cmd.Flags().String("status", "", "filter: pending, triggered, canceled, failed")
	cmd.Flags().Int("limit", 0, "max stop orders per page (server default 100, max 1000)")
	cmd.Flags().String("cursor", "", "pagination cursor from a previous page")
	return cmd
}

// stopOrderAuthorizationCmd reports whether the wallet has authorized the stop-order operator.
func (a *app) stopOrderAuthorizationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "authorization <symbol>",
		Short: "Show stop-order operator authorization",
		Long:  "Report whether the wallet has granted the market's stop-order registry operator permission to place orders on its behalf.",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get stop-order authorization",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			authorized, err := a.client.GetStopOrderAuthorization(args[0])
			if err != nil {
				return err
			}
			if isJSON(cmd) {
				return printJSON(struct {
					Authorized bool `json:"authorized"`
				}{authorized})
			}
			fmt.Printf("authorized: %t\n", authorized)
			return nil
		},
	}
}

// stopOrderApproveCmd grants the stop-order operator permission to place orders for the wallet.
func (a *app) stopOrderApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve <symbol>",
		Short: "Authorize the stop-order operator",
		Long:  "Grant the market's stop-order registry operator permission to place orders on your behalf. Required once per market before creating stop orders.",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Approve stop-order operator",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireEth(cmd); err != nil {
				return err
			}
			tx, err := a.client.PrepareStopOrderApproval(args[0])
			if err != nil {
				return err
			}
			_, err = a.eth.SignAndSend(tx, "Stop-order approval")
			return err
		},
	}
}

// ensureStopOrderAuthorized authorizes the stop-order operator on-chain if not already granted.
func (a *app) ensureStopOrderAuthorized(symbol string) error {
	authorized, err := a.client.GetStopOrderAuthorization(symbol)
	if err != nil {
		return fmt.Errorf("check stop-order authorization: %w", err)
	}
	if authorized {
		return nil
	}
	tx, err := a.client.PrepareStopOrderApproval(symbol)
	if err != nil {
		return fmt.Errorf("prepare stop-order approval: %w", err)
	}
	if _, err := a.eth.SignAndSend(tx, "Stop-order approval"); err != nil {
		return fmt.Errorf("stop-order approval: %w", err)
	}
	return nil
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
