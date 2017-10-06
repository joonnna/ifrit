package client

import (
	"runtime"

	"github.com/joonnna/capstone/node"
)

type Client struct {
	node *node.Node
}

func StartClient(entryAddr string) *Client {
	runtime.GOMAXPROCS(runtime.NumCPU())

	/*
		comm, err := rpc.NewComm(entryAddr)
		if err != nil {
			panic(err)
		}

		logger := logger.CreateLogger(n.NodeComm.HostInfo(), "nodelog")
		comm.SetLogger(logger)
	*/
	n, err := node.NewNode(entryAddr)
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
