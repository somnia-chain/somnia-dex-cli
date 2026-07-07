package api

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Market represents a trading pair on dreamDEX.
type Market struct {
	Symbol        string `json:"symbol"`
	Contract      string `json:"contract"`
	Base          string `json:"base"`
	Quote         string `json:"quote"`
	BaseDecimals  int    `json:"baseDecimals"`
	QuoteDecimals int    `json:"quoteDecimals"`
	TickSize      string `json:"tickSize"`
	LotSize       string `json:"lotSize"`
	MinQuantity   string `json:"minQuantity"`
	StopRegistry  string `json:"stopRegistry,omitempty"`
}

type Markets struct {
	Markets []Market `json:"markets"`
}

func (m Markets) JSON() any { return m }

func (m Markets) Table() [][]string {
	v := [][]string{{"Symbol", "Contract", "Base", "Quote", "Tick Size", "Lot Size", "Min Qty"}}
	for _, mk := range m.Markets {
		v = append(v, []string{mk.Symbol, mk.Contract, mk.Base, mk.Quote, mk.TickSize, mk.LotSize, mk.MinQuantity})
	}
	return v
}

// Currency represents a token supported by the exchange.
type Currency struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Decimals int    `json:"decimals"`
}

type Currencies struct {
	Currencies []Currency `json:"currencies"`
}

func (c Currencies) JSON() any { return c }

func (c Currencies) Table() [][]string {
	v := [][]string{{"Code", "Name", "Decimals", "Address"}}
	for _, cur := range c.Currencies {
		v = append(v, []string{cur.Code, cur.Name, strconv.Itoa(cur.Decimals), cur.ID})
	}
	return v
}

// PriceLevel is a single price/quantity entry in an order book.
type PriceLevel struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

// OrderBook holds the bids and asks for a market.
type OrderBook struct {
	Symbol    string       `json:"symbol"`
	Timestamp int64        `json:"timestamp"`
	Bids      []PriceLevel `json:"bids"`
	Asks      []PriceLevel `json:"asks"`
}

type OrderBooks struct {
	OrderBooks []OrderBook `json:"orderbooks"`
}

func (o OrderBooks) JSON() any {
	type bookWithMid struct {
		OrderBook
		Mid string `json:"mid,omitempty"`
	}
	out := make([]bookWithMid, len(o.OrderBooks))
	for i, b := range o.OrderBooks {
		out[i].OrderBook = b
		if len(b.Asks) > 0 && len(b.Bids) > 0 {
			ask, _ := strconv.ParseFloat(b.Asks[0].Price, 64)
			bid, _ := strconv.ParseFloat(b.Bids[0].Price, 64)
			out[i].Mid = strconv.FormatFloat((bid+ask)/2, 'f', -1, 64)
		}
	}
	return struct {
		OrderBooks []bookWithMid `json:"orderbooks"`
	}{out}
}

func (o OrderBooks) Table() [][]string {
	v := [][]string{{"Position", "Price", "Quantity"}}
	for _, book := range o.OrderBooks {
		for j := len(book.Asks) - 1; j >= 0; j-- {
			v = append(v, []string{"ask", book.Asks[j].Price, book.Asks[j].Quantity})
		}
		if len(book.Asks) > 0 && len(book.Bids) > 0 {
			ask, _ := strconv.ParseFloat(book.Asks[0].Price, 64)
			bid, _ := strconv.ParseFloat(book.Bids[0].Price, 64)
			v = append(v, []string{"mid", strconv.FormatFloat((bid+ask)/2, 'f', -1, 64), ""})
		}
		for _, b := range book.Bids {
			v = append(v, []string{"bid", b.Price, b.Quantity})
		}
	}
	return v
}

