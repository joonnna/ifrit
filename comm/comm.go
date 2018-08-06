package comm

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	pb "github.com/joonnna/ifrit/protobuf"
)

var (
	errNilCert = errors.New("Given certificate was nil")
	errNilPriv = errors.New("Given private key was nil")
)

type Comm struct {
	s *gRPCServer
	*gRPCClient
}

func NewComm(cert, caCert *x509.Certificate, priv *ecdsa.PrivateKey, l net.Listener) (*Comm, error) {
	if cert == nil {
		return nil, errNilCert
	}

	if priv == nil {
		return nil, errNilPriv
	}

	serverConf := serverConfig(cert, caCert, priv)

	server := newServer(serverConf, l)

	clientConf := clientConfig(cert, caCert, priv)

	return &Comm{
		s:          server,
		gRPCClient: newClient(clientConf),
	}, nil
}

func (c *Comm) Register(p pb.GossipServer) {
	pb.RegisterGossipServer(c.s.rpcServer, p)
}

func (c *Comm) Start() {
	c.s.start()
}

func (c *Comm) Stop() {
	c.s.stop()
}

func (c *Comm) Addr() string {
	return c.s.addr()
}

func serverConfig(c, caCert *x509.Certificate, key *ecdsa.PrivateKey) *tls.Config {
	tlsCert := tls.Certificate{
		Certificate: [][]byte{c.Raw},
		PrivateKey:  key,
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	if caCert == nil {
		conf.ClientAuth = tls.RequestClientCert
	} else {
		pool := x509.NewCertPool()
		pool.AddCert(caCert)
		conf.ClientCAs = pool
		conf.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return conf
}

func clientConfig(c, caCert *x509.Certificate, key *ecdsa.PrivateKey) *tls.Config {
	tlsCert := tls.Certificate{
		Certificate: [][]byte{c.Raw},
		PrivateKey:  key,
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	if caCert != nil {
		pool := x509.NewCertPool()
		pool.AddCert(caCert)
		conf.RootCAs = pool
	} else {
		conf.InsecureSkipVerify = true
	}

	return conf
}
