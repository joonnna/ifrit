package client

import (
	"github.com/joonnna/ifrit/lib/node"
	"github.com/joonnna/ifrit/lib/rpc"
)

type Client struct {
	node *node.Node
}

type Gossip interface {
	Cmp(other Gossip) bool
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

func (c *Client) Members() []string {
	return c.node.LiveMembers()
}

func (c *Client) AddGossip() error {
	return nil
}
