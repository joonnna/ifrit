package communication


import (
	"fmt"
	"os"
	"strings"
	"net"
	"errors"
	"golang.org/x/net/context"
	"sync"
	"github.com/joonnna/capstone/logger"
	"github.com/joonnna/capstone/protobuf"
	"google.golang.org/grpc"
)

const (
	startPort = 8345
)

var (
	errReachable = errors.New("Remote entity not reachable")
)

type Comm struct {
	allConnections map[string]gossip.GossipClient
	connectionMutex sync.RWMutex

	rpcServer *grpc.Server

	localAddr string
	listener net.Listener

	log *logger.Log
}

func NewComm (log *logger.Log) *Comm {
	var l net.Listener
	var err error

	host, _ := os.Hostname()
	hostName := strings.Split(host, ".")[0]

	port := startPort

	for {
		l, err = net.Listen("tcp", fmt.Sprintf("%s:%d", hostName, port))
		if err != nil {
			log.Err.Println(err)
		} else {
			break
		}
		port += 1
	}

	comm := &Comm {
		allConnections: make(map[string]gossip.GossipClient),
		localAddr: fmt.Sprintf("%s:%d", hostName, port),
		listener: l,
		log: log,
		rpcServer: grpc.NewServer(),
	}

	return comm
}

func (c *Comm) Register(g gossip.GossipServer) {
	gossip.RegisterGossipServer(c.rpcServer, g)
}


func (c *Comm) Start() error {
	return c.rpcServer.Serve(c.listener)
}

func (c Comm) HostInfo () string {
	return c.localAddr
}

func (c *Comm) ShutDown () {
	c.rpcServer.GracefulStop()
}


func (c *Comm) dial (addr string) (gossip.GossipClient, error) {
	var client gossip.GossipClient

	opts := grpc.WithInsecure()

	conn, err := grpc.Dial(addr, opts)
	if err != nil {
		c.log.Err.Println(err)
		return nil, errReachable
	} else {
		client = gossip.NewGossipClient(conn)
		c.addConnection(addr, client)
	}

	return client, nil
}

func (c *Comm) Gossip (addr string, args *gossip.NodeInfo) (*gossip.Nodes, error) {
	client, err := c.getClient(addr)
	if err != nil {
		return nil, err
	}

	r, err := client.Spread(context.Background(), args)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (c *Comm) Monitor (addr string, args *gossip.Ping) (*gossip.Pong, error) {
	client, err := c.getClient(addr)
	if err != nil {
		return nil, err
	}

	r, err := client.Monitor(context.Background(), args)
	if err != nil {
		return nil, err
	}

	return r, nil
}


func (c *Comm) Accuse (addrList []string, args *gossip.Accusation) (*gossip.Empty, error) {
	var r *gossip.Empty

	for _, addr := range addrList {
		client, err := c.getClient(addr)
		if err != nil {
			continue
		}

		r, err = client.Accuse(context.Background(), args)
		if err != nil {
			continue
		}
	}
	return r, nil
}


func (c *Comm) getClient (addr string) (gossip.GossipClient, error) {
	var client gossip.GossipClient
	var err error

	if !c.existConnection(addr) {
		client, err = c.dial(addr)
		if err != nil {
			return client, err
		}
	} else {
		client = c.getConnection(addr)
	}

	return client, nil
}
