package rpc

import (
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net"

	"github.com/joonnna/firechain/lib/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	startPort = 7235
)

var (
	errInvalidInterface = errors.New("Provided interface is invalid")
)

type Server struct {
	rpcServer *grpc.Server

	listener net.Listener
	addr     string
	//options []grpc.ServerOption
}

func NewServer() *Server {
	hostName := getLocalIP()
	l, addr := getOpenPort(hostName)

	return &Server{
		listener: l,
		addr:     addr,
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
	return s.addr
}

func getLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return ""
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			return ip.String()
		}
	}
	return ""
}

func getOpenPort(hostName string) (net.Listener, string) {
	var l net.Listener
	var err error
	port := startPort

	for {
		addr := fmt.Sprintf("%s:%d", hostName, (1000 + (rand.Int() % port)))
		l, err = net.Listen("tcp", addr)
		if err == nil {
			return l, addr
		}
		port += 1
	}

	return nil, ""
}
