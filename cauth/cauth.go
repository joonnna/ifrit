package cauth

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"
)

var (
	errNoAddr         = errors.New("No network address provided in cert request.")
	errNoPort         = errors.New("No port number specified in config.")
	errNoCertFilepath = errors.New("Tried to save public group certificates with no filepath set in config")
	errNoKeyFilepath  = errors.New("Tried to save private key with no filepath set in config")

	RingNumberOid asn1.ObjectIdentifier = []int{2, 5, 13, 37}
)

type Ca struct {
	privKey *rsa.PrivateKey
	pubKey  crypto.PublicKey

	path        string
	keyFilePath string

	groups []*group

	listener   net.Listener
	httpServer *http.Server
}

type group struct {
	knownCerts      []*x509.Certificate
	knownCertsMutex sync.RWMutex

	existingIds map[string]bool
	idMutex     sync.RWMutex

	groupCert *x509.Certificate

	bootNodes     uint32
	currBootNodes uint32

	numRings uint32
}

// LoadCa initializes a CA from a file path
func LoadCa(path string) (*Ca, error) {
	keyPath := filepath.Join(path, "key.pem")

	fp, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	// Load private key
	keyBlock, _ := pem.Decode(fp)
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)

	if err != nil {
		return nil, err
	}

	c := &Ca{
		privKey:     key,
		pubKey:      key.Public(),
		path:        path,
		keyFilePath: "key.pem",
	}

	// Load group certificates
	groupCertFiles, err := filepath.Glob(filepath.Join(path, "g-*.pem"))
	if err != nil {
		return nil, err
	}

	for _, fileName := range groupCertFiles {

		g := &group{
			knownCerts:  make([]*x509.Certificate, 0),
			bootNodes:   0,
			numRings:    4, // default value
			existingIds: make(map[string]bool),
		}

		// Read group certificate
		fp, err := ioutil.ReadFile(fileName)
		if err != nil {
			return nil, err
		}
		certBlock, _ := pem.Decode(fp)
		cert, err := x509.ParseCertificate(certBlock.Bytes)

		g.groupCert = cert

		// Search for number of rings extension
		for _, ext := range cert.Extensions {
			if ext.Id.Equal(RingNumberOid) {
				g.numRings = binary.LittleEndian.Uint32(ext.Value)
				break
			}
		}

		// Add group object to CA
		c.groups = append(c.groups, g)
		log.Info("add group", "serial", g.groupCert.SerialNumber.String(), "numrings", g.numRings)
	}

	return c, nil
}

// Create and returns  a new certificate authority instance.
// Generates a private/public keypair for internal use.
func NewCa(path string) (*Ca, error) {
	privKey, err := genKeys()
	if err != nil {
		return nil, err
	}

	c := &Ca{
		privKey:     privKey,
		pubKey:      privKey.Public(),
		path:        path,
		keyFilePath: "key.pem",
	}

	return c, nil
}

// SavePrivateKey writes the CA private key to the given io object.
func (c *Ca) SavePrivateKey() error {
	if c.keyFilePath == "" {
		return errNoKeyFilepath
	}

	p := filepath.Join(c.path, c.keyFilePath)

	f, err := os.Create(p)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	b := x509.MarshalPKCS1PrivateKey(c.privKey)

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: b,
	}

	return pem.Encode(f, block)
}

// SaveCertificate Public key / certificate to the given io object.
func (c *Ca) SaveCertificate() error {
	for _, j := range c.groups {

		p := filepath.Join(c.path, fmt.Sprintf("g-%s.pem", j.groupCert.SerialNumber))

		f, err := os.Create(p)
		if err != nil {
			log.Error(err.Error())
			return err
		}

		b := j.groupCert.Raw

		block := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: b,
		}
		err = pem.Encode(f, block)
		if err != nil {
			return err
		}
	}

	return nil
}

// Shutsdown the certificate authority instance, will no longer serve signing requests.
func (c *Ca) Shutdown() {
	log.Info("Shuting down certificate authority")
	c.listener.Close()
}

// Starts serving certificate signing requests, requires the amount of gossip rings
// to be used in the network between ifrit clients.
func (c *Ca) Start(host string, port int) error {
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	c.listener = l

	log.Info("Started certificate authority", "addr", c.listener.Addr().String())

	return c.httpHandler()
}

// Returns the address(ip:port) of the certificate authority, this can
// be directly used as input to the ifrit client entry address.
func (c *Ca) Addr() string {
	return c.listener.Addr().String()
}

