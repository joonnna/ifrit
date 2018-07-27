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

	"github.com/joonnna/ifrit/netutil"
	"github.com/spf13/viper"

	log "github.com/inconshreveable/log15"
)

var (
	errNoAddr         = errors.New("No network address provided in cert request.")
	errNoPort         = errors.New("No port number specified in config.")
	errNoCertFilepath = errors.New("Tried to save public group certificates with no filepath set in config")
	errNoKeyFilepath  = errors.New("Tried to save private key with no filepath set in config")
)

type Ca struct {
	privKey *rsa.PrivateKey
	pubKey  crypto.PublicKey

	keyFilePath  string
	certFilePath string

	numRings  uint32
	bootNodes uint32

	groups []*group

	listener net.Listener
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

// Create and returns  a new certificate authority instance.
// Generates a private/public keypair for internal use.
func NewCa() (*Ca, error) {
	err := readConfig()
	if err != nil {
		return nil, err
	}

	privKey, err := genKeys()
	if err != nil {
		return nil, err
	}

	if exists := viper.IsSet("port"); !exists {
		return nil, errNoPort
	}

	l, err := netutil.ListenOnPort(viper.GetInt("port"))
	if err != nil {
		return nil, err
	}

	c := &Ca{
		privKey:      privKey,
		pubKey:       privKey.Public(),
		listener:     l,
		keyFilePath:  viper.GetString("key_filepath"),
		certFilePath: viper.GetString("certificate_filepath"),
		numRings:     uint32(viper.GetInt32("num_rings")),
		bootNodes:    uint32(viper.GetInt32("boot_nodes")),
	}

	return c, nil
}

// SavePrivateKey writes the CA private key to the given io object.
func (c *Ca) SavePrivateKey() error {
	if c.keyFilePath == "" {
		return errNoKeyFilepath
	}

	f, err := os.Create(c.keyFilePath)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	log.Info("Save CA private key.", "file", c.keyFilePath)

	b := x509.MarshalPKCS1PrivateKey(c.privKey)

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: b,
	}

	return pem.Encode(f, block)
}

// SaveCertificate Public key / certifiace to the given io object.
func (c *Ca) SaveCertificate() error {
	if c.certFilePath == "" {
		return errNoCertFilepath
	}

	f, err := os.Create(c.certFilePath)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	log.Info("Save CA certificate.", "file", c.certFilePath)

	for _, j := range c.groups {
		b := j.groupCert.Raw

		block := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: b,
		}
		err := pem.Encode(f, block)
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
func (c *Ca) Start() error {
	err := c.NewGroup(c.numRings, c.bootNodes)
	if err != nil {
		return err
	}

	log.Info("Started certificate authority", "addr", c.listener.Addr().String())

	return c.httpHandler()
}

// Returns the address(ip:port) of the certificate authority, this can
// be directly used as input to the ifrit client entry address.
func (c Ca) Addr() string {
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
	http.HandleFunc("/certificateRequest", c.certRequestHandler)

	err := http.Serve(c.listener, nil)
	if err != nil {
		log.Error(err.Error())
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

func readConfig() error {
	viper.SetConfigName("ca_config")
	viper.AddConfigPath("/var/tmp")
	viper.AddConfigPath(".")

	viper.SetConfigType("yaml")

	err := viper.ReadInConfig()
	if err != nil {
		log.Error(err.Error())
		return err
	}

	viper.SetDefault("num_rings", 3)
	viper.SetDefault("boot_nodes", 1)

	viper.WriteConfig()

	return nil
}
