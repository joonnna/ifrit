package comm

import (
	"crypto/tls"
	"errors"
	"net"
	"time"

	log "github.com/inconshreveable/log15"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"
)

var (
	errInvalidInterface = errors.New("Provided interface is invalid")
	errNoPort           = errors.New("Listener has no port")
	errNoAddr           = errors.New("Can't lookup own hostname")
)

const (
	defaultPort = 8300
)

type gRPCServer struct {
	rpcServer *grpc.Server

	listener   net.Listener
	listenAddr string
}

func newServer(config *tls.Config, l net.Listener) (*gRPCServer, error) {
	var serverOpts []grpc.ServerOption

	if config == nil {
		return nil, errNilConfig
	}

	keepAlive := keepalive.ServerParameters{
		MaxConnectionIdle: time.Minute * 5,
		Time:              time.Minute * 5,
	}

	creds := credentials.NewTLS(config)

	serverOpts = append(serverOpts, grpc.Creds(creds))
	serverOpts = append(serverOpts, grpc.KeepaliveParams(keepAlive))

	return &gRPCServer{
		listener:   l,
		listenAddr: l.Addr().String(),
		rpcServer:  grpc.NewServer(serverOpts...),
	}, nil
}

func (s *gRPCServer) start() error {
	err := s.rpcServer.Serve(s.listener)
	if err != nil {
		log.Error(err.Error())
	}

	return err
}

func (s *gRPCServer) stop() {
	s.rpcServer.Stop()
}

func (s *gRPCServer) addr() string {
	return s.listenAddr
}
