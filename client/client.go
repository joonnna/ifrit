package client

import (
	"errors"
	"io"

	"github.com/joonnna/ifrit/lib/node"
	"github.com/joonnna/ifrit/lib/rpc"
)

type Client struct {
	node *node.Node
}

var (
	errInvalidId = errors.New("Supplied ID is of length 0")
)

/*
type Gossip interface {
	Cmp(other []byte) bool
	Id() []byte
	Data() []byte
}
*/

func NewClient(entryAddr string, cmpFunc func(this, other []byte) bool) (*Client, error) {
	c := rpc.NewClient()

	s, err := rpc.NewServer()
	if err != nil {
		return nil, err
	}

	n, err := node.NewNode(entryAddr, c, s, cmpFunc)
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

func (c *Client) AddGossip(id []byte, data io.Reader) error {
	if len(id) <= 0 {
		return errInvalidId
	}

	return c.node.AppendGossipData(id, data)
}
