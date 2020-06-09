package comm

import (
	"crypto/tls"
	"errors"
	"sync"
	"time"
	"io"

	pb "github.com/joonnna/ifrit/protobuf"
	log "github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
)

var (
	errReachable = errors.New("Remote entity not reachable")
	errNilConfig = errors.New("Provided tls config was nil")
)

type gRPCClient struct {
	allConnections  map[string]*conn
	connectionMutex sync.RWMutex

	dialOptions []grpc.DialOption
}

type conn struct {
	pb.GossipClient
	cc *grpc.ClientConn
}

func newClient(config *tls.Config) (*gRPCClient, error) {
	var dialOptions []grpc.DialOption

	if config == nil {
		return nil, errNilConfig
	}

	creds := credentials.NewTLS(config)

	dialOptions = append(dialOptions, grpc.WithTransportCredentials(creds))
	dialOptions = append(dialOptions, grpc.WithBackoffMaxDelay(time.Minute*1))

	if compress := viper.GetBool("use_compression"); compress {
		dialOptions = append(dialOptions,
			grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	}

	return &gRPCClient{
		allConnections: make(map[string]*conn),
		dialOptions:    dialOptions,
	}, nil
}

func (c *gRPCClient) Gossip(addr string, args *pb.State) (*pb.StateResponse, error) {
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

func (c *gRPCClient) Send(addr string, args *pb.Msg) (*pb.MsgResponse, error) {
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

func (c *gRPCClient) StreamMessenger(addr string, input, reply chan []byte) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	conn, err := c.connection(addr)
	if err != nil {
		return err
	}
	
	s, err := conn.Stream(context.Background()) 
	if err != nil {
		return err
	}

	ctx := s.Context()

	// Sending messages from input stream to the server. 
	// Runs until the producer closes the channel
	wg.Add(1)
	go func() {
		defer wg.Done()
		for content := range input {
			msg := &pb.Msg{
				Content: content,
			}
			if err := s.Send(msg); err != nil {
				log.Error(err.Error())
			}
		}
		if err := s.CloseSend(); err != nil {
			log.Error(err.Error())
		}
	}()

	// Receiving messages from the server. Runs until the channel no longer blocks
	// TODO: implement a nicer abstraction to safeguard the use of the channels
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			msg, err := s.Recv()
			if err == io.EOF {
				return 
			} 
			if err != nil {
				log.Error(err.Error())
			}
			reply <- msg.GetContent()
		}
	}()
	
	<-ctx.Done()
	close(reply)
	if err := ctx.Err(); err != nil {
		return err
	}

	return nil
}

func (c *gRPCClient) CloseConn(addr string) {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()

	if conn, ok := c.allConnections[addr]; ok {
		conn.cc.Close()
		delete(c.allConnections, addr)
	}
}

func (c *gRPCClient) dial(addr string) (*conn, error) {
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
			GossipClient: pb.NewGossipClient(cc),
			cc:           cc,
		}
		c.allConnections[addr] = connection

		return connection, nil
	}
}

func (c *gRPCClient) connection(addr string) (*conn, error) {
	if conn := c.getConnection(addr); conn != nil {
		return conn, nil
	} else {
		return c.dial(addr)
	}
}

func (c *gRPCClient) getConnection(addr string) *conn {
	c.connectionMutex.RLock()
	defer c.connectionMutex.RUnlock()

	return c.allConnections[addr]
}
