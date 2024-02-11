package tstemporal

import "go.temporal.io/sdk/client"

type Client struct {
	namespace string
	Client    client.Client
}

func (c *Client) Close() {
	c.Client.Close()
}

func NewLazyClient(opts client.Options) (*Client, error) {
	namespace := client.DefaultNamespace
	if opts.Namespace != "" {
		namespace = opts.Namespace
	}
	c, err := client.NewLazyClient(opts)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c, namespace: namespace}, nil
}

func Dial(opts client.Options) (*Client, error) {
	namespace := client.DefaultNamespace
	if opts.Namespace != "" {
		namespace = opts.Namespace
	}
	c, err := client.Dial(opts)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c, namespace: namespace}, nil
}