// Ticker holds 24-hour OHLCV statistics for a market.
type Ticker struct {
	Symbol    string `json:"symbol"`
	Timestamp int64  `json:"timestamp"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
	Volume    string `json:"volume"`
}

// Trade represents a completed trade on a market. Maker and Taker are only populated
// for privileged reads (bearer token with the trades:read_any scope).
type Trade struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Symbol    string `json:"symbol"`
	Side      string `json:"side"`
	Price     string `json:"price"`
	Amount    string `json:"amount"`
	Cost      string `json:"cost"`
	Maker     string `json:"maker,omitempty"`
	Taker     string `json:"taker,omitempty"`
}

type Trades struct {
	Trades []Trade `json:"trades"`
}

func (t Trades) JSON() any { return t }

func (t Trades) Table() [][]string {
	v := [][]string{{"Symbol", "Time", "Side", "Price", "Amount", "Cost"}}
	for _, tr := range t.Trades {
		v = append(v, []string{
			tr.Symbol, time.UnixMilli(tr.Timestamp).Format(time.RFC3339),
			tr.Side, tr.Price, tr.Amount, tr.Cost,
		})
	}
	return v
}

// Candle holds OHLCV data for a single time interval.
type Candle struct {
	Timestamp int64  `json:"timestamp"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
	Volume    string `json:"volume"`
}

type Candles struct {
	Candles []Candle `json:"candles"`
}

func (c Candles) JSON() any { return c }

func (c Candles) Table() [][]string {
	v := [][]string{{"Time", "Open", "High", "Low", "Close", "Volume"}}
	for _, cn := range c.Candles {
		v = append(v, []string{
			time.UnixMilli(cn.Timestamp).Format(time.RFC3339),
			cn.Open, cn.High, cn.Low, cn.Close, cn.Volume,
		})
	}
	return v
}

// GetMarkets returns all available trading pairs.
func (c *Client) GetMarkets() ([]Market, error) {
	var resp struct {
		Markets []Market `json:"markets"`
	}
	return resp.Markets, c.do("GET", "/v0/markets", nil, nil, &resp)
}

// GetCurrencies returns all supported currencies.
func (c *Client) GetCurrencies() ([]Currency, error) {
	var resp struct {
		Currencies []Currency `json:"currencies"`
	}
	return resp.Currencies, c.do("GET", "/v0/currencies", nil, nil, &resp)
}

// GetOrderBooks returns order books for the given symbols, optionally limited to depth levels.
func (c *Client) GetOrderBooks(symbols []string, depth int) ([]OrderBook, error) {
	q := url.Values{}
	for _, s := range symbols {
		q.Add("symbols", s)
	}
	if depth > 0 {
		q.Set("depth", fmt.Sprintf("%d", depth))
	}
	var resp struct {
		OrderBooks []OrderBook `json:"orderbooks"`
	}
	return resp.OrderBooks, c.do("GET", "/v0/orderbooks", q, nil, &resp)
}

// GetTicker returns 24-hour statistics for a market.
func (c *Client) GetTicker(symbol string) ([]Ticker, error) {
	var resp struct {
		Symbols []Ticker `json:"symbols"`
	}
	path := fmt.Sprintf("/v0/markets/%s/tickers", url.PathEscape(symbol))
	return resp.Symbols, c.do("GET", path, nil, nil, &resp)
}

// GetTickers returns 24-hour statistics for the given symbols, or all markets when symbols is empty.
func (c *Client) GetTickers(symbols []string) ([]Ticker, error) {
	q := url.Values{}
	for _, s := range symbols {
		q.Add("symbols", s)
	}
	var resp struct {
		Symbols []Ticker `json:"symbols"`
	}
	return resp.Symbols, c.do("GET", "/v0/tickers", q, nil, &resp)
}

// MarketVolume holds traded volume for a single market over a time window.
type MarketVolume struct {
	Symbol         string `json:"symbol"`
	Since          int64  `json:"since"`
	Until          int64  `json:"until"`
	BaseVolume     string `json:"baseVolume"`
	BaseVolumeRaw  string `json:"baseVolumeRaw"`
	QuoteVolume    string `json:"quoteVolume"`
	QuoteVolumeRaw string `json:"quoteVolumeRaw"`
}

func (v MarketVolume) JSON() any { return v }

