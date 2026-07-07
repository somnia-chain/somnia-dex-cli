package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/njayp/ophis"
	"github.com/spf13/cobra"
	"golang.org/x/net/websocket"
)

// watchCmd returns the "watch" parent command for streaming live market data.
func (a *app) watchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream live market data via WebSocket",
		Long: `Stream live market data over WebSocket. All watch commands maintain a persistent
connection and print updates as they arrive. Use --timeout to auto-terminate
after a duration (e.g. 30s, 5m).`,
	}
	cmd.PersistentFlags().Duration("timeout", 0, "auto-terminate after duration (e.g. 30s, 5m)")
	cmd.AddCommand(
		a.watchOrderbookCmd(),
		a.watchTradesCmd(),
		a.watchCandlesCmd(),
		a.watchOrderCmd(),
		a.watchStopOrderCmd(),
	)
	return cmd
}

// watchOrderbookCmd streams live order book updates.
func (a *app) watchOrderbookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "orderbook [symbol]",
		Short: "Stream order book updates",
		Long:  "Stream real-time order book updates showing bid/ask changes as they happen.",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Watch orderbook",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			symbols, err := a.resolveSymbols(args)
			if err != nil {
				return err
			}
			return a.stream(cmd, "orderbook", map[string]any{
				"symbols": symbols,
			}, displayOrderbook)
		},
	}
}

// watchTradesCmd streams live trade executions.
func (a *app) watchTradesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trades [symbol]",
		Short: "Stream live trades",
		Long:  "Stream trade executions in real time as they occur on the exchange.",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Watch trades",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			symbols, err := a.resolveSymbols(args)
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetInt("limit")
			return a.stream(cmd, "trades", map[string]any{
				"symbols": symbols,
				"limit":   limit,
			}, displayTrades)
		},
	}
	cmd.Flags().Int("limit", 20, "number of initial trades")
	return cmd
}

// watchCandlesCmd streams live OHLCV candle updates.
func (a *app) watchCandlesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "candles [symbol]",
		Short: "Stream candle updates",
		Long:  "Stream live OHLCV candlestick updates at a specified interval.",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Watch candles",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			symbols, err := a.resolveSymbols(args)
			if err != nil {
				return err
			}
			interval, _ := cmd.Flags().GetString("interval")
			// OHLCV channel takes a single symbol; subscribe to each.
			subs := make([]subscription, len(symbols))
			for i, s := range symbols {
				subs[i] = subscription{
					channel: "ohlcv",
					params: map[string]any{
						"symbol":    s,
						"timeframe": interval,
					},
				}
			}
			return a.streamMulti(cmd, subs, displayCandles)
		},
	}
	cmd.Flags().String("interval", "1m", "candle interval: 1m, 5m, 15m, 1h, 4h, 1d")
	return cmd
}

// watchOrderCmd streams updates for a specific order.
func (a *app) watchOrderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "order <id>",
		Short: "Watch an order for status changes",
		Long:  "Watch a specific order for status changes (fills, cancellations, etc.) in real time.",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Watch order",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.stream(cmd, "order", map[string]any{
				"orderId": args[0],
			}, displayOrder)
		},
	}
}

// watchStopOrderCmd polls a stop order for status changes.
func (a *app) watchStopOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stoporder <symbol> <id>",
		Short: "Watch a stop order for status changes",
		Long: `Poll a stop order for status changes. Prints the current state on first fetch,
then prints updates when the status changes. Exits automatically when the stop
order reaches a terminal state (triggered, canceled, failed).`,
		Args: cobra.ExactArgs(2),
		Annotations: map[string]string{
			ophis.AnnotationTitle: "Watch stop order",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireAuth(cmd); err != nil {
				return err
			}
			interval, _ := cmd.Flags().GetDuration("interval")
			return a.pollStopOrder(cmd, args[0], args[1], interval)
		},
	}
	cmd.Flags().Duration("interval", 2*time.Second, "poll interval")
	return cmd
}

