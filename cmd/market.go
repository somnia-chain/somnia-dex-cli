package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/njayp/ophis"
	"github.com/somnia-chain/somnia-dex-cli/internal/api"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(marketsCmd, currenciesCmd, orderbookCmd, tickerCmd, tradesCmd, candlesCmd)

	orderbookCmd.Flags().Int("depth", 0, "number of price levels to show")
	tradesCmd.Flags().Int("limit", 20, "max trades to return")
	candlesCmd.Flags().String("interval", "1h", "candle interval (1m, 5m, 15m, 1h, 4h, 1d)")
	candlesCmd.Flags().Int("limit", 20, "max candles to return")
}

var marketsCmd = &cobra.Command{
	Use:   "markets",
	Short: "List all trading pairs",
	Annotations: map[string]string{
		ophis.AnnotationReadOnly: "true",
		ophis.AnnotationTitle:    "List markets",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		markets, err := client.GetMarkets()
		if err != nil {
			return err
		}
		if isJSON(cmd) {
			return printJSON(markets)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SYMBOL\tCONTRACT\tBASE\tQUOTE\tTICK SIZE\tLOT SIZE\tMIN QTY")
		for _, m := range markets {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				m.Symbol, m.Contract, m.Base, m.Quote, m.TickSize, m.LotSize, m.MinQuantity)
		}
		return w.Flush()
	},
}

var currenciesCmd = &cobra.Command{
	Use:   "currencies",
	Short: "List all supported currencies",
	Annotations: map[string]string{
		ophis.AnnotationReadOnly: "true",
		ophis.AnnotationTitle:    "List currencies",
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		currencies, err := client.GetCurrencies()
		if err != nil {
			return err
		}
		if isJSON(cmd) {
			return printJSON(currencies)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CODE\tNAME\tDECIMALS\tADDRESS")
		for _, c := range currencies {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", c.Code, c.Name, c.Decimals, c.ID)
		}
		return w.Flush()
	},
}

var orderbookCmd = &cobra.Command{
	Use:   "orderbook [symbol]",
	Short: "Show order book (all markets if no symbol given)",
	Args:  cobra.MaximumNArgs(1),
	Annotations: map[string]string{
		ophis.AnnotationReadOnly: "true",
		ophis.AnnotationTitle:    "Show order book",
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		symbols, err := resolveSymbols(args)
		if err != nil {
			return err
		}
		depth, _ := cmd.Flags().GetInt("depth")
		books, err := client.GetOrderBooks(symbols, depth)
		if err != nil {
			return err
		}
		if isJSON(cmd) {
			return printJSON(books)
		}
		for i, book := range books {
			if i > 0 {
				fmt.Println()
			}
			fmt.Println(book.Symbol)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for j := len(book.Asks) - 1; j >= 0; j-- {
				fmt.Fprintf(w, "  ask\t%s\t%s\n", book.Asks[j].Price, book.Asks[j].Quantity)
			}
			fmt.Fprintln(w, "  \t────\t────")
			for _, b := range book.Bids {
				fmt.Fprintf(w, "  bid\t%s\t%s\n", b.Price, b.Quantity)
			}
			w.Flush()
		}
		return nil
	},
}

var tickerCmd = &cobra.Command{
	Use:   "ticker [symbol]",
	Short: "Show 24h market statistics (all markets if no symbol given)",
	Args:  cobra.MaximumNArgs(1),
	Annotations: map[string]string{
		ophis.AnnotationReadOnly: "true",
		ophis.AnnotationTitle:    "Get 24h ticker",
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		symbols, err := resolveSymbols(args)
		if err != nil {
			return err
		}
		var all []api.Ticker
		for _, sym := range symbols {
			tickers, err := client.GetTicker(sym)
			if err != nil {
				return err
			}
			all = append(all, tickers...)
		}
		if isJSON(cmd) {
			return printJSON(all)
		}
		for _, t := range all {
			fmt.Printf("%s  O:%s  H:%s  L:%s  C:%s  V:%s  %s\n",
				t.Symbol, t.Open, t.High, t.Low, t.Close, t.Volume,
				time.UnixMilli(t.Timestamp).Format(time.RFC3339))
		}
		return nil
	},
}

var tradesCmd = &cobra.Command{
	Use:   "trades [symbol]",
	Short: "Show recent trades (all markets if no symbol given)",
	Args:  cobra.MaximumNArgs(1),
	Annotations: map[string]string{
		ophis.AnnotationReadOnly: "true",
		ophis.AnnotationTitle:    "List recent trades",
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		symbols, err := resolveSymbols(args)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		var all []api.Trade
		for _, sym := range symbols {
			trades, err := client.GetTrades(sym, 0, limit)
			if err != nil {
				return err
			}
			all = append(all, trades...)
		}
		if isJSON(cmd) {
			return printJSON(all)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SYMBOL\tTIME\tSIDE\tPRICE\tAMOUNT\tCOST")
		for _, t := range all {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				t.Symbol,
				time.UnixMilli(t.Timestamp).Format(time.RFC3339),
				t.Side, t.Price, t.Amount, t.Cost)
		}
		return w.Flush()
	},
}

var candlesCmd = &cobra.Command{
	Use:   "candles [symbol]",
	Short: "Show OHLCV candle data (all markets if no symbol given)",
	Args:  cobra.MaximumNArgs(1),
	Annotations: map[string]string{
		ophis.AnnotationReadOnly: "true",
		ophis.AnnotationTitle:    "Get OHLCV candles",
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		symbols, err := resolveSymbols(args)
		if err != nil {
			return err
		}
		interval, _ := cmd.Flags().GetString("interval")
		limit, _ := cmd.Flags().GetInt("limit")
		var all []api.Candle
		for _, sym := range symbols {
			candles, err := client.GetCandles(sym, interval, limit)
			if err != nil {
				return err
			}
			all = append(all, candles...)
		}
		if isJSON(cmd) {
			return printJSON(all)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TIME\tOPEN\tHIGH\tLOW\tCLOSE\tVOLUME")
		for _, c := range all {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				time.UnixMilli(c.Timestamp).Format(time.RFC3339),
				c.Open, c.High, c.Low, c.Close, c.Volume)
		}
		return w.Flush()
	},
}

func resolveSymbols(args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	markets, err := client.GetMarkets()
	if err != nil {
		return nil, fmt.Errorf("fetch markets: %w", err)
	}
	symbols := make([]string, len(markets))
	for i, m := range markets {
		symbols[i] = m.Symbol
	}
	return symbols, nil
}
