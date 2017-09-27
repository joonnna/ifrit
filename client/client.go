package client

import (
	"runtime"

	"github.com/joonnna/capstone/logger"
	"github.com/joonnna/capstone/node"
	"github.com/joonnna/capstone/node/rpc"
)

type Client struct {
	node *node.Node
}

func StartClient(entryAddr string) *Client {
	runtime.GOMAXPROCS(runtime.NumCPU())

	comm, err := rpc.NewComm(entryAddr)
	if err != nil {
		panic(err)
	}

	logger := logger.CreateLogger(comm.HostInfo(), "clientlog")
	comm.SetLogger(logger)

	n, err := node.NewNode(comm, logger)
	if err != nil {
		panic(err)
	}

	client := &Client{
		node: n,
	}

	go client.node.Start(node.NormalProtocol)

	return client
}

func (c *Client) ShutDownClient() {
	c.node.ShutDownNode()
}
