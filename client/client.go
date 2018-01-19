package client

import (
	"errors"

	"github.com/joonnna/ifrit/lib/node"
	"github.com/joonnna/ifrit/lib/rpc"
)

type Client struct {
	node *node.Node
}

var (
	errInvalidId = errors.New("Supplied ID is of length 0")
)

type Messenger interface {
	//Cmp(first, second []byte) bool
	Receive(gossip []byte) error
	Data(ids [][]byte) [][]byte
	Send() []byte
	//Ids() [][]byte
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

/*
func (c *Client) AddGossip(id []byte, data io.Reader) error {
	if len(id) <= 0 {
		return errInvalidId
	}

	return c.node.AppendGossipData(id, data)
}
*/
