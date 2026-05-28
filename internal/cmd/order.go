package cmd

import (
	"fmt"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
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
		Long: `Place, list, cancel, and reduce orders on dreamDEX.

Orders are instructions to buy or sell tokens. Market orders execute immediately at
the best available price. Limit orders rest on the order book at a specified price
until filled, canceled, or expired.`,
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
		Long: `Place a new order on dreamDEX. Market orders (default) execute immediately as
limit IOC orders priced from the current order book with a slippage tolerance.
Limit orders rest on the book at the specified price.

Sub-types control execution: normalOrder (rests until filled), fillOrKill (fill
entirely or cancel), immediateOrCancel (fill what you can, cancel the rest),
postOnly (only accepted if it rests on the book).

The CLI handles token approval, signing, and transaction submission automatically.`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Place order",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			side, _ := cmd.Flags().GetString("side")
			typ, _ := cmd.Flags().GetString("type")
			amount, _ := cmd.Flags().GetString("amount")
			price, _ := cmd.Flags().GetString("price")
			orderType, _ := cmd.Flags().GetString("order-type")
			fundingSource, _ := cmd.Flags().GetString("funding-source")
			slippage, _ := cmd.Flags().GetFloat64("slippage")

			return a.placeOrder(cmd, args[0], side, typ, amount, price, orderType, fundingSource, slippage)
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
	cmd.MarkFlagRequired("side")
	cmd.MarkFlagRequired("amount")
	return cmd
}

// orderListCmd returns the "order list" command, which lists orders for one or all markets.
func (a *app) orderListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [symbol]",
		Short: "List orders (all markets if no symbol given)",
		Long: `List orders for one or all markets. Optionally filter by status: open, closed,
canceled, expired, or rejected.`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "List orders",
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
			slices.SortFunc(all, func(a, b api.Order) int {
				return int(a.CreatedAt - b.CreatedAt)
			})
			return printResult(cmd, api.Orders{Orders: all})
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
		Long:  "Show full details for a single order, including status, fill progress, and transaction hash.",
		Args:  cobra.ExactArgs(2),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Get order",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := a.client.GetOrder(args[0], args[1])
			if err != nil {
				return err
			}
			if isJSON(cmd) {
				return printJSON(o)
			}
			fmt.Printf("Symbol:    %s\n", o.Symbol)
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
		Long:  "Cancel an open order. Signs and submits a cancellation transaction on-chain.",
		Args:  cobra.ExactArgs(2),
		Annotations: map[string]string{
			ophis.AnnotationTitle:       "Cancel order",
			ophis.AnnotationDestructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireEth(cmd); err != nil {
				return err
			}
			tx, err := a.client.CancelOrder(args[0], args[1])
			if err != nil {
				return err
			}
			_, err = a.eth.SignAndSend(tx, "Cancel")
			return err
		},
	}
	return cmd
}

// orderReduceCmd returns the "order reduce" command, which reduces an order's remaining quantity.
func (a *app) orderReduceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reduce <symbol> <id>",
		Short: "Reduce an open order's remaining quantity",
		Long: `Reduce an open order's remaining quantity without cancelling it. Signs and
submits an amendment transaction on-chain.`,
		Args: cobra.ExactArgs(2),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Reduce order",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireEth(cmd); err != nil {
				return err
			}
			qty, _ := cmd.Flags().GetString("quantity")
			tx, err := a.client.ReduceOrder(args[0], args[1], qty)
			if err != nil {
				return err
			}
			_, err = a.eth.SignAndSend(tx, "Order reduce")
			return err
		},
	}
	cmd.Flags().String("quantity", "", "new remaining quantity (required)")
	cmd.MarkFlagRequired("quantity")
	return cmd
}

// orderPlacedTopic is the keccak256 of the OrderPlaced event signature.
var orderPlacedTopic = crypto.Keccak256Hash(
	[]byte("OrderPlaced(uint128,(uint128,bool,address,uint64,uint256,uint256,uint256,uint64))"),
)

