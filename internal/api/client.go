package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// APIError represents a structured error from the dreamDEX API.
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

// Client is a dreamDEX API client.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	Log     *slog.Logger
}

// NewClient creates a new API client for the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    http.DefaultClient,
	}
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
	OrderID  string         `json:"orderId,omitempty"`
	Approval *OrderApproval `json:"approval,omitempty"`
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

	if c.Log != nil {
		attrs := []any{"method", method, "url", u}
		if bodyBytes != nil {
			attrs = append(attrs, "body", string(bodyBytes))
		}
		c.Log.Debug("http >>", attrs...)
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if c.Log != nil {
		c.Log.Debug("http <<", "status", resp.StatusCode, "body", string(respBody))
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		if apiErr.Name == "unauthorized" && c.Token == "" {
			apiErr.Description = "not logged in; run: DREAMDEX_PRIVATE_KEY=0x... dreamdex login"
		}
		return &apiErr
	}

	if result != nil {
		return json.Unmarshal(respBody, result)
	}
	return nil
}
