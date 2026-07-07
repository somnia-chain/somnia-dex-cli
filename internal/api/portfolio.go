package api

import (
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

// Portfolio holds trading performance metrics for a wallet. Only headline fields are
// modeled for tabular display; the full response is preserved for JSON output.
type Portfolio struct {
	Wallet    string `json:"wallet"`
	Timeframe string `json:"timeframe"`
	AsOf      int64  `json:"asOf"`
	Pnl       struct {
		TotalUsd string `json:"totalUsd"`
	} `json:"pnl"`
	Mwrr struct {
		Return       *float64 `json:"return"`
		GainUsd      string   `json:"gainUsd"`
		DepositedUsd string   `json:"depositedUsd"`
	} `json:"mwrr"`
	Volume struct {
		PeriodUsd   string `json:"periodUsd"`
		LifetimeUsd string `json:"lifetimeUsd"`
		SessionUsd  string `json:"sessionUsd,omitempty"`
	} `json:"volume"`
	FeesSaved struct {
		CexRateBps  int    `json:"cexRateBps"`
		PeriodUsd   string `json:"periodUsd"`
		LifetimeUsd string `json:"lifetimeUsd"`
	} `json:"feesSaved"`

	raw json.RawMessage
}

func (p Portfolio) JSON() any { return p.raw }

func (p Portfolio) Table() [][]string {
	ret := "n/a"
	if p.Mwrr.Return != nil {
		ret = strconv.FormatFloat(*p.Mwrr.Return*100, 'f', 2, 64) + "%"
	}
	rows := [][]string{{"Metric", "Value"}}
	rows = append(rows,
		[]string{"Wallet", p.Wallet},
		[]string{"Timeframe", p.Timeframe},
		[]string{"As Of", time.UnixMilli(p.AsOf).Format(time.RFC3339)},
		[]string{"PnL (USD)", p.Pnl.TotalUsd},
		[]string{"Return", ret},
		[]string{"Volume period (USD)", p.Volume.PeriodUsd},
		[]string{"Volume lifetime (USD)", p.Volume.LifetimeUsd},
		[]string{"Fees saved period (USD)", p.FeesSaved.PeriodUsd},
		[]string{"Fees saved lifetime (USD)", p.FeesSaved.LifetimeUsd},
	)
	if p.Volume.SessionUsd != "" {
		rows = append(rows, []string{"Volume session (USD)", p.Volume.SessionUsd})
	}
	return rows
}

// GetPortfolio returns portfolio metrics for the authenticated wallet. timeframe is one of
// "24h", "7d", "30d", "all"; sessionSince (unix ms) adds a session volume figure when non-zero.
func (c *Client) GetPortfolio(timeframe string, sessionSince int64) (*Portfolio, error) {
	q := url.Values{}
	if timeframe != "" {
		q.Set("timeframe", timeframe)
	}
	if sessionSince > 0 {
		q.Set("sessionSince", strconv.FormatInt(sessionSince, 10))
	}
	var raw json.RawMessage
	if err := c.do("GET", "/v0/portfolio", q, nil, &raw); err != nil {
		return nil, err
	}
	var p Portfolio
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	p.raw = raw
	return &p, nil
}
