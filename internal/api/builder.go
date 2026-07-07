package api

import (
	"fmt"
	"net/url"
	"strconv"
)

// BuilderApproval is a wallet's builder-fee approval and the protocol cap, in BPS_TIMES_1K units.
type BuilderApproval struct {
	Builder        string `json:"builder"`
	Approved       int64  `json:"approved"`
	Effective      int64  `json:"effective"`
	ProtocolMaxFee int64  `json:"protocolMaxFee"`
}

func (b BuilderApproval) JSON() any { return b }

func (b BuilderApproval) Table() [][]string {
	return [][]string{
		{"Builder", "Approved", "Effective", "Protocol Max"},
		{b.Builder,
			strconv.FormatInt(b.Approved, 10),
			strconv.FormatInt(b.Effective, 10),
			strconv.FormatInt(b.ProtocolMaxFee, 10)},
	}
}

// BuilderApprovalRequest is the body for preparing a builder approval.
// MaxFeeBpsTimes1k of 0 revokes an existing approval.
type BuilderApprovalRequest struct {
	Builder          string `json:"builder"`
	MaxFeeBpsTimes1k int64  `json:"maxFeeBpsTimes1k"`
}

// GetBuilderApproval returns a wallet's builder approval for a specific builder on a market.
func (c *Client) GetBuilderApproval(symbol, wallet, builder string) (*BuilderApproval, error) {
	q := url.Values{"walletAddress": {wallet}, "builder": {builder}}
	var resp BuilderApproval
	path := fmt.Sprintf("/v0/markets/%s/builder/approval", url.PathEscape(symbol))
	return &resp, c.do("GET", path, q, nil, &resp)
}

// PrepareBuilderApproval returns an unsigned transaction granting (or revoking) a builder's fee permission.
func (c *Client) PrepareBuilderApproval(symbol string, req *BuilderApprovalRequest) (*Transaction, error) {
	var resp Transaction
	path := fmt.Sprintf("/v0/markets/%s/builder/approve", url.PathEscape(symbol))
	return &resp, c.do("POST", path, nil, req, &resp)
}

// GetBuilderMaxFee returns the protocol-wide cap on builder approvals, in BPS_TIMES_1K units.
func (c *Client) GetBuilderMaxFee(symbol string) (int64, error) {
	var resp struct {
		MaxFeeBpsTimes1k int64 `json:"maxFeeBpsTimes1k"`
	}
	path := fmt.Sprintf("/v0/markets/%s/builder/max-fee", url.PathEscape(symbol))
	return resp.MaxFeeBpsTimes1k, c.do("GET", path, nil, nil, &resp)
}
