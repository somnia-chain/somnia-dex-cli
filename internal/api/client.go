package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// APIError represents a structured error from the DreamDEX API.
type APIError struct {
	Status      int    `json:"status"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (e *APIError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("%s: %s", e.Name, e.Description)
	}
	return e.Name
}

// Client is a DreamDEX API client.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	Debug   bool
}

// NewClient creates a new API client for the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    http.DefaultClient,
	}
}

func (c *Client) do(method, path string, query url.Values, body, result any) error {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var bodyReader io.Reader
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, ">> %s %s\n", method, u)
		if bodyBytes != nil {
			var buf bytes.Buffer
			json.Indent(&buf, bodyBytes, "", "  ")
			fmt.Fprintf(os.Stderr, ">> %s\n", buf.String())
		}
	}

	req, err := http.NewRequest(method, u, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		if apiErr.Name == "unauthorized" && c.Token == "" {
			apiErr.Description = "not logged in; run: DREAMDEX_PRIVATE_KEY=0x... dreamdex login"
		}
		return &apiErr
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// Market represents a trading pair on DreamDEX.
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
}

// Currency represents a token supported by the exchange.
type Currency struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Decimals int    `json:"decimals"`
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
	Nonce     string       `json:"nonce"`
	Bids      []PriceLevel `json:"bids"`
	Asks      []PriceLevel `json:"asks"`
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

// Trade represents a completed trade on a market.
type Trade struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Symbol    string `json:"symbol"`
	Side      string `json:"side"`
	Price     string `json:"price"`
	Amount    string `json:"amount"`
	Cost      string `json:"cost"`
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

// Order represents an order on the exchange.
type Order struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"createdAt"`
	Symbol        string `json:"symbol"`
	Type          string `json:"type"`
	Side          string `json:"side"`
	Price         string `json:"price"`
	Amount        string `json:"amount"`
	Filled        string `json:"filled"`
	Remaining     string `json:"remaining"`
	WalletAddress string `json:"walletAddress,omitempty"`
	TxHash        string `json:"txHash,omitempty"`
}

// OrderApproval indicates a token approval is required before an order can execute.
type OrderApproval struct {
	Token  string `json:"token"`
	Amount string `json:"amount"`
}

// Transaction is an unsigned EVM transaction returned by the API for client-side signing.
type Transaction struct {
	To       string         `json:"to"`
	Data     string         `json:"data"`
	Value    string         `json:"value"`
	ChainID  string         `json:"chainId"`
	GasLimit string         `json:"gasLimit,omitempty"`
	Nonce    string         `json:"nonce,omitempty"`
	Approval *OrderApproval `json:"approval,omitempty"`
}

// VaultBalance holds a currency balance in the vault.
type VaultBalance struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

// PrepareOrderRequest is the request body for preparing a new order.
type PrepareOrderRequest struct {
	Type               string `json:"type"`
	Side               string `json:"side"`
	Amount             string `json:"amount"`
	WalletAddress      string `json:"walletAddress"`
	Price              string `json:"price,omitempty"`
	OrderType          string `json:"orderType,omitempty"`
	FundingSource      string `json:"fundingSource,omitempty"`
	SelfMatchingOption string `json:"selfMatchingOption,omitempty"`
	ExpiresAt          int64  `json:"expiresAt,omitempty"`
}

// VaultActionRequest is the request body for vault approve, deposit, and withdraw operations.
type VaultActionRequest struct {
	WalletAddress string `json:"walletAddress"`
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
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

// GetNonce fetches a one-time nonce for SIWE authentication.
func (c *Client) GetNonce() (string, error) {
	var resp struct {
		Nonce string `json:"nonce"`
	}
	return resp.Nonce, c.do("GET", "/v0/auth/nonce", nil, nil, &resp)
}

// Login exchanges a signed SIWE message for a JWT token.
func (c *Client) Login(message, signature string) (token string, expiresAt int64, err error) {
	body := struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}{message, signature}
	var resp struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expiresAt"`
	}
	err = c.do("POST", "/v0/auth/login", nil, &body, &resp)
	return resp.Token, resp.ExpiresAt, err
}

