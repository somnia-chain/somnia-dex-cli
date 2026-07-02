package api

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// StopOrder represents a conditional stop order on the SpotStopOrderRegistry.
type StopOrder struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	CreatedAt       int64  `json:"createdAt"`
	Symbol          string `json:"symbol"`
	Type            string `json:"type"`
	Side            string `json:"side"`
	Amount          string `json:"amount,omitempty"`
	TriggerPrice    string `json:"triggerPrice"`
	TriggerOperator string `json:"triggerOperator"`
	SpotOrderID     string `json:"spotOrderId,omitempty"`
	WalletAddress   string `json:"walletAddress,omitempty"`
}

type StopOrders struct {
	StopOrders []StopOrder `json:"stopOrders"`
}

func (s StopOrders) JSON() any { return s }

func (s StopOrders) Table() [][]string {
	v := [][]string{
		{"Symbol", "ID", "Created", "Status", "Type", "Side", "Amount", "Trigger", "Operator", "Spot Order"},
	}
	for _, o := range s.StopOrders {
		v = append(v, []string{
			o.Symbol, o.ID, time.UnixMilli(o.CreatedAt).String(), o.Status, o.Type, o.Side,
			o.Amount, o.TriggerPrice, o.TriggerOperator, o.SpotOrderID,
		})
	}
	return v
}

// PrepareStopOrderRequest is the request body for preparing a new stop order.
type PrepareStopOrderRequest struct {
	Type            string `json:"type"`
	Side            string `json:"side"`
	Amount          string `json:"amount"`
	TriggerPrice    string `json:"triggerPrice"`
	TriggerOperator string `json:"triggerOperator"`
	WalletAddress   string `json:"walletAddress"`
	Price           string `json:"price,omitempty"`
}

// PrepareStopOrder returns an unsigned transaction for creating a stop order.
func (c *Client) PrepareStopOrder(symbol string, req *PrepareStopOrderRequest) (*Transaction, error) {
	var resp Transaction
	path := fmt.Sprintf("/v0/markets/%s/stop-orders", url.PathEscape(symbol))
	return &resp, c.do("POST", path, nil, req, &resp)
}

// GetStopOrders lists stop orders for a market, optionally filtered by status.
func (c *Client) GetStopOrders(symbol, status string) ([]StopOrder, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	var resp struct {
		StopOrders []StopOrder `json:"stopOrders"`
	}
	path := fmt.Sprintf("/v0/markets/%s/stop-orders", url.PathEscape(symbol))
	return resp.StopOrders, c.do("GET", path, q, nil, &resp)
}

// GetStopOrder fetches a single stop order by ID.
func (c *Client) GetStopOrder(symbol, id string) (*StopOrder, error) {
	var resp StopOrder
	path := fmt.Sprintf("/v0/markets/%s/stop-orders/%s", url.PathEscape(symbol), url.PathEscape(id))
	return &resp, c.do("GET", path, nil, nil, &resp)
}

// CancelStopOrder returns an unsigned transaction to cancel a pending stop order.
func (c *Client) CancelStopOrder(symbol, id string) (*Transaction, error) {
	var resp Transaction
	path := fmt.Sprintf("/v0/markets/%s/stop-orders/%s", url.PathEscape(symbol), url.PathEscape(id))
	return &resp, c.do("DELETE", path, nil, nil, &resp)
}

// GetAllStopOrders lists the authenticated wallet's stop orders across markets, optionally
// filtered by symbols and status. Returns the page of stop orders and the next cursor.
func (c *Client) GetAllStopOrders(symbols []string, status string, limit int, cursor string) ([]StopOrder, string, error) {
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
		StopOrders []StopOrder `json:"stopOrders"`
		NextCursor string      `json:"nextCursor"`
	}
	return resp.StopOrders, resp.NextCursor, c.do("GET", "/v0/stop-orders", q, nil, &resp)
}

// GetStopOrderAuthorization reports whether the wallet has granted the market's stop-order
// registry operator permission to place orders on its behalf.
func (c *Client) GetStopOrderAuthorization(symbol string) (bool, error) {
	var resp struct {
		Authorized bool `json:"authorized"`
	}
	path := fmt.Sprintf("/v0/markets/%s/stop-orders/authorization", url.PathEscape(symbol))
	return resp.Authorized, c.do("GET", path, nil, nil, &resp)
}

// PrepareStopOrderApproval returns an unsigned transaction granting the stop-order registry
// operator permission to place orders on the wallet's behalf.
func (c *Client) PrepareStopOrderApproval(symbol string) (*Transaction, error) {
	var resp Transaction
	path := fmt.Sprintf("/v0/markets/%s/stop-orders/approve", url.PathEscape(symbol))
	return &resp, c.do("POST", path, nil, nil, &resp)
}
