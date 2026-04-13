package api

import (
	"fmt"
	"net/url"
)

// VaultBalance holds a currency balance in the vault.
type VaultBalance struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

type VaultBalances struct {
	Balances []VaultBalance `json:"balances"`
}

func (v VaultBalances) JSON() any { return v }

func (v VaultBalances) Table() [][]string {
	rows := [][]string{{"Currency", "Balance"}}
	for _, b := range v.Balances {
		rows = append(rows, []string{b.Currency, b.Amount})
	}
	return rows
}

// VaultActionRequest is the request body for vault approve, deposit, and withdraw operations.
type VaultActionRequest struct {
	WalletAddress string `json:"walletAddress"`
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
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