// PrepareOrder returns an unsigned transaction for placing an order on the given market.
func (c *Client) PrepareOrder(symbol string, req *PrepareOrderRequest) (*Transaction, error) {
	var resp Transaction
	path := fmt.Sprintf("/v0/markets/%s/orders", url.PathEscape(symbol))
	return &resp, c.do("POST", path, nil, req, &resp)
}

// GetOrders lists orders for a market, optionally filtered by status.
func (c *Client) GetOrders(symbol, status string) ([]Order, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	var resp struct {
		Orders []Order `json:"orders"`
	}
	path := fmt.Sprintf("/v0/markets/%s/orders", url.PathEscape(symbol))
	return resp.Orders, c.do("GET", path, q, nil, &resp)
}

// GetOrder returns details for a single order.
func (c *Client) GetOrder(symbol, id string) (*Order, error) {
	var resp Order
	path := fmt.Sprintf("/v0/markets/%s/orders/%s", url.PathEscape(symbol), url.PathEscape(id))
	return &resp, c.do("GET", path, nil, nil, &resp)
}

// CancelOrder returns an unsigned transaction to cancel an open order on-chain.
func (c *Client) CancelOrder(symbol, id string) (*Transaction, error) {
	var resp Transaction
	path := fmt.Sprintf("/v0/markets/%s/orders/%s", url.PathEscape(symbol), url.PathEscape(id))
	return &resp, c.do("DELETE", path, nil, nil, &resp)
}

// ReduceOrder returns an unsigned transaction to reduce an order's remaining quantity.
func (c *Client) ReduceOrder(symbol, id, newQty string) (*Transaction, error) {
	var resp Transaction
	body := struct {
		NewQuantityRemaining string `json:"newQuantityRemaining"`
	}{newQty}
	path := fmt.Sprintf("/v0/markets/%s/orders/%s/reduce", url.PathEscape(symbol), url.PathEscape(id))
	return &resp, c.do("PATCH", path, nil, &body, &resp)
}

// GetVaultBalance returns vault balances for a wallet on the given market.
func (c *Client) GetVaultBalance(symbol, wallet string) ([]VaultBalance, error) {
	q := url.Values{"walletAddress": {wallet}}
	var resp struct {
		Balances []VaultBalance `json:"balances"`
	}
	path := fmt.Sprintf("/v0/markets/%s/vault/balance", url.PathEscape(symbol))
	return resp.Balances, c.do("GET", path, q, nil, &resp)
}

// PrepareApproval returns an unsigned transaction to approve token spending for vault deposits.
func (c *Client) PrepareApproval(symbol string, req *VaultActionRequest) (*Transaction, error) {
	var resp Transaction
	path := fmt.Sprintf("/v0/markets/%s/vault/approve", url.PathEscape(symbol))
	return &resp, c.do("POST", path, nil, req, &resp)
}

// PrepareDeposit returns an unsigned transaction to deposit tokens into the vault.
func (c *Client) PrepareDeposit(symbol string, req *VaultActionRequest) (*Transaction, error) {
	var resp Transaction
	path := fmt.Sprintf("/v0/markets/%s/vault/deposit", url.PathEscape(symbol))
	return &resp, c.do("POST", path, nil, req, &resp)
}

// PrepareWithdraw returns an unsigned transaction to withdraw tokens from the vault.
func (c *Client) PrepareWithdraw(symbol string, req *VaultActionRequest) (*Transaction, error) {
	var resp Transaction
	path := fmt.Sprintf("/v0/markets/%s/vault/withdraw", url.PathEscape(symbol))
	return &resp, c.do("POST", path, nil, req, &resp)
}
