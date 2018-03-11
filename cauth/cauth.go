package cauth

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joonnna/ifrit/logger"
	"github.com/joonnna/ifrit/netutil"
)

const (
	caPort = 8090
)

var (
	errNoAddr = errors.New("No network address provided in cert request!")
)

type Ca struct {
	log *logger.Log

	privKey *rsa.PrivateKey
	pubKey  crypto.PublicKey

	groups []*group

	listener net.Listener
}

type group struct {
	ringNum uint32

	knownCerts      []*x509.Certificate
	knownCertsMutex sync.RWMutex

	groupCert *x509.Certificate

	bootNodes     int
	currBootNodes int
}

// Create and returns  a new certificate authority instance.
// Generates a private/public keypair for internal use.
func NewCa() (*Ca, error) {
	privKey, err := genKeys()
	if err != nil {
		return nil, err
	}

	hostName, _ := os.Hostname()
	l, err := netutil.ListenOnPort(hostName, caPort)
	if err != nil {
		return nil, err
	}

	c := &Ca{
		log:      logger.CreateStdOutLogger("ca", "caLog"),
		privKey:  privKey,
		pubKey:   privKey.Public(),
		listener: l,
	}

	return c, nil
}

// SavePrivateKey writes the CA private key to the given io object.
func (c *Ca) SavePrivateKey(out io.Writer) {

	b := x509.MarshalPKCS1PrivateKey(c.privKey)

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: b,
	}

	pem.Encode(out, block)
}

// SaveCertificate Public key / certifiace to the given io object.
func (c *Ca) SaveCertificate(out io.Writer) error {

	for _, j := range c.groups {
		b := j.groupCert.Raw

		block := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: b,
		}
		pem.Encode(out, block)
	}

	return nil
}

// Shutsdown the certificate authority instance, will no longer serve signing requests.
func (c *Ca) Shutdown() {
	c.log.Info.Println("Shuting down certificate authority on: ", c.GetAddr())
	c.listener.Close()
}

// Starts serving certificate signing requests, requires the amount of gossip rings
// to be used in the network between ifrit clients.
func (c *Ca) Start(numRings uint32) error {
	c.log.Info.Println("Started certificate authority on: ", c.GetAddr())
	err := c.NewGroup(numRings)
	if err != nil {
		return err
	}
	return c.httpHandler()
}

// Returns the address(ip:port) of the certificate authority, this can
// be directly used as input to the ifrit client entry address.
func (c Ca) GetAddr() string {
	return c.listener.Addr().String()
}

// NewGroup creates a new Ifrit group.
func (c *Ca) NewGroup(ringNum uint32) error {
	serialNumber, err := genSerialNumber()
	if err != nil {
		c.log.Err.Println(err)
		return err
	}

	ringBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(ringBytes[0:], ringNum)

	ext := pkix.Extension{
		Id:       []int{2, 5, 13, 37},
		Critical: false,
		Value:    ringBytes,
	}

	caCert := &x509.Certificate{
		SerialNumber:          serialNumber,
		SubjectKeyId:          []byte{1, 2, 3, 4, 5},
		BasicConstraintsValid: true,
		IsCA:            true,
		NotBefore:       time.Now().AddDate(-10, 0, 0),
		NotAfter:        time.Now().AddDate(10, 0, 0),
		PublicKey:       c.pubKey,
		ExtraExtensions: []pkix.Extension{ext},
		ExtKeyUsage:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:        x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
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

	bootNodes := 3

	g := &group{
		groupCert:     cert,
		ringNum:       ringNum,
		knownCerts:    make([]*x509.Certificate, bootNodes),
		bootNodes:     bootNodes,
		currBootNodes: 0,
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

func (c *Ca) httpHandler() error {
	http.HandleFunc("/downloadGroupCertificate", c.downloadHandler)
	http.HandleFunc("/certificateRequest", c.certRequestHandler)

	err := http.Serve(c.listener, nil)
	if err != nil {
		c.log.Err.Println(err)
		return err
	}

	return nil
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
	//var oidExtensionBasicConstraints = []int{2, 5, 29, 19}
	//var oidExtensionExtendedKeyUsage = []int{2, 5, 29, 37}
	//var oidExtensionSubjectAltName = []int{2, 5, 29, 17}

	ringBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(ringBytes[0:], g.ringNum)

	ext := pkix.Extension{
		Id:       []int{2, 5, 13, 37},
		Critical: false,
		Value:    ringBytes,
	}

	if len(reqCert.Subject.Locality) < 2 {
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
		SerialNumber:    serialNumber,
		SubjectKeyId:    id,
		Subject:         reqCert.Subject,
		NotBefore:       time.Now().AddDate(-10, 0, 0),
		NotAfter:        time.Now().AddDate(10, 0, 0),
		ExtraExtensions: []pkix.Extension{ext},
		PublicKey:       reqCert.PublicKey,
		IPAddresses:     []net.IP{ip},
		ExtKeyUsage:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:        x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
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

	if g.currBootNodes < g.bootNodes {
		g.knownCerts[g.currBootNodes] = new
	}

	g.currBootNodes++

	return g.currBootNodes <= g.bootNodes
}

func (g *group) getTrustedNodes() [][]byte {
	g.knownCertsMutex.RLock()
	defer g.knownCertsMutex.RUnlock()
	var ret [][]byte

	certs := make([]*x509.Certificate, len(g.knownCerts))

	if g.currBootNodes >= g.bootNodes {
		copy(certs, g.knownCerts)
	} else {
		copy(certs, g.knownCerts[:g.currBootNodes])
	}

	for _, c := range certs {
		if c != nil {
			ret = append(ret, c.Raw)
		}
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
	//return uuid.NewV1().Bytes()

	nonce := make([]byte, 32)
	rand.Read(nonce)
	return nonce

}

func genSerialNumber() (*big.Int, error) {
	sLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	s, err := rand.Int(rand.Reader, sLimit)
	if err != nil {
		return nil, err
	}

	return s, nil
}
