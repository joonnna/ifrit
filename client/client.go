package client

import (
	"fmt"
	"runtime"

	"github.com/joonnna/firechain/lib/node"
	"github.com/joonnna/firechain/lib/rpc"
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
		fmt.Println(err)
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
