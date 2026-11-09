package client

import (
	"context"
	"net/http"

	cloudmodel "kicau.local/edge/internal/cloud/model"
)

func (c *Client) Commands(ctx context.Context) ([]cloudmodel.Command, error) {
	var response struct {
		Commands []cloudmodel.Command `json:"commands"`
	}
	err := c.request(ctx, http.MethodGet, "", nil, &response)
	return response.Commands, err
}
func (c *Client) Report(ctx context.Context, results []cloudmodel.CommandResult) ([]string, error) {
	var response struct {
		Acknowledged []string `json:"acknowledged"`
	}
	err := c.request(ctx, http.MethodPost, "/results", struct {
		Results []cloudmodel.CommandResult `json:"results"`
	}{results}, &response)
	return response.Acknowledged, err
}
