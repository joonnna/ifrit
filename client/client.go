package client

import (
	"runtime"

	"github.com/joonnna/firechain/node"
	"github.com/joonnna/firechain/rpc"
)

type Client struct {
	node *node.Node
}

func NewClient(entryAddr string) *Client {
	runtime.GOMAXPROCS(runtime.NumCPU())

	c := rpc.NewClient()
	s := rpc.NewServer()

	n, err := node.NewNode(entryAddr, c, s)
	if err != nil {
		panic(err)
	}

	client := &Client{
		node: n,
	}

	return client
}

func (c *Client) ShutDown() {
	c.node.ShutDownNode()
}

func (c *Client) Start() {
	c.node.Start(node.NormalProtocol)
}