// orderIDFromReceipt extracts the order ID from an OrderPlaced event in the receipt.
func orderIDFromReceipt(receipt *types.Receipt) string {
	for _, log := range receipt.Logs {
		if len(log.Topics) >= 2 && log.Topics[0] == orderPlacedTopic {
			return new(big.Int).SetBytes(log.Topics[1].Bytes()).String()
		}
	}
	return ""
}

// placeOrder is the shared implementation for order placement used by both
// "order place" and the "buy"/"sell" shorthand commands.
func (a *app) placeOrder(cmd *cobra.Command, symbol, side, typ, amount, price, orderType, fundingSource string, slippage float64) error {
	if err := a.requireEth(cmd); err != nil {
		return err
	}

	if typ == "limit" && price == "" {
		return fmt.Errorf("--price is required for limit orders")
	}

	// Market orders: convert to limit IOC with orderbook-derived price.
	if typ == "market" {
		p, err := marketPrice(a.client, symbol, side, slippage)
		if err != nil {
			return fmt.Errorf("determine market price: %w", err)
		}
		price = p
		typ = "limit"
		if orderType == "" {
			orderType = "immediateOrCancel"
		}
	}

	wallet := a.eth.Address().Hex()
	tx, err := a.client.PrepareOrder(symbol, &api.PrepareOrderRequest{
		Type:          typ,
		Side:          side,
		Amount:        amount,
		WalletAddress: wallet,
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
		approveTx, err := a.client.PrepareApproval(symbol, &api.VaultActionRequest{
			WalletAddress: wallet,
			Currency:      code,
			Amount:        tx.Approval.Amount,
		})
		if err != nil {
			return fmt.Errorf("prepare approval: %w", err)
		}
		approveLabel := fmt.Sprintf("Approval (%s %s)", tx.Approval.Amount, code)
		if _, err := a.eth.SignAndSend(approveTx, approveLabel); err != nil {
			return fmt.Errorf("token approval: %w", err)
		}
	}

	receipt, err := a.eth.SignAndSend(tx, "Order")
	if err != nil {
		return err
	}
	if id := orderIDFromReceipt(receipt); id != "" {
		fmt.Printf("Order ID: %s\n", id)
		return nil
	}
	return &ExitError{Code: ExitNoFill, Err: fmt.Errorf("order was not placed (no fills available)")}
}

// buyCmd returns the top-level "buy" shorthand command.
func (a *app) buyCmd() *cobra.Command {
	return a.tradeShorthand("buy")
}

// sellCmd returns the top-level "sell" shorthand command.
func (a *app) sellCmd() *cobra.Command {
	return a.tradeShorthand("sell")
}

// tradeShorthand builds a top-level buy/sell command: "dreamdex buy 100 STT/USDT".
func (a *app) tradeShorthand(side string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <amount> <symbol>", side),
		Short: fmt.Sprintf("Place a %s order (shorthand for 'order place')", side),
		Long: fmt.Sprintf(`Shorthand for "dreamdex order place <symbol> --side %s --amount <amount>".

Defaults to a market order. Pass --price to place a limit order instead.
All other order place flags are supported.`, side),
		Args: cobra.ExactArgs(2),
		Annotations: map[string]string{
			ophis.AnnotationTitle: strings.ToUpper(side[:1]) + side[1:] + " order",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			amount, symbol := args[0], args[1]
			price, _ := cmd.Flags().GetString("price")
			orderType, _ := cmd.Flags().GetString("order-type")
			fundingSource, _ := cmd.Flags().GetString("funding-source")
			slippage, _ := cmd.Flags().GetFloat64("slippage")

			typ := "market"
			if price != "" {
				typ = "limit"
			}

			return a.placeOrder(cmd, symbol, side, typ, amount, price, orderType, fundingSource, slippage)
		},
	}
	f := cmd.Flags()
	f.String("price", "", "limit price (omit for market order)")
	f.String("order-type", "", "normalOrder, fillOrKill, immediateOrCancel, postOnly")
	f.String("funding-source", "", "wallet or vault")
	f.Float64("slippage", 0.5, "slippage tolerance for market orders (percent)")
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