// NewGroup creates a new Ifrit group.
func (c *Ca) NewGroup(ringNum, bootNodes uint32) error {
	serialNumber, err := genSerialNumber()
	if err != nil {
		log.Error(err.Error())
		return err
	}

	ringBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(ringBytes[0:], ringNum)

	ext := pkix.Extension{
		Id:       RingNumberOid,
		Critical: false,
		Value:    ringBytes,
	}

	caCert := &x509.Certificate{
		SerialNumber:          serialNumber,
		SubjectKeyId:          []byte{1, 2, 3, 4, 5},
		BasicConstraintsValid: true,
		IsCA:                  true,
		NotBefore:             time.Now().AddDate(-10, 0, 0),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		PublicKey:             c.pubKey,
		ExtraExtensions:       []pkix.Extension{ext},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	gCert, err := x509.CreateCertificate(rand.Reader, caCert, caCert, c.pubKey, c.privKey)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	cert, err := x509.ParseCertificate(gCert)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	g := &group{
		groupCert:   cert,
		numRings:    ringNum,
		knownCerts:  make([]*x509.Certificate, bootNodes),
		bootNodes:   bootNodes,
		existingIds: make(map[string]bool),
	}

	c.groups = append(c.groups, g)

	log.Info("Created new a new group!")

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
	mux := http.NewServeMux()
	mux.HandleFunc("/certificateRequest", c.certificateSigning)

	c.httpServer = &http.Server{
		Handler: mux,
	}

	err := c.httpServer.Serve(c.listener)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	return nil
}

func (c *Ca) certificateSigning(w http.ResponseWriter, r *http.Request) {
	var body bytes.Buffer
	io.Copy(&body, r.Body)
	r.Body.Close()

	reqCert, err := x509.ParseCertificateRequest(body.Bytes())
	if err != nil {
		log.Error(err.Error())
		return
	}

	g := c.groups[0]

	log.Info("Got a certificat request", "addr", reqCert.Subject.Locality)

	//No idea what this is
	//var oidExtensionBasicConstraints = []int{2, 5, 29, 19}
	//var oidExtensionExtendedKeyUsage = []int{2, 5, 29, 37}
	//var oidExtensionSubjectAltName = []int{2, 5, 29, 17}

	ringBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(ringBytes[0:], g.numRings)

	ext := pkix.Extension{
		Id:       []int{2, 5, 13, 37},
		Critical: false,
		Value:    ringBytes,
	}

	if len(reqCert.Subject.Locality) < 2 {
		log.Error(errNoAddr.Error())
		return
	}

	serialNumber, err := genSerialNumber()
	if err != nil {
		log.Error(err.Error())
		return
	}

	ipAddr, err := net.ResolveIPAddr("ip4", strings.Split(reqCert.Subject.Locality[0], ":")[0])
	if err != nil {
		log.Error(err.Error())
		return
	}
	id := g.genId()

	newCert := &x509.Certificate{
		SerialNumber:    serialNumber,
		SubjectKeyId:    id,
		Subject:         reqCert.Subject,
		NotBefore:       time.Now().AddDate(-10, 0, 0),
		NotAfter:        time.Now().AddDate(10, 0, 0),
		ExtraExtensions: []pkix.Extension{ext},
		PublicKey:       reqCert.PublicKey,
		IPAddresses:     []net.IP{ipAddr.IP},
		ExtKeyUsage:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:        x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	signedCert, err := x509.CreateCertificate(rand.Reader, newCert, g.groupCert, reqCert.PublicKey, c.privKey)
	if err != nil {
		log.Error(err.Error())
		return
	}

	knownCert, err := x509.ParseCertificate(signedCert)
	if err != nil {
		log.Error(err.Error())
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

	log.Info("Including known certificates in response", "amount", len(respStruct.KnownCerts))

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(respStruct)

	_, err = w.Write(b.Bytes())
	if err != nil {
		log.Error(err.Error())
		return
	}
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
	var certs []*x509.Certificate

	if g.currBootNodes >= g.bootNodes {
		certs = make([]*x509.Certificate, g.bootNodes)
		copy(certs, g.knownCerts)
	} else {
		certs = make([]*x509.Certificate, g.currBootNodes)
		copy(certs, g.knownCerts[:g.currBootNodes])
	}

	for _, c := range certs {
		ret = append(ret, c.Raw)
	}

	return ret
}

func (g *group) genId() []byte {
	g.idMutex.Lock()
	defer g.idMutex.Unlock()

	nonce := make([]byte, 32)

	for {
		_, err := rand.Read(nonce)
		if err != nil {
			log.Error(err.Error())
			continue
		}

		key := string(nonce)

		if _, ok := g.existingIds[key]; !ok {
			g.existingIds[key] = true
			break
		}

	}

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
