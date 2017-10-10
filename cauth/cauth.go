package cauth

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/joonnna/capstone/logger"
	"github.com/satori/go.uuid"
)

var (
	errNoAddr = errors.New("No network address provided in cert request!")
)

const (
	startPort = 7234
)

type Ca struct {
	log *logger.Log

	privKey *rsa.PrivateKey
	pubKey  crypto.PublicKey

	groups []*group
}

type group struct {
	ringNum uint8

	knownCerts      []*x509.Certificate
	knownCertsMutex sync.RWMutex

	groupCert *x509.Certificate

	bootNodes int
}

func NewCa() (*Ca, error) {
	privKey, err := genKeys()
	if err != nil {
		return nil, err
	}

	c := &Ca{
		log:     logger.CreateLogger("ca", "caLog"),
		privKey: privKey,
		pubKey:  privKey.Public(),
	}

	return c, nil
}

func (c *Ca) NewGroup(ringNum uint8) error {
	serialNumber, err := genSerialNumber()
	if err != nil {
		c.log.Err.Println(err)
		return err
	}

	caCert := &x509.Certificate{
		SerialNumber:          serialNumber,
		SubjectKeyId:          []byte{1, 2, 3, 4, 5},
		BasicConstraintsValid: true,
		IsCA:        true,
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		PublicKey:   c.pubKey,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	gCert, err := x509.CreateCertificate(rand.Reader, caCert, caCert, c.pubKey, c.privKey)
	if err != nil {
		c.log.Err.Println(err)
		return err
	}

	cert, err := x509.ParseCertificate(gCert)
	if err != nil {
		c.log.Err.Println(err)
		return err
	}

	g := &group{
		groupCert:  cert,
		ringNum:    ringNum,
		knownCerts: make([]*x509.Certificate, 0),
		bootNodes:  3,
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

	/*
		host, _ := os.Hostname()
		hostName := strings.Split(host, ".")[0]
	*/
	hostName := getLocalIP()

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
	var body bytes.Buffer

	io.Copy(&body, r.Body)
	r.Body.Close()

	reqCert, err := x509.ParseCertificateRequest(body.Bytes())
	if err != nil {
		c.log.Err.Println(err)
		return
	}

	g := c.groups[0]

	c.log.Info.Println("Got a certificate request from", reqCert.Subject.Locality)

	//No idea what this is
	var oidExtensionBasicConstraints = []int{2, 5, 29, 19}

	extensions := []pkix.Extension{
		pkix.Extension{
			Id: oidExtensionBasicConstraints, Critical: true, Value: []uint8{g.ringNum},
		},
	}
	if len(reqCert.Subject.Locality[0]) < 1 {
		c.log.Err.Println(errNoAddr)
		return
	}

	serialNumber, err := genSerialNumber()
	if err != nil {
		c.log.Err.Println(err)
		return
	}

	ip := net.ParseIP(strings.Split(reqCert.Subject.Locality[0], ":")[0])
	id := genId()

	newCert := &x509.Certificate{
		SerialNumber:          serialNumber,
		SubjectKeyId:          id,
		Subject:               reqCert.Subject,
		BasicConstraintsValid: true,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		Extensions:            extensions,
		PublicKey:             reqCert.PublicKey,
		IPAddresses:           []net.IP{ip},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	signedCert, err := x509.CreateCertificate(rand.Reader, newCert, g.groupCert, reqCert.PublicKey, c.privKey)
	if err != nil {
		c.log.Err.Println(err)
		return
	}

	knownCert, err := x509.ParseCertificate(signedCert)
	if err != nil {
		c.log.Err.Println(err)
		return
	}

	trusted := g.addKnownCert(knownCert)

	respStruct := struct {
		OwnCert    []byte
		KnownCerts [][]byte
		CaCert     []byte
		Trusted    bool
	}{
		OwnCert:    signedCert,
		KnownCerts: g.getTrustedNodes(),
		CaCert:     g.groupCert.Raw,
		Trusted:    trusted,
	}

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(respStruct)

	_, err = w.Write(b.Bytes())
	if err != nil {
		c.log.Err.Println(err)
		return
	}
}

func (c *Ca) downloadHandler(w http.ResponseWriter, r *http.Request) {
	c.log.Info.Println("Got a download request!")
}

func (g *group) addKnownCert(new *x509.Certificate) bool {
	g.knownCertsMutex.Lock()
	defer g.knownCertsMutex.Unlock()

	g.knownCerts = append(g.knownCerts, new)

	return len(g.knownCerts) <= g.bootNodes
}

func (g *group) getTrustedNodes() [][]byte {
	g.knownCertsMutex.RLock()
	defer g.knownCertsMutex.RUnlock()
	var certs []*x509.Certificate
	var ret [][]byte

	if len(g.knownCerts) < g.bootNodes {
		certs = g.knownCerts
	} else {
		certs = g.knownCerts[:g.bootNodes]
	}

	for _, c := range certs {
		ret = append(ret, c.Raw)
	}

	return ret
}

func genId() []byte {
	/*
		var id []byte

		for {
			id = uuid.NewV4().Bytes()
			exists := false

			for _, c := range existing {
				if bytes.Equal(c.SubjectKeyId, id) {
					exists = true
					break
				}
			}

			if !exists {
				break
			}
		}

		return id
	*/

	//return append(uuid.NewV1().Bytes(), uuid.NewV1().Bytes()...)
	return uuid.NewV1().Bytes()
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

func genSerialNumber() (*big.Int, error) {
	sLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	s, err := rand.Int(rand.Reader, sLimit)
	if err != nil {
		return nil, err
	}

	return s, nil
}
