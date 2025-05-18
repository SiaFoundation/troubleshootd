package api

import (
	"context"

	"go.sia.tech/jape"
	"go.sia.tech/troubleshootd/troubleshoot"
)

// Client is a client for the troubleshoot API.
type Client struct {
	c jape.Client
}

// TestConnection tests the host's connection to the API server.
func (c *Client) TestConnection(ctx context.Context, host troubleshoot.Host) (result troubleshoot.Result, err error) {
	err = c.c.POST(ctx, "/troubleshoot", host, &result)
	return
}

// NewClient creates a new client for the troubleshoot API.
func NewClient(addr string) *Client {
	return &Client{
		c: jape.Client{
			BaseURL: addr,
		},
	}
}
