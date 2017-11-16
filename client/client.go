package client

import (
	"github.com/joonnna/go-fireflies/lib/node"
	"github.com/joonnna/go-fireflies/lib/rpc"
)

type Client struct {
	node *node.Node
}

func NewClient(entryAddr string) (*Client, error) {
	c := rpc.NewClient()

	s, err := rpc.NewServer()
	if err != nil {
		return nil, err
	}

	n, err := node.NewNode(entryAddr, c, s)
	if err != nil {
		return nil, err
	}

	client := &Client{
		node: n,
	}

	return client, nil
}

func (c *Client) ShutDown() {
	c.node.ShutDownNode()
}

func (c *Client) Start() {
	c.node.Start()
}
