package api

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Order represents an order on the exchange.
type Order struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	CreatedAt      int64  `json:"createdAt"`
	Symbol         string `json:"symbol"`
	Side           string `json:"side"`
	Price          string `json:"price"`
	ExecutionPrice string `json:"executionPrice,omitempty"`
	Amount         string `json:"amount"`
	Filled         string `json:"filled"`
	Remaining      string `json:"remaining"`
	WalletAddress  string `json:"walletAddress,omitempty"`
	TxHash         string `json:"txHash,omitempty"`
}

type Orders struct {
	Orders []Order `json:"orders"`
}

func (o Orders) JSON() any {
	return o
}

func (o Orders) Table() [][]string {
	v := [][]string{
		{"Symbol", "ID", "Status", "Created", "Side", "Price", "Exec", "Amount", "Filled", "Remaining"},
	}

	for _, order := range o.Orders {
		createdAt := time.UnixMilli(order.CreatedAt)

		v = append(v, []string{
			order.Symbol, order.ID, order.Status, createdAt.String(), order.Side, order.Price, order.ExecutionPrice, order.Amount, order.Filled, order.Remaining,
		})
	}

	return v
}

// PrepareOrderRequest is the request body for preparing a new order.
type PrepareOrderRequest struct {
	Type                 string `json:"type"`
	Side                 string `json:"side"`
	Amount               string `json:"amount"`
	Price                string `json:"price,omitempty"`
	OrderType            string `json:"orderType,omitempty"`
	FundingSource        string `json:"fundingSource,omitempty"`
	SelfMatchingOption   string `json:"selfMatchingOption,omitempty"`
	ExpiresAt            int64  `json:"expiresAt,omitempty"`
	Builder              string `json:"builder,omitempty"`
	BuilderFeeBpsTimes1k int64  `json:"builderFeeBpsTimes1k,omitempty"`
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

// GetAllOrders lists the authenticated wallet's orders across markets, optionally
// filtered by symbols and status. Returns the page of orders and the next cursor.
func (c *Client) GetAllOrders(symbols []string, status string, limit int, cursor string) ([]Order, string, error) {
	q := url.Values{}
	for _, s := range symbols {
		q.Add("symbols", s)
	}
	if status != "" {
		q.Set("status", status)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	var resp struct {
		Orders     []Order `json:"orders"`
		NextCursor string  `json:"nextCursor"`
	}
	return resp.Orders, resp.NextCursor, c.do("GET", "/v0/orders", q, nil, &resp)
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
