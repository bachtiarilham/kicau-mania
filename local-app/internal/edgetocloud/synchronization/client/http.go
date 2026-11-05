// Package client transports durable changes to the cloud over authenticated HTTPS.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
	"net/http"
	"time"
)

type Client struct {
	endpoint, token string
	http            *http.Client
}

func New(endpoint, token string) *Client {
	return &Client{endpoint: endpoint, token: token, http: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) Send(ctx context.Context, changes []exchange.Change) ([]string, error) {
	body, err := json.Marshal(struct {
		Changes []exchange.Change `json:"changes"`
	}{changes})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("invalid sync endpoint")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloud unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		return nil, fmt.Errorf("cloud returned %d", res.StatusCode)
	}
	var response struct {
		Acknowledged []string `json:"acknowledged"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1024*1024)).Decode(&response); err != nil {
		return nil, fmt.Errorf("invalid acknowledgment response")
	}
	return response.Acknowledged, nil
}
