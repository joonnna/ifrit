package rpc

import (
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/joonnna/ifrit/protobuf"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
)

var (
	errReachable = errors.New("Remote entity not reachable")
)

type Client struct {
	allConnections  map[string]*conn
	connectionMutex sync.RWMutex

	dialOptions []grpc.DialOption
}

type conn struct {
	gossip.GossipClient
	cc *grpc.ClientConn
}

func NewClient() *Client {
	return &Client{
		allConnections: make(map[string]*conn),
	}
}

func (c *Client) Init(config *tls.Config) {
	creds := credentials.NewTLS(config)

	c.dialOptions = append(c.dialOptions, grpc.WithTransportCredentials(creds))
	c.dialOptions = append(c.dialOptions, grpc.WithBackoffMaxDelay(time.Minute*1))
	c.dialOptions = append(c.dialOptions, grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
}

func (c *Client) Gossip(addr string, args *gossip.State) (*gossip.StateResponse, error) {
	conn, err := c.connection(addr)
	if err != nil {
		return nil, err
	}

	r, err := conn.Spread(context.Background(), args)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (c *Client) SendMsg(addr string, args *gossip.Msg) (*gossip.MsgResponse, error) {
	conn, err := c.connection(addr)
	if err != nil {
		return nil, err
	}

	r, err := conn.Messenger(context.Background(), args)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (c *Client) CloseConn(addr string) {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()

	if conn, ok := c.allConnections[addr]; ok {
		conn.cc.Close()
		delete(c.allConnections, addr)
	}
}

func (c *Client) dial(addr string) (*conn, error) {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()

	if conn, ok := c.allConnections[addr]; ok {
		return conn, nil
	}

	cc, err := grpc.Dial(addr, c.dialOptions...)
	if err != nil {
		return nil, err
	} else {
		connection := &conn{
			GossipClient: gossip.NewGossipClient(cc),
			cc:           cc,
		}
		c.allConnections[addr] = connection

		return connection, nil
	}
}

func (c *Client) connection(addr string) (*conn, error) {
	if conn := c.getConnection(addr); conn != nil {
		return conn, nil
	} else {
		return c.dial(addr)
	}
}

func (c *Client) getConnection(addr string) *conn {
	c.connectionMutex.RLock()
	defer c.connectionMutex.RUnlock()

	return c.allConnections[addr]
}
