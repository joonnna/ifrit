package client

import (
	"github.com/joonnna/capstone/node"
	"github.com/joonnna/capstone/communication"
	"github.com/joonnna/capstone/logger"
	"runtime"
)


type Client struct {
	node *node.Node
}

func StartClient(entryAddr string) *Client {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var numRings uint8
	numRings = 3

	logger := logger.CreateLogger()

	comm := communication.NewComm(logger)

	client := &Client{
		node: node.NewNode(entryAddr, comm, logger, comm.HostInfo(), numRings),
	}

	go client.node.Start()

	return client
}

func (c *Client) ShutDownClient() {
	c.node.ShutDownNode()
}
