package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

// orderCmd returns the "order" parent command for managing orders.
func (a *app) orderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: "Manage orders",
	}
	cmd.AddCommand(
		a.orderPlaceCmd(),
		a.orderListCmd(),
		a.orderGetCmd(),
		a.orderCancelCmd(),
		a.orderReduceCmd(),
	)
	return cmd
}

// orderPlaceCmd returns the "order place" command, which prepares, signs, and sends a new order.
func (a *app) orderPlaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "place <symbol>",
		Short: "Place a new order",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Place order",
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
			price, _ := cmd.Flags().GetString("price")
			orderType, _ := cmd.Flags().GetString("order-type")
			fundingSource, _ := cmd.Flags().GetString("funding-source")

			if typ == "limit" && price == "" {
				return fmt.Errorf("--price is required for limit orders")
			}

			tx, err := a.client.PrepareOrder(args[0], &api.PrepareOrderRequest{
				Type:          typ,
				Side:          side,
				Amount:        amount,
				WalletAddress: addr.Hex(),
				Price:         price,
				OrderType:     orderType,
				FundingSource: fundingSource,
			})
			if err != nil {
				return fmt.Errorf("prepare order: %w", err)
			}

			if tx.Approval != nil {
				fmt.Printf("Token approval required before this order can execute:\n")
				fmt.Printf("  Token:  %s\n", tx.Approval.Token)
				fmt.Printf("  Amount: %s\n", tx.Approval.Amount)
				fmt.Println("\nApprove with: dreamdex vault approve <symbol> --currency <code> --amount <amount>")
				fmt.Println("Then retry this order.")
				return nil
			}

			wait, _ := cmd.Flags().GetBool("wait")
			return signAndSend(cmd, key, tx, wait)
		},
	}
	f := cmd.Flags()
	f.String("side", "", "buy or sell (required)")
	f.String("type", "market", "market or limit")
	f.String("amount", "", "order amount (required)")
	f.String("price", "", "limit price (required for limit orders)")
	f.String("order-type", "", "normalOrder, fillOrKill, immediateOrCancel, postOnly")
	f.String("funding-source", "", "wallet or vault")
	f.Bool("wait", false, "wait for transaction confirmation")
	cmd.MarkFlagRequired("side")
	cmd.MarkFlagRequired("amount")
	return cmd
}

// orderListCmd returns the "order list" command, which lists orders for one or all markets.
func (a *app) orderListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [symbol]",
		Short: "List orders (all markets if no symbol given)",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "List orders",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			symbols, err := a.resolveSymbols(args)
			if err != nil {
				return err
			}
			status, _ := cmd.Flags().GetString("status")
			var all []api.Order
			for _, sym := range symbols {
				orders, err := a.client.GetOrders(sym, status)
				if err != nil {
					return err
				}
				all = append(all, orders...)
			}
			if isJSON(cmd) {
				return printJSON(all)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SYMBOL\tID\tSTATUS\tTYPE\tSIDE\tPRICE\tAMOUNT\tFILLED\tREMAINING")
			for _, o := range all {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					o.Symbol, o.ID, o.Status, o.Type, o.Side, o.Price, o.Amount, o.Filled, o.Remaining)
			}
			return w.Flush()
		},
	}
	cmd.Flags().String("status", "", "filter: open, closed, canceled, expired, rejected")
	return cmd
}

// orderGetCmd returns the "order get" command, which shows details for a single order.
func (a *app) orderGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <symbol> <id>",
		Short: "Get order details",
		Args:  cobra.ExactArgs(2),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get order",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := a.client.GetOrder(args[0], args[1])
			if err != nil {
				return err
			}
			if isJSON(cmd) {
				return printJSON(o)
			}
			fmt.Printf("Order:     %s\n", o.ID)
			fmt.Printf("Status:    %s\n", o.Status)
			fmt.Printf("Type:      %s %s\n", o.Side, o.Type)
			fmt.Printf("Price:     %s\n", o.Price)
			fmt.Printf("Amount:    %s\n", o.Amount)
			fmt.Printf("Filled:    %s\n", o.Filled)
			fmt.Printf("Remaining: %s\n", o.Remaining)
			if o.TxHash != "" {
				fmt.Printf("Tx Hash:   %s\n", o.TxHash)
			}
			fmt.Printf("Created:   %s\n", time.UnixMilli(o.CreatedAt).Format(time.RFC3339))
			return nil
		},
	}
}

// orderCancelCmd returns the "order cancel" command, which cancels an open order.
func (a *app) orderCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <symbol> <id>",
		Short: "Cancel an open order",
		Args:  cobra.ExactArgs(2),
		Annotations: map[string]string{
			ophis.AnnotationTitle:       "Cancel order",
			ophis.AnnotationDestructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := a.client.CancelOrder(args[0], args[1])
			if err != nil {
				return err
			}
			if isJSON(cmd) {
				return printJSON(o)
			}
			fmt.Printf("Order %s canceled (status: %s)\n", o.ID, o.Status)
			return nil
		},
	}
}

// orderReduceCmd returns the "order reduce" command, which reduces an order's remaining quantity.
func (a *app) orderReduceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reduce <symbol> <id>",
		Short: "Reduce an open order's remaining quantity",
		Args:  cobra.ExactArgs(2),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Reduce order",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := privateKey()
			if err != nil {
				return err
			}
			qty, _ := cmd.Flags().GetString("quantity")
			tx, err := a.client.ReduceOrder(args[0], args[1], qty)
			if err != nil {
				return err
			}
			wait, _ := cmd.Flags().GetBool("wait")
			return signAndSend(cmd, key, tx, wait)
		},
	}
	cmd.Flags().String("quantity", "", "new remaining quantity (required)")
	cmd.Flags().Bool("wait", false, "wait for transaction confirmation")
	cmd.MarkFlagRequired("quantity")
	return cmd
}
