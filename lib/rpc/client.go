package rpc

import (
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/joonnna/ifrit/lib/protobuf"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	errReachable = errors.New("Remote entity not reachable")
)

type Client struct {
	allConnections  map[string]gossip.GossipClient
	connectionMutex sync.RWMutex

	dialOptions []grpc.DialOption
}

func NewClient() *Client {
	return &Client{
		allConnections: make(map[string]gossip.GossipClient),
	}
}

func (c *Client) Init(config *tls.Config) {
	//comp := grpc.NewGZIPCompressor()
	//decomp := grpc.NewGZIPDecompressor()
	creds := credentials.NewTLS(config)

	c.dialOptions = append(c.dialOptions, grpc.WithTransportCredentials(creds))
	//c.dialOptions = append(c.dialOptions, grpc.WithCompressor(comp))
	//c.dialOptions = append(c.dialOptions, grpc.WithDecompressor(decomp))
	c.dialOptions = append(c.dialOptions, grpc.WithBackoffMaxDelay(time.Minute*5))
}

func (c *Client) Dos(addr string, args *gossip.State) (*gossip.StateResponse, error) {
	client, err := c.dial(addr)
	if err != nil {
		return nil, err
	}

	r, err := client.Spread(context.Background(), args)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (c *Client) Gossip(addr string, args *gossip.State) (*gossip.StateResponse, error) {
	client, err := c.getClient(addr)
	if err != nil {
		return nil, err
	}

	r, err := client.Spread(context.Background(), args)
	if err != nil {
		c.removeConnection(addr)
		return nil, err
	}

	return r, nil
}

func (c *Client) SendMsg(addr string, args *gossip.Msg) (*gossip.MsgResponse, error) {
	client, err := c.getClient(addr)
	if err != nil {
		return nil, err
	}

	r, err := client.Messenger(context.Background(), args)
	if err != nil {
		c.removeConnection(addr)
		return nil, err
	}

	return r, nil
}

func (c *Client) dial(addr string) (gossip.GossipClient, error) {
	var client gossip.GossipClient

	conn, err := grpc.Dial(addr, c.dialOptions...)
	if err != nil {
		return nil, errReachable
	} else {
		client = gossip.NewGossipClient(conn)
		c.addConnection(addr, client)
	}

	return client, nil
}

func (c *Client) getClient(addr string) (gossip.GossipClient, error) {
	var client gossip.GossipClient
	var err error

	if !c.existConnection(addr) {
		client, err = c.dial(addr)
		if err != nil {
			return nil, err
		}
	} else {
		client = c.getConnection(addr)
	}

	return client, nil
}

func (c *Client) existConnection(addr string) bool {
	c.connectionMutex.RLock()
	defer c.connectionMutex.RUnlock()

	_, ok := c.allConnections[addr]

	return ok
}

func (c *Client) getConnection(addr string) gossip.GossipClient {
	c.connectionMutex.RLock()
	defer c.connectionMutex.RUnlock()

	return c.allConnections[addr]
}

func (c *Client) addConnection(addr string, conn gossip.GossipClient) {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()

	if _, ok := c.allConnections[addr]; ok {
		return
	}

	c.allConnections[addr] = conn

}

func (c *Client) removeConnection(addr string) {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()

	delete(c.allConnections, addr)
}
