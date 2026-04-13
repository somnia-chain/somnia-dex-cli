package api

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
