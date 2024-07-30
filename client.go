package tempts

import "go.temporal.io/sdk/client"

// Client is a wrapper for the temporal SDK client.
type Client struct {
	Client client.Client
}

// Close terminates the connection to the temporal server.
func (c *Client) Close() {
	c.Client.Close()
}

// NewLazyClient is equivalent to Dial, but doesn't conect to the server until necessary.
func NewLazyClient(opts client.Options) (*Client, error) {
	c, err := client.NewLazyClient(opts)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}

// Dial connects to the temporal server and returns a client.
func Dial(opts client.Options) (*Client, error) {
	c, err := client.Dial(opts)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}

// NewFromSDK allows the caller to pass in an existing temporal SDK client and manually specify which name that client was connected to.
func NewFromSDK(c client.Client) (*Client, error) {
	return &Client{Client: c}, nil
}
