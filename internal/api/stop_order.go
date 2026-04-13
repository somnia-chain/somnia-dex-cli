package api

import (
	"fmt"
	"net/url"
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
		{"ID", "Created", "Status", "Type", "Side", "Amount", "Trigger", "Operator", "Spot Order"},
	}
	for _, o := range s.StopOrders {
		v = append(v, []string{
			o.ID, time.UnixMilli(o.CreatedAt).String(), o.Status, o.Type, o.Side,
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

// CancelStopOrder returns an unsigned transaction to cancel a pending stop order.
func (c *Client) CancelStopOrder(symbol, id string) (*Transaction, error) {
	var resp Transaction
	path := fmt.Sprintf("/v0/markets/%s/stop-orders/%s", url.PathEscape(symbol), url.PathEscape(id))
	return &resp, c.do("DELETE", path, nil, nil, &resp)
}
