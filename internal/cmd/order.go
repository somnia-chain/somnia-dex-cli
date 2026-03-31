package cmd

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
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
			slippage, _ := cmd.Flags().GetFloat64("slippage")

			if typ == "limit" && price == "" {
				return fmt.Errorf("--price is required for limit orders")
			}

			// Market orders: convert to limit IOC with orderbook-derived price.
			if typ == "market" {
				p, err := marketPrice(a.client, args[0], side, slippage)
				if err != nil {
					return fmt.Errorf("determine market price: %w", err)
				}
				price = p
				typ = "limit"
				if orderType == "" {
					orderType = "immediateOrCancel"
				}
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
				code := tx.Approval.Token
				if currencies, err := a.client.GetCurrencies(); err == nil {
					for _, c := range currencies {
						if strings.EqualFold(c.ID, tx.Approval.Token) {
							code = c.Code
							break
						}
					}
				}
				approveTx, err := a.client.PrepareApproval(args[0], &api.VaultActionRequest{
					WalletAddress: addr.Hex(),
					Currency:      code,
					Amount:        tx.Approval.Amount,
				})
				if err != nil {
					return fmt.Errorf("prepare approval: %w", err)
				}
				approveLabel := fmt.Sprintf("Approval (%s %s)", tx.Approval.Amount, code)
				if err := signAndSend(cmd, key, approveTx, true, approveLabel); err != nil {
					return fmt.Errorf("token approval: %w", err)
				}
			}

			wait, _ := cmd.Flags().GetBool("wait")
			return signAndSend(cmd, key, tx, wait, "Order")
		},
	}
	f := cmd.Flags()
	f.String("side", "", "buy or sell (required)")
	f.String("type", "market", "market or limit")
	f.String("amount", "", "order amount (required)")
	f.String("price", "", "limit price (required for limit orders)")
	f.String("order-type", "", "normalOrder, fillOrKill, immediateOrCancel, postOnly")
	f.String("funding-source", "", "wallet or vault")
	f.Float64("slippage", 0.5, "slippage tolerance for market orders (percent)")
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
			fmt.Fprintln(w, "SYMBOL\tID\tCREATED\tSTATUS\tTYPE\tSIDE\tPRICE\tAMOUNT\tFILLED\tREMAINING")
			for _, o := range all {
				created := time.UnixMilli(o.CreatedAt).Format("2006-01-02 15:04:05")
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					o.Symbol, o.ID, created, o.Status, o.Type, o.Side, o.Price, o.Amount, o.Filled, o.Remaining)
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

// orderCancelCmd returns the "order cancel" command, which cancels an open order on-chain.
func (a *app) orderCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <symbol> <id>",
		Short: "Cancel an open order",
		Args:  cobra.ExactArgs(2),
		Annotations: map[string]string{
			ophis.AnnotationTitle:       "Cancel order",
			ophis.AnnotationDestructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := privateKey()
			if err != nil {
				return err
			}
			tx, err := a.client.CancelOrder(args[0], args[1])
			if err != nil {
				return err
			}
			wait, _ := cmd.Flags().GetBool("wait")
			return signAndSend(cmd, key, tx, wait, "Cancel")
		},
	}
	cmd.Flags().Bool("wait", false, "wait for transaction confirmation")
	return cmd
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
			return signAndSend(cmd, key, tx, wait, "Order reduce")
		},
	}
	cmd.Flags().String("quantity", "", "new remaining quantity (required)")
	cmd.Flags().Bool("wait", false, "wait for transaction confirmation")
	cmd.MarkFlagRequired("quantity")
	return cmd
}

// marketPrice derives a worst-case price for a market order from the orderbook.
// For buys it uses the best ask + slippage; for sells, best bid - slippage.
// The result is rounded to the market's tick size.
func marketPrice(c *api.Client, symbol, side string, slippagePct float64) (string, error) {
	books, err := c.GetOrderBooks([]string{symbol}, 1)
	if err != nil {
		return "", fmt.Errorf("fetch orderbook: %w", err)
	}
	if len(books) == 0 {
		return "", fmt.Errorf("no orderbook for %s", symbol)
	}
	book := books[0]

	var bestPrice float64
	switch side {
	case "buy":
		if len(book.Asks) == 0 {
			return "", fmt.Errorf("no asks in orderbook for %s", symbol)
		}
		bestPrice, _ = strconv.ParseFloat(book.Asks[0].Price, 64)
		bestPrice *= 1 + slippagePct/100
	case "sell":
		if len(book.Bids) == 0 {
			return "", fmt.Errorf("no bids in orderbook for %s", symbol)
		}
		bestPrice, _ = strconv.ParseFloat(book.Bids[0].Price, 64)
		bestPrice *= 1 - slippagePct/100
	default:
		return "", fmt.Errorf("invalid side: %s", side)
	}

	// Round to tick size.
	markets, err := c.GetMarkets()
	if err != nil {
		return "", fmt.Errorf("fetch markets: %w", err)
	}
	tickSize := 0.0
	for _, m := range markets {
		if m.Symbol == symbol {
			tickSize, _ = strconv.ParseFloat(m.TickSize, 64)
			break
		}
	}
	if tickSize <= 0 {
		return "", fmt.Errorf("unknown tick size for %s", symbol)
	}

	// Round: buys up to next tick, sells down.
	var rounded float64
	if side == "buy" {
		rounded = math.Ceil(bestPrice/tickSize) * tickSize
	} else {
		rounded = math.Floor(bestPrice/tickSize) * tickSize
	}

	// Format with enough decimals to represent the tick size.
	decimals := 0
	if s := strconv.FormatFloat(tickSize, 'f', -1, 64); strings.Contains(s, ".") {
		decimals = len(s) - strings.Index(s, ".") - 1
	}
	return strconv.FormatFloat(rounded, 'f', decimals, 64), nil
}