// pollStopOrder polls the stop order API until a terminal status or context cancellation.
func (a *app) pollStopOrder(cmd *cobra.Command, symbol, id string, interval time.Duration) error {
	timeout, _ := cmd.Flags().GetDuration("timeout")
	jsonMode := isJSON(cmd)

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	tick := time.NewTicker(interval)
	defer tick.Stop()

	var lastStatus string
	for {
		order, err := a.findStopOrder(symbol, id)
		if err != nil {
			return fmt.Errorf("get stop order: %w", err)
		}

		if order.Status != lastStatus {
			if jsonMode {
				printJSON(order)
			} else {
				ts := time.Now().Format("15:04:05")
				fmt.Printf("[%s] %s  stop-order #%s  %s %s  trigger:%s %s  status:%s\n",
					ts, order.Symbol, order.ID, order.Side, order.Type,
					order.TriggerOperator, order.TriggerPrice, order.Status)
			}
			lastStatus = order.Status
		}

		switch lastStatus {
		case "triggered", "canceled", "failed":
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

type subscription struct {
	channel     string
	params      any
	bearerToken string
}

// stream connects to the WebSocket, subscribes to a channel, and prints messages.
func (a *app) stream(cmd *cobra.Command, channel string, params any, display func(json.RawMessage)) error {
	return a.streamMulti(cmd, []subscription{{channel: channel, params: params}}, display)
}

// parseLogLevel converts a level name to slog.Level.
func parseLogLevel(name string) slog.Level {
	switch strings.ToLower(name) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

// wsSend sends a JSON message over the WebSocket, logging the raw body at debug level.
func wsSend(ws *websocket.Conn, log *slog.Logger, v any) error {
	raw, _ := json.Marshal(v)
	log.Debug("ws >>", "body", string(raw))
	return websocket.JSON.Send(ws, v)
}

// wsRecv receives a JSON message from the WebSocket, logging the raw body at debug level.
func wsRecv(ws *websocket.Conn, log *slog.Logger, v any) error {
	err := websocket.JSON.Receive(ws, v)
	if err == nil {
		if raw, ok := v.(*json.RawMessage); ok {
			log.Debug("ws <<", "body", string(*raw))
		}
	}
	return err
}

// streamMulti connects to the WebSocket, subscribes to multiple channels, and prints messages.
func (a *app) streamMulti(cmd *cobra.Command, subs []subscription, display func(json.RawMessage)) error {
	apiURL, _ := cmd.Flags().GetString("api-url")
	wsURL := strings.Replace(strings.Replace(apiURL, "https://", "wss://", 1), "http://", "ws://", 1) + "/v0/ws/public"
	timeout, _ := cmd.Flags().GetDuration("timeout")

	log := a.log

	log.Debug("connecting", "url", wsURL)

	cfg, err := websocket.NewConfig(wsURL, apiURL)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if a.client.Token != "" {
		cfg.Header.Set("Authorization", "Bearer "+a.client.Token)
	}
	ws, err := websocket.DialConfig(cfg)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer ws.Close()

	log.Info("connected")

	for _, sub := range subs {
		msg := map[string]any{
			"operation": "subscribe",
			"channel":   sub.channel,
			"params":    sub.params,
		}
		if sub.bearerToken != "" {
			msg["bearerToken"] = sub.bearerToken
		}
		if err := wsSend(ws, log, msg); err != nil {
			return fmt.Errorf("subscribe: %w", err)
		}
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
		log.Info("will auto-terminate", "after", timeout)
	}

	// Close connection on interrupt/timeout; ping every 30s.
	go func() {
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Info("disconnecting")
				ws.Close()
				return
			case <-tick.C:
				wsSend(ws, log, map[string]string{"operation": "ping"}) //nolint:errcheck
			}
		}
	}()

	jsonMode := isJSON(cmd)

	// Read loop.
	for {
		var msg json.RawMessage
		if err := wsRecv(ws, log, &msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}

		var envelope struct {
			Type      string `json:"type"`
			Operation string `json:"operation"`
		}
		json.Unmarshal(msg, &envelope)
		switch {
		case envelope.Type == "subscribed" || envelope.Type == "unsubscribed" || envelope.Operation == "pong":
			if jsonMode {
				os.Stdout.Write(msg)
				fmt.Println()
			}
			continue
		case envelope.Type == "error":
			log.Error("server error", "body", string(msg))
			continue
		}

		if jsonMode {
			os.Stdout.Write(msg)
			fmt.Println()
		} else {
			display(msg)
		}
	}
}

// Display functions for human-readable output.

func displayOrderbook(msg json.RawMessage) {
	var m struct {
		Type   string `json:"type"`
		Symbol string `json:"symbol"`
		Bids   []struct {
			Price    string `json:"price"`
			Quantity string `json:"quantity"`
		} `json:"bids"`
		Asks []struct {
			Price    string `json:"price"`
			Quantity string `json:"quantity"`
		} `json:"asks"`
	}
	if json.Unmarshal(msg, &m) != nil {
		return
	}
	ts := time.Now().Format("15:04:05")
	fmt.Printf("[%s] %s %s\n", ts, m.Symbol, m.Type)
	for i := len(m.Asks) - 1; i >= 0; i-- {
		fmt.Printf("  ask  %-10s %s\n", m.Asks[i].Price, m.Asks[i].Quantity)
	}
	if len(m.Asks) > 0 || len(m.Bids) > 0 {
		fmt.Println("       ----      ----")
	}
	for _, b := range m.Bids {
		fmt.Printf("  bid  %-10s %s\n", b.Price, b.Quantity)
	}
	fmt.Println()
}

func displayTrades(msg json.RawMessage) {
	var m struct {
		Type   string `json:"type"`
		Symbol string `json:"symbol"`
		Trade  *struct {
			Price    string `json:"price"`
			Quantity string `json:"quantity"`
			Side     string `json:"side"`
		} `json:"trade"`
		Trades []struct {
			Price    string `json:"price"`
			Quantity string `json:"quantity"`
			Side     string `json:"side"`
		} `json:"trades"`
	}
	if json.Unmarshal(msg, &m) != nil {
		return
	}
	ts := time.Now().Format("15:04:05")
	if m.Trade != nil {
		fmt.Printf("[%s] %s  %-4s  %s @ %s\n", ts, m.Symbol, m.Trade.Side, m.Trade.Quantity, m.Trade.Price)
	}
	for _, t := range m.Trades {
		fmt.Printf("[%s] %s  %-4s  %s @ %s\n", ts, m.Symbol, t.Side, t.Quantity, t.Price)
	}
}

func displayCandles(msg json.RawMessage) {
	var m struct {
		Type      string `json:"type"`
		Symbol    string `json:"symbol"`
		Timeframe string `json:"timeframe"`
		Candle    *struct {
			Open   string `json:"open"`
			High   string `json:"high"`
			Low    string `json:"low"`
			Close  string `json:"close"`
			Volume string `json:"volume"`
		} `json:"candle"`
		Candles []struct {
			Open      string `json:"open"`
			High      string `json:"high"`
			Low       string `json:"low"`
			Close     string `json:"close"`
			Volume    string `json:"volume"`
			Timestamp int64  `json:"timestamp"`
		} `json:"candles"`
	}
	if json.Unmarshal(msg, &m) != nil {
		return
	}
	ts := time.Now().Format("15:04:05")
	if m.Candle != nil {
		fmt.Printf("[%s] %s %s  O:%s H:%s L:%s C:%s V:%s\n",
			ts, m.Symbol, m.Timeframe, m.Candle.Open, m.Candle.High, m.Candle.Low, m.Candle.Close, m.Candle.Volume)
	}
	for _, c := range m.Candles {
		ct := time.UnixMilli(c.Timestamp).Format("15:04:05")
		fmt.Printf("[%s] %s %s  O:%s H:%s L:%s C:%s V:%s\n",
			ct, m.Symbol, m.Timeframe, c.Open, c.High, c.Low, c.Close, c.Volume)
	}
}

func displayOrder(msg json.RawMessage) {
	var m struct {
		Type  string `json:"type"`
		Order *struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Side   string `json:"side"`
			Price  string `json:"price"`
			Filled string `json:"filled"`
			Market string `json:"market"`
		} `json:"order"`
	}
	if json.Unmarshal(msg, &m) != nil {
		return
	}
	if m.Order == nil {
		return
	}
	ts := time.Now().Format("15:04:05")
	fmt.Printf("[%s] %s  %s %s  price:%s  filled:%s  status:%s\n",
		ts, m.Type, m.Order.Market, m.Order.Side, m.Order.Price, m.Order.Filled, m.Order.Status)
}
