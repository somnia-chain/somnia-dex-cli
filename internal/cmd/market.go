package cmd

import (
	"fmt"
	"time"

	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

// marketsCmd returns the "markets" command, which lists all trading pairs.
func (a *app) marketsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "markets",
		Short: "List all trading pairs",
		Long: `List all available trading pairs on dreamDEX. Shows each market's symbol,
contract address, base/quote tokens, tick size, lot size, and minimum order quantity.`,
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "List markets",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			markets, err := a.client.GetMarkets()
			if err != nil {
				return err
			}
			return printResult(cmd, api.Markets{Markets: markets})
		},
	}
}

// currenciesCmd returns the "currencies" command, which lists all supported tokens.
func (a *app) currenciesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "currencies",
		Short: "List all supported currencies",
		Long: `List all tokens supported by dreamDEX. Shows each currency's code, name,
decimal precision, and contract address.`,
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "List currencies",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			currencies, err := a.client.GetCurrencies()
			if err != nil {
				return err
			}
			return printResult(cmd, api.Currencies{Currencies: currencies})
		},
	}
}

// orderbookCmd returns the "orderbook" command, which shows bids and asks for one or all markets.
func (a *app) orderbookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orderbook <symbol>",
		Short: "Show order book",
		Long: `Display the order book for a market, showing current bid and ask price levels
with their quantities. Use --depth to limit the number of levels shown per side.`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Show order book",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			depth, _ := cmd.Flags().GetInt("depth")
			books, err := a.client.GetOrderBooks(args, depth)
			if err != nil {
				return err
			}
			return printResult(cmd, api.OrderBooks{OrderBooks: books})
		},
	}
	cmd.Flags().Int("depth", 0, "number of price levels to show")
	return cmd
}

// tickerCmd returns the "ticker" command, which shows 24h market statistics.
func (a *app) tickerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ticker [symbol]",
		Short: "Show 24h market statistics (all markets if no symbol given)",
		Long: `Show 24-hour OHLCV statistics for one or all markets, including open, high,
low, close prices, and total trading volume.`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get 24h ticker",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			symbols, err := a.resolveSymbols(args)
			if err != nil {
				return err
			}
			var all []api.Ticker
			for _, sym := range symbols {
				tickers, err := a.client.GetTicker(sym)
				if err != nil {
					return err
				}
				all = append(all, tickers...)
			}
			if isJSON(cmd) {
				return printJSON(struct {
					Tickers []api.Ticker `json:"tickers"`
				}{all})
			}
			for _, t := range all {
				fmt.Printf("%s  O:%s  H:%s  L:%s  C:%s  V:%s  %s\n",
					t.Symbol, t.Open, t.High, t.Low, t.Close, t.Volume,
					time.UnixMilli(t.Timestamp).Format(time.RFC3339))
			}
			return nil
		},
	}
}

// tradesCmd returns the "trades" command, which shows recent trades.
func (a *app) tradesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trades [symbol]",
		Short: "Show recent trades (all markets if no symbol given)",
		Long: `Show recently executed trades for one or all markets. Each trade includes the
price, quantity, and side (buy/sell).`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "List recent trades",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			symbols, err := a.resolveSymbols(args)
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetInt("limit")
			var all []api.Trade
			for _, sym := range symbols {
				trades, err := a.client.GetTrades(sym, 0, limit)
				if err != nil {
					return err
				}
				all = append(all, trades...)
			}
			return printResult(cmd, api.Trades{Trades: all})
		},
	}
	cmd.Flags().Int("limit", 20, "max trades to return")
	return cmd
}

// candlesCmd returns the "candles" command, which shows OHLCV candle data.
func (a *app) candlesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "candles <symbol>",
		Short: "Show OHLCV candle data",
		Long: `Show OHLCV candlestick data for a market at a specified time interval. Useful
for analysing price trends over time.`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get OHLCV candles",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			interval, _ := cmd.Flags().GetString("interval")
			limit, _ := cmd.Flags().GetInt("limit")
			candles, err := a.client.GetCandles(args[0], interval, limit)
			if err != nil {
				return err
			}
			return printResult(cmd, api.Candles{Candles: candles})
		},
	}
	cmd.Flags().String("interval", "1h", "candle interval (1m, 5m, 15m, 1h, 4h, 1d)")
	cmd.Flags().Int("limit", 20, "max candles to return")
	return cmd
}
