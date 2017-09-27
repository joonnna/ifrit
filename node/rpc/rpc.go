package rpc

import (
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/joonnna/capstone/logger"
	"github.com/joonnna/capstone/protobuf"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

const (
	startPort = 8345
)

var (
	errReachable = errors.New("Remote entity not reachable")
)

type Comm struct {
	allConnections  map[string]gossip.GossipClient
	connectionMutex sync.RWMutex

	rpcServer *grpc.Server

	localAddr string
	listener  net.Listener

	privKey    *rsa.PrivateKey
	pubKey     crypto.PublicKey
	localCert  *x509.Certificate
	entryCerts []*x509.Certificate
	caCertPool *x509.CertPool

	log *logger.Log

	dialCreds grpc.DialOption
}

func NewComm(caAddr string) (*Comm, error) {
	var err error
	/*
		host, _ := os.Hostname()
		hostName := strings.Split(host, ".")[0]
	*/
	hostName := getLocalIP()

	privKey, err := genKeys()
	if err != nil {
		return nil, err
	}

	l, addr := getOpenPort(hostName)

	certs, err := sendCertRequest(caAddr, privKey, addr)
	if err != nil {
		return nil, err
	}

	config, err := genServerConfig(certs, privKey)
	if err != nil {
		return nil, err
	}

	creds := credentials.NewTLS(config)

	options := grpc.Creds(creds)

	keepAlive := keepalive.ServerParameters{
		Time:    time.Second * 60,
		Timeout: time.Second * 20,
	}

	opt := grpc.KeepaliveParams(keepAlive)

	pool := x509.NewCertPool()
	pool.AddCert(certs.caCert)

	comm := &Comm{
		allConnections: make(map[string]gossip.GossipClient),
		localAddr:      addr,
		listener:       l,
		rpcServer:      grpc.NewServer(options, opt),
		privKey:        privKey,
		pubKey:         privKey.Public(),
		localCert:      certs.ownCert,
		entryCerts:     certs.knownCerts,
		caCertPool:     pool,
		//dialCreds:      grpc.WithTransportCredentials(creds),
	}

	return comm, nil
}

func (c Comm) NumRings() uint8 {
	return c.localCert.Extensions[0].Value[0]
}

func (c Comm) ID() []byte {
	return c.localCert.SubjectKeyId
}

func (c Comm) EntryCertificates() []*x509.Certificate {
	return c.entryCerts
}

func (c *Comm) SetLogger(log *logger.Log) {
	c.log = log
}

func (c *Comm) Register(g gossip.GossipServer) {
	gossip.RegisterGossipServer(c.rpcServer, g)
}

func (c *Comm) Start() error {
	return c.rpcServer.Serve(c.listener)
}

func (c Comm) HostInfo() string {
	return c.localAddr
}

func (c *Comm) ShutDown() {
	c.rpcServer.GracefulStop()
}

func (c *Comm) dial(addr string) (gossip.GossipClient, error) {
	var client gossip.GossipClient

	test := tls.Certificate{
		Certificate: [][]byte{c.localCert.Raw},
		PrivateKey:  c.privKey,
		//Leaf:        c.localCert,
	}

	keepAlive := keepalive.ClientParameters{
		Time:    time.Second * 60,
		Timeout: time.Second * 20,
	}

	creds := credentials.NewTLS(&tls.Config{
		Certificates:       []tls.Certificate{test},
		ServerName:         strings.Split(addr, ":")[0], // NOTE: this is required!
		RootCAs:            c.caCertPool,
		InsecureSkipVerify: true,
	})

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds), grpc.WithKeepaliveParams(keepAlive))
	if err != nil {
		c.log.Err.Println(err)
		return nil, errReachable
	} else {
		client = gossip.NewGossipClient(conn)
		c.addConnection(addr, client)
	}

	return client, nil
}

func (c *Comm) Gossip(addr string, args *gossip.GossipMsg) (*gossip.Empty, error) {
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

func (c *Comm) Monitor(addr string, args *gossip.Ping) (*gossip.Pong, error) {
	client, err := c.getClient(addr)
	if err != nil {
		return nil, err
	}

	r, err := client.Monitor(context.Background(), args)
	if err != nil {
		c.removeConnection(addr)
		return nil, err
	}

	return r, nil
}

func (c *Comm) getClient(addr string) (gossip.GossipClient, error) {
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
		addr := fmt.Sprintf("%s:%d", hostName, port)
		l, err = net.Listen("tcp", addr)
		if err == nil {
			return l, addr
		}
		port += 1
	}

	return nil, ""
}
