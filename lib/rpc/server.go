package rpc

import (
	"crypto/tls"
	"errors"
	"net"

	"github.com/joonnna/firechain/lib/netutils"
	"github.com/joonnna/firechain/lib/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	errInvalidInterface = errors.New("Provided interface is invalid")
)

type Server struct {
	rpcServer *grpc.Server

	listener net.Listener
}

func NewServer() *Server {
	hostName := netutils.GetLocalIP()
	l, err := netutils.GetListener(hostName)
	if err != nil {
		panic(err)
	}

	return &Server{
		listener: l,
	}
}

func (s *Server) Init(config *tls.Config, n interface{}) error {
	registerInterface, ok := n.(gossip.GossipServer)
	if !ok {
		return errInvalidInterface
	}

	creds := credentials.NewTLS(config)

	options := grpc.Creds(creds)
	s.rpcServer = grpc.NewServer(options)

	gossip.RegisterGossipServer(s.rpcServer, registerInterface)

	return nil
}

func (s *Server) Start() error {
	return s.rpcServer.Serve(s.listener)
}

func (s *Server) ShutDown() {
	s.rpcServer.GracefulStop()
}

func (s *Server) HostInfo() string {
	return s.listener.Addr().String()
}
