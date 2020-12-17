package cauth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/joonnna/ifrit/protobuf"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type stub struct {
}

func TestTLSconnection(t *testing.T) {
	priv, err := genKeys()
	if err != nil {
		fmt.Println(err)
		return
	}

	serialNumber, err := genSerialNumber()
	if err != nil {
		fmt.Println(err)
		return
	}

	caCert := &x509.Certificate{
		SerialNumber:          serialNumber,
		SubjectKeyId:          []byte{1, 2, 3, 4, 5},
		BasicConstraintsValid: true,
		IsCA:                  true,
		Subject: pkix.Name{
			Country:            []string{"SHIEEET"},
			Organization:       []string{"Yjwt"},
			OrganizationalUnit: []string{"YjwtU"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment,
	}

	gCert, err := x509.CreateCertificate(rand.Reader, caCert, caCert, priv.Public(), priv)
	if err != nil {
		fmt.Println(err)
		return
	}

	cert, err := x509.ParseCertificate(gCert)
	if err != nil {
		fmt.Println(err)
		return
	}

	addr := "127.0.0.1:8345"
	c1, p1 := genCert("127.0.0.1")
	c2, p2 := genCert("127.0.0.1")

	sc1 := signCert(c1, cert, priv)
	sc2 := signCert(c2, cert, priv)

	t1 := &stub{}
	t2 := &stub{}

	go t1.startServing(sc1, cert, p1, addr)

	t2.startPinging(sc2, cert, p2, addr)
}

func (s *stub) startServing(c, caCert *x509.Certificate, priv *rsa.PrivateKey, addr string) {
	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	certPool.AddCert(caCert)

	// Create the channel to listen on
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println(err)
		return
	}

	localCert := tls.Certificate{
		Certificate: [][]byte{c.Raw},
		PrivateKey:  priv,
	}

	// Create the TLS credentials
	creds := credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{localCert},
		ClientCAs:    certPool,
	})

	// Create the gRPC server with the credentials
	srv := grpc.NewServer(grpc.Creds(creds))

	// Register the handler object
	gossip.RegisterGossipServer(srv, s)

	// Serve and Listen
	if err := srv.Serve(lis); err != nil {
		fmt.Println(err)
		return
	}

	return

}

func (s *stub) Spread(ctx context.Context, msg *gossip.State) (*gossip.StateResponse, error) {
	return nil, nil
}

func (s *stub) Messenger(ctx context.Context, msg *gossip.Msg) (*gossip.MsgResponse, error) {
	return &gossip.MsgResponse{Content: []byte("this is a test message")}, nil
}

func (s *stub) startPinging(c, caCert *x509.Certificate, priv *rsa.PrivateKey, addr string) {
	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	certPool.AddCert(caCert)

	localCert := tls.Certificate{
		Certificate: [][]byte{c.Raw},
		PrivateKey:  priv,
	}

	creds := credentials.NewTLS(&tls.Config{
		ServerName:   strings.Split(addr, ":")[0], // NOTE: this is required!
		Certificates: []tls.Certificate{localCert},
		RootCAs:      certPool,
	})

	// Create a connection with the TLS credentials
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Println(err)
		return
	}

	// Initialize the client and make the request
	client := gossip.NewGossipClient(conn)
	_, err = client.Messenger(context.Background(), &gossip.Msg{Content: []byte("YOYO")})
	if err != nil {
		fmt.Println(err)
		return
	}
}

func signCert(c *x509.Certificate, caCert *x509.Certificate, privKey *rsa.PrivateKey) *x509.Certificate {
	certificate, err := x509.CreateCertificate(rand.Reader, c, caCert, c.PublicKey, privKey)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	ret, err := x509.ParseCertificate(certificate)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return ret
}

func genCert(host string) (*x509.Certificate, *rsa.PrivateKey) {
	priv, err := genKeys()
	if err != nil {
		fmt.Println(err)
		return nil, nil
	}

	serialNumber, err := genSerialNumber()
	if err != nil {
		fmt.Println(err)
		return nil, nil
	}

	ip := net.ParseIP(host)

	c := &x509.Certificate{
		SerialNumber:          serialNumber,
		BasicConstraintsValid: true,
		PublicKey:             priv.Public(),
		Subject: pkix.Name{
			Country:            []string{"China"},
			Organization:       []string{"Yjwt"},
			OrganizationalUnit: []string{"YjwtU"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		IPAddresses: []net.IP{ip},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	return c, priv
}
