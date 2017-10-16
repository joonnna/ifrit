package udp

import (
	"github.com/joonnna/firechain/lib/protobuf"
)

type Client struct {
}

func (c *Client) Ping(addr string, args *gossip.Ping) (*gossip.Pong, error) {
	return nil, nil
}