func (v MarketVolume) Table() [][]string {
	return [][]string{
		{"Symbol", "Base Volume", "Quote Volume", "Since", "Until"},
		{v.Symbol, v.BaseVolume, v.QuoteVolume,
			time.UnixMilli(v.Since).Format(time.RFC3339),
			time.UnixMilli(v.Until).Format(time.RFC3339)},
	}
}

// GetMarketVolume returns traded volume for a market over an optional [since, until) window (unix ms).
func (c *Client) GetMarketVolume(symbol string, since, until int64) (*MarketVolume, error) {
	q := url.Values{}
	if since > 0 {
		q.Set("since", strconv.FormatInt(since, 10))
	}
	if until > 0 {
		q.Set("until", strconv.FormatInt(until, 10))
	}
	var resp MarketVolume
	path := fmt.Sprintf("/v0/markets/%s/volume", url.PathEscape(symbol))
	return &resp, c.do("GET", path, q, nil, &resp)
}

// GetTrades returns recent trades for a market, optionally filtered by timestamp and count.
func (c *Client) GetTrades(symbol string, since int64, limit int) ([]Trade, error) {
	q := url.Values{}
	if since > 0 {
		q.Set("since", fmt.Sprintf("%d", since))
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	var resp struct {
		Trades []Trade `json:"trades"`
	}
	path := fmt.Sprintf("/v0/markets/%s/trades", url.PathEscape(symbol))
	return resp.Trades, c.do("GET", path, q, nil, &resp)
}

// GetMyTrades returns the authenticated wallet's trades for a market. Returns the page and next cursor.
func (c *Client) GetMyTrades(symbol string, since int64, limit int, cursor string) ([]Trade, string, error) {
	q := tradeQuery(since, limit, cursor)
	var resp struct {
		Trades     []Trade `json:"trades"`
		NextCursor string  `json:"nextCursor"`
	}
	path := fmt.Sprintf("/v0/markets/%s/trades/mine", url.PathEscape(symbol))
	return resp.Trades, resp.NextCursor, c.do("GET", path, q, nil, &resp)
}

// GetAllMyTrades returns the authenticated wallet's trades across markets, optionally filtered by symbols.
func (c *Client) GetAllMyTrades(symbols []string, since int64, limit int, cursor string) ([]Trade, string, error) {
	q := tradeQuery(since, limit, cursor)
	for _, s := range symbols {
		q.Add("symbols", s)
	}
	var resp struct {
		Trades     []Trade `json:"trades"`
		NextCursor string  `json:"nextCursor"`
	}
	return resp.Trades, resp.NextCursor, c.do("GET", "/v0/trades", q, nil, &resp)
}

// GetTraderTrades returns trades for an arbitrary wallet on a market (privileged: requires the
// trades:read_any scope). Filter by side with as = "maker" or "taker".
func (c *Client) GetTraderTrades(symbol, address string, since, until int64, as string) ([]Trade, error) {
	q := url.Values{}
	if since > 0 {
		q.Set("since", strconv.FormatInt(since, 10))
	}
	if until > 0 {
		q.Set("until", strconv.FormatInt(until, 10))
	}
	if as != "" {
		q.Set("as", as)
	}
	var resp struct {
		Trades []Trade `json:"trades"`
	}
	path := fmt.Sprintf("/v0/markets/%s/trades/%s", url.PathEscape(symbol), url.PathEscape(address))
	return resp.Trades, c.do("GET", path, q, nil, &resp)
}

// tradeQuery builds the common since/limit/cursor query for trade listings.
func tradeQuery(since int64, limit int, cursor string) url.Values {
	q := url.Values{}
	if since > 0 {
		q.Set("since", strconv.FormatInt(since, 10))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	return q
}

// GetCandles returns OHLCV candle data for a market at the given interval.
func (c *Client) GetCandles(symbol, interval string, limit int) ([]Candle, error) {
	q := url.Values{"interval": {interval}}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	var resp struct {
		Candles []Candle `json:"candles"`
	}
	path := fmt.Sprintf("/v0/markets/%s/candles", url.PathEscape(symbol))
	return resp.Candles, c.do("GET", path, q, nil, &resp)
}
