package client

import (
	"github.com/joonnna/capstone/node"
	"github.com/joonnna/capstone/communication"
	"github.com/joonnna/capstone/logger"
)


type Client struct {
	node *node.Node
}

func StartClient(entryAddr string) *Client {
	logger := logger.CreateLogger()

	client := &Client{}
	comm := communication.NewComm(logger)

	client.node = node.NewNode(entryAddr, comm, logger)

	go client.node.Start()

	return client
}
