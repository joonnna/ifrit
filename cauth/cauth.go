package cauth

import (
	"github.com/joonnna/capstone/logger"
	"crypto/x509"
	"crypto/rsa"
	"crypto/rand"
	"crypto"
	"math/big"
	"os"
	"strings"
	"bytes"
	"net"
	"net/http"
	"fmt"
	"io"
)

const (
	startPort = 7234
)


type Ca struct {
	log *logger.Log

	privKey *rsa.PrivateKey
	pubKey crypto.PublicKey

	groups []*group
}

type group struct {
	nodeAddrs []string
	cert *x509.Certificate
}



func NewCa() (*Ca, error) {
	privKey, err := genKeys()
	if err != nil {
		return nil, err
	}

	c := &Ca{
		log: logger.CreateLogger("ca", "caLog"),
		privKey: privKey,
		pubKey: privKey.Public(),
	}

	return c, nil
}


func (c *Ca) NewGroup() error {
	caCert := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		SubjectKeyId: []byte{1,2,3,4,5},
		BasicConstraintsValid: true,
		IsCA: true,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageDigitalSignature|x509.KeyUsageCertSign,
	}

	gCert, err := x509.CreateCertificate(rand.Reader, caCert, caCert, c.pubKey, c.privKey)
	if err != nil {
		c.log.Err.Println(err)
		return err
	}

	c.log.Debug.Println(len(gCert))

	cert, err := x509.ParseCertificate(gCert)
	if err != nil {
		c.log.Err.Println(err)
		return err
	}

	g := &group {
		cert: cert,
	}

	c.groups = append(c.groups, g)

	c.log.Info.Println("Created new a new group!")

	return nil
}

func genKeys() (*rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}

	return priv, nil
}

func (c *Ca) HttpHandler(ch chan string) {
	var l net.Listener
	var err error

	host, _ := os.Hostname()
	hostName := strings.Split(host, ".")[0]

	port := startPort

	for {
		l, err = net.Listen("tcp", fmt.Sprintf("%s:%d", hostName, port))
		if err != nil {
			c.log.Err.Println(err)
		} else {
			break
		}
		port += 1
	}

	http.HandleFunc("/downloadGroupCertificate", c.downloadHandler)
	http.HandleFunc("/certificateRequest", c.certRequestHandler)

	addr := fmt.Sprintf("http://%s:%d", hostName, port)

	ch <- addr

	err = http.Serve(l, nil)
	if err != nil {
		c.log.Err.Println(err)
		return
	}
}


func (c *Ca) certRequestHandler(w http.ResponseWriter, r *http.Request) {
	var body *bytes.Buffer
	io.Copy(body, r.Body)
	r.Body.Close()

	reqCert, err := x509.ParseCertificateRequest(body.Bytes())
	if err != nil {
		c.log.Err.Println(err)
		return
	}

	newCert, err := x509.CreateCertificate(rand.Reader, c.groups[0].cert, c.groups[0].cert, reqCert.PublicKey, c.privKey)
	if err != nil {
		c.log.Err.Println(err)
		return
	}

	/*
	cert, err := x509.ParseCertificate(newCert)
	if err != nil {
		c.log.Err.Println(err)
		return
	}
	*/
	_, err = w.Write(newCert)
	if err != nil {
		c.log.Err.Println(err)
		return
	}
}


func (c *Ca) downloadHandler(w http.ResponseWriter, r *http.Request) {

}
