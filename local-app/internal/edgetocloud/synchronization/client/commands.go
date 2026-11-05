package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
)

func (c *Client) commandRequest(ctx context.Context, method, suffix string, body any, out any) error {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.endpoint, "/")+suffix, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("invalid command endpoint")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cloud command channel unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("cloud command HTTP %d", res.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 2*1024*1024)).Decode(out); err != nil {
		return fmt.Errorf("invalid cloud command response")
	}
	return nil
}
func (c *Client) Commands(ctx context.Context) ([]exchange.Command, error) {
	var response struct {
		Commands []exchange.Command `json:"commands"`
	}
	err := c.commandRequest(ctx, "GET", "", nil, &response)
	return response.Commands, err
}
func (c *Client) Report(ctx context.Context, results []exchange.CommandResult) ([]string, error) {
	var response struct {
		Acknowledged []string `json:"acknowledged"`
	}
	err := c.commandRequest(ctx, "POST", "/results", struct {
		Results []exchange.CommandResult `json:"results"`
	}{results}, &response)
	return response.Acknowledged, err
}
