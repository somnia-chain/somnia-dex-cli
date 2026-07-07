package api

import (
	"fmt"
	"net/url"
	"strconv"
)

// CurrencyBalance is the wallet and vault balance for a single currency.
type CurrencyBalance struct {
	Currency string `json:"currency"`
	Wallet   string `json:"wallet"`
	Vault    string `json:"vault"`
}

// MarketBalance holds balances for both sides of a market.
type MarketBalance struct {
	Symbol string          `json:"symbol"`
	Base   CurrencyBalance `json:"base"`
	Quote  CurrencyBalance `json:"quote"`
}

// WalletBalance holds a wallet's balances across markets at a given block.
type WalletBalance struct {
	Wallet      string          `json:"wallet"`
	BlockNumber int64           `json:"blockNumber"`
	Markets     []MarketBalance `json:"markets"`
}

func (w WalletBalance) JSON() any { return w }

func (w WalletBalance) Table() [][]string {
	rows := [][]string{{"Symbol", "Currency", "Wallet", "Vault"}}
	for _, m := range w.Markets {
		for _, b := range []CurrencyBalance{m.Base, m.Quote} {
			rows = append(rows, []string{m.Symbol, b.Currency, b.Wallet, b.Vault})
		}
	}
	return rows
}

// WalletMarketVolume is traded volume for a single market within a wallet volume report.
type WalletMarketVolume struct {
	Symbol      string `json:"symbol"`
	BaseVolume  string `json:"baseVolume"`
	QuoteVolume string `json:"quoteVolume"`
}

// WalletVolume holds a wallet's traded volume per market over a time window.
type WalletVolume struct {
	Wallet  string               `json:"wallet"`
	Since   int64                `json:"since"`
	Until   int64                `json:"until"`
	Volumes []WalletMarketVolume `json:"volumes"`
}

func (w WalletVolume) JSON() any { return w }

func (w WalletVolume) Table() [][]string {
	rows := [][]string{{"Symbol", "Base Volume", "Quote Volume"}}
	for _, v := range w.Volumes {
		rows = append(rows, []string{v.Symbol, v.BaseVolume, v.QuoteVolume})
	}
	return rows
}

// SmartWallets holds the smart wallets provisioned for an EOA.
type SmartWallets struct {
	Wallet       string `json:"wallet"`
	SmartWallets []struct {
		Address string `json:"address"`
	} `json:"smartWallets"`
}

func (s SmartWallets) JSON() any { return s }

func (s SmartWallets) Table() [][]string {
	rows := [][]string{{"Smart Wallet"}}
	for _, sw := range s.SmartWallets {
		rows = append(rows, []string{sw.Address})
	}
	return rows
}

// GetWalletBalance returns a wallet's balances across markets. block pins the read when non-zero.
func (c *Client) GetWalletBalance(wallet string, block int64) (*WalletBalance, error) {
	q := url.Values{}
	if block > 0 {
		q.Set("blockNumber", strconv.FormatInt(block, 10))
	}
	var resp WalletBalance
	path := fmt.Sprintf("/v0/wallets/%s/balance", url.PathEscape(wallet))
	return &resp, c.do("GET", path, q, nil, &resp)
}

// GetWalletVolume returns a wallet's traded volume per market over an optional [since, until) window.
func (c *Client) GetWalletVolume(wallet string, since, until int64) (*WalletVolume, error) {
	q := url.Values{}
	if since > 0 {
		q.Set("since", strconv.FormatInt(since, 10))
	}
	if until > 0 {
		q.Set("until", strconv.FormatInt(until, 10))
	}
	var resp WalletVolume
	path := fmt.Sprintf("/v0/wallets/%s/volume", url.PathEscape(wallet))
	return &resp, c.do("GET", path, q, nil, &resp)
}

// GetSmartWallets resolves the smart wallets provisioned for an EOA.
func (c *Client) GetSmartWallets(wallet string) (*SmartWallets, error) {
	var resp SmartWallets
	path := fmt.Sprintf("/v0/wallets/%s/smart-wallets", url.PathEscape(wallet))
	return &resp, c.do("GET", path, nil, nil, &resp)
}
