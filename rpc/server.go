package rpc

import (
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/joonnna/ifrit/netutil"
	"github.com/joonnna/ifrit/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

var (
	errInvalidInterface = errors.New("Provided interface is invalid")
	errNoPort           = errors.New("Listener has no port")
)

type Server struct {
	rpcServer *grpc.Server

	listener net.Listener
	port     int
}

func NewServer() (*Server, error) {
	l, err := netutil.ListenOnPort(8100)
	if err != nil {
		return nil, err
	}

	split := strings.Split(l.Addr().String(), ":")
	if len(split) < 2 {
		return nil, errNoPort
	}

	port, err := strconv.Atoi(split[1])
	if err != nil {
		return nil, err
	}

	return &Server{
		listener: l,
		port:     port,
	}, nil
}

func (s *Server) Init(config *tls.Config, n interface{}, maxConcurrent uint32) error {
	var serverOpts []grpc.ServerOption

	registerInterface, ok := n.(gossip.GossipServer)
	if !ok {
		return errInvalidInterface
	}

	keepAlive := keepalive.ServerParameters{
		MaxConnectionIdle: time.Minute * 20,
		Time:              time.Minute * 20,
	}

	creds := credentials.NewTLS(config)

	//comp := grpc.NewGZIPCompressor()
	//decomp := grpc.NewGZIPDecompressor()

	serverOpts = append(serverOpts, grpc.Creds(creds))
	//serverOpts = append(serverOpts, grpc.RPCCompressor(comp))
	//serverOpts = append(serverOpts, grpc.RPCDecompressor(decomp))
	serverOpts = append(serverOpts, grpc.KeepaliveParams(keepAlive))

	serverOpts = append(serverOpts, grpc.MaxConcurrentStreams(maxConcurrent))

	s.rpcServer = grpc.NewServer(serverOpts...)

	gossip.RegisterGossipServer(s.rpcServer, registerInterface)

	return nil
}

func (s *Server) Start() error {
	return s.rpcServer.Serve(s.listener)
}

func (s *Server) ShutDown() {
	s.rpcServer.Stop()
}

func (s *Server) Addr() string {
	//return s.port
	return s.listener.Addr().String()
}
