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

// marketsCmd returns the "markets" command, which lists all trading pairs.
func (a *app) marketsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "markets",
		Short: "List all trading pairs",
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "List markets",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			markets, err := a.client.GetMarkets()
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
}

// currenciesCmd returns the "currencies" command, which lists all supported tokens.
func (a *app) currenciesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "currencies",
		Short: "List all supported currencies",
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "List currencies",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			currencies, err := a.client.GetCurrencies()
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
}

// orderbookCmd returns the "orderbook" command, which shows bids and asks for one or all markets.
func (a *app) orderbookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orderbook [symbol]",
		Short: "Show order book (all markets if no symbol given)",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Show order book",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			symbols, err := a.resolveSymbols(args)
			if err != nil {
				return err
			}
			depth, _ := cmd.Flags().GetInt("depth")
			books, err := a.client.GetOrderBooks(symbols, depth)
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
	cmd.Flags().Int("depth", 0, "number of price levels to show")
	return cmd
}

// tickerCmd returns the "ticker" command, which shows 24h market statistics.
func (a *app) tickerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ticker [symbol]",
		Short: "Show 24h market statistics (all markets if no symbol given)",
		Args:  cobra.MaximumNArgs(1),
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
}

// tradesCmd returns the "trades" command, which shows recent trades.
func (a *app) tradesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trades [symbol]",
		Short: "Show recent trades (all markets if no symbol given)",
		Args:  cobra.MaximumNArgs(1),
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
	cmd.Flags().Int("limit", 20, "max trades to return")
	return cmd
}

// candlesCmd returns the "candles" command, which shows OHLCV candle data.
func (a *app) candlesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "candles [symbol]",
		Short: "Show OHLCV candle data (all markets if no symbol given)",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationReadOnly: "true",
			ophis.AnnotationTitle:    "Get OHLCV candles",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			symbols, err := a.resolveSymbols(args)
			if err != nil {
				return err
			}
			interval, _ := cmd.Flags().GetString("interval")
			limit, _ := cmd.Flags().GetInt("limit")
			var all []api.Candle
			for _, sym := range symbols {
				candles, err := a.client.GetCandles(sym, interval, limit)
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
	cmd.Flags().String("interval", "1h", "candle interval (1m, 5m, 15m, 1h, 4h, 1d)")
	cmd.Flags().Int("limit", 20, "max candles to return")
	return cmd
}
