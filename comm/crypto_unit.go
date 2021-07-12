package comm

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
)

var (
	errNoRingNum   = errors.New("No ringnumber present in received certificate")
	errNoIp        = errors.New("No ip present in received identity")
	errNoAddrs     = errors.New("Not enough addresses present in identity")
	errInvlPath    = errors.New("Argument path to load empty")
	errPemDecode   = errors.New("Unable to decode content in given file")
	errInvlKeyPath = errors.New("Storage path-argument is invalid")
)

type CryptoUnit struct {
	priv   *ecdsa.PrivateKey
	pk     pkix.Name
	caAddr string

	self       *x509.Certificate
	ca         *x509.Certificate
	numRings   uint32
	knownCerts []*x509.Certificate
	trusted    bool
}

type certResponse struct {
	OwnCert    []byte
	KnownCerts [][]byte
	CaCert     []byte
	Trusted    bool
}

type certSet struct {
	ownCert    *x509.Certificate
	caCert     *x509.Certificate
	knownCerts []*x509.Certificate
	trusted    bool
}

func NewCu(identity pkix.Name, caAddr string, dnsLabel string) (*CryptoUnit, error) {
	var certs *certSet
	var extValue []byte

	if addrs := len(identity.Locality); addrs < 2 {
		return nil, errNoAddrs
	}

	serviceAddr := strings.Split(identity.Locality[0], ":")
	if len(serviceAddr) <= 0 {
		return nil, errNoIp
	}

	// serviceIP, err := net.LookupIP(serviceAddr[0])
	// if err != nil {
	// 	return nil, err
	// }

	// ip := net.ParseIP(serviceIP[0].String())
	// if ip == nil {
	// 	return nil, errNoIp
	// }

	priv, err := genKeys()
	if err != nil {
		return nil, err
	}

	if caAddr != "" {
		addr := fmt.Sprintf("http://%s/certificateRequest", caAddr)
		certs, err = sendCertRequest(priv, addr, identity, dnsLabel)
		if err != nil {
			return nil, err
		}

	} else {
		// TODO only have numrings in notes and not certificate?
		certs, err = selfSignedCert(priv, identity)
		if err != nil {
			return nil, err
		}
	}

	for _, e := range certs.ownCert.Extensions {
		if e.Id.Equal(asn1.ObjectIdentifier{2, 5, 13, 37}) {
			extValue = e.Value
		}
	}

	if extValue == nil {
		return nil, errNoRingNum
	}

	numRings := binary.LittleEndian.Uint32(extValue[0:])

	return &CryptoUnit{
		ca:         certs.caCert,
		self:       certs.ownCert,
		numRings:   numRings,
		caAddr:     caAddr,
		pk:         identity,
		priv:       priv,
		knownCerts: certs.knownCerts,
		trusted:    certs.trusted,
	}, nil
}

func LoadCu(certPath string, identity pkix.Name, caAddr string) (*CryptoUnit, error) {
	var extValue []byte

	if certPath == "" {
		return nil, errInvlPath
	}

	certs, err := loadCertSet(certPath)
	if err != nil {
		return nil, err
	}

	priv, err := loadPrivKey(certPath)
	if err != nil {
		return nil, err
	}

	for _, e := range certs.ownCert.Extensions {
		if e.Id.Equal(asn1.ObjectIdentifier{2, 5, 13, 37}) {
			extValue = e.Value
		}
	}

	if extValue == nil {
		return nil, errNoRingNum
	}

	numRings := binary.LittleEndian.Uint32(extValue[0:])

	kake := &CryptoUnit{
		ca:         certs.caCert,
		self:       certs.ownCert,
		numRings:   numRings,
		caAddr:     caAddr,
		pk:         identity,
		priv:       priv,
		knownCerts: certs.knownCerts,
		trusted:    certs.trusted,
	}

	return kake, nil
}

func (cu *CryptoUnit) Trusted() bool {
	return cu.trusted
}

func (cu *CryptoUnit) Certificate() *x509.Certificate {
	return cu.self
}

func (cu *CryptoUnit) CaCertificate() *x509.Certificate {
	return cu.ca
}

func (cu *CryptoUnit) NumRings() uint32 {
	return cu.numRings
}

func (cu *CryptoUnit) Priv() *ecdsa.PrivateKey {
	return cu.priv
}

func (cu *CryptoUnit) ContactList() []*x509.Certificate {
	ret := make([]*x509.Certificate, 0, len(cu.knownCerts))

	for _, c := range cu.knownCerts {
		ret = append(ret, c)
	}

	return ret
}

func (cu *CryptoUnit) Verify(data, r, s []byte, pub *ecdsa.PublicKey) bool {
	if pub == nil {
		log.Error("Peer had no publicKey")
		return false
	}

	var rInt, sInt big.Int

	b := hashContent(data)

	rInt.SetBytes(r)
	sInt.SetBytes(s)

	return ecdsa.Verify(pub, b, &rInt, &sInt)
}

func (cu *CryptoUnit) Sign(data []byte) ([]byte, []byte, error) {
	hash := hashContent(data)

	r, s, err := ecdsa.Sign(rand.Reader, cu.priv, hash)
	if err != nil {
		return nil, nil, err
	}

	return r.Bytes(), s.Bytes(), nil
}

/* Save private key for node crypto-unit to new file in argument directory-path.
 * - marius
 */
func (cu *CryptoUnit) SavePrivateKey(path string) error {

	path = filepath.Join(path, fmt.Sprintf("certificate-%s", cu.self.SerialNumber))

	err := os.MkdirAll(path, fs.ModePerm)
	if err != nil {
		return err
	}

	path = filepath.Join(path, "key.pem")

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	keyBytes, err := x509.MarshalECPrivateKey(cu.priv)
	if err != nil {
		return err
	}

	block := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	}

	err = pem.Encode(f, block)
	if err != nil {
		return err
	}

	log.Info("private-key stored", "path", path)

	return f.Close()
}

/* Save network-neighbours, ca, and own certificate in new files inside argument path.
 * - marius
 */
func (cu *CryptoUnit) SaveCertificate(path string) error {

	path = filepath.Join(path, fmt.Sprintf("certificate-%s", cu.self.SerialNumber))

	err := os.MkdirAll(path, fs.ModePerm)
	if err != nil {
		return err
	}

	/*
	 * Neigbours.
	 */
	for i, knownCert := range cu.knownCerts {
		fname := filepath.Join(path, fmt.Sprintf("g-%s.pem", knownCert.SerialNumber))

		err = saveCert(knownCert, fname)
		if err != nil {
			log.Error(err.Error())
		}

		log.Info(fmt.Sprintf("known-certificate #%v stored", i+1), "path", fname)
	}

	/*
	 * CA.
	 */
	fname := filepath.Join(path, fmt.Sprintf("ca-%s.pem", cu.ca.SerialNumber))

	err = saveCert(cu.ca, fname)
	if err != nil {
		log.Error(err.Error())
	}

	log.Info("CA-certificate stored", "path", fname)

	/*
	 * Self.
	 */
	fname = filepath.Join(path, fmt.Sprintf("self-%s.pem", cu.self.SerialNumber))

	err = saveCert(cu.self, fname)
	if err != nil {
		log.Error(err.Error())
	}

	log.Info("own-certificate stored", "path", fname)

	return nil
}

func saveCert(cert *x509.Certificate, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}

	if err = pem.Encode(f, block); err != nil {
		_ = f.Close()
		return err
	}

	if err = f.Close(); err != nil {
		return err
	}

	return nil
}

func loadCertSet(certPath string) (*certSet, error) {
	if certPath == "" {
		return nil, errInvlPath
	}

	/*
	 * Personal certificate.
	 */
	matches, err := filepath.Glob(filepath.Join(certPath, "self-*.pem"))
	if err != nil {
		return nil, err
	} else if matches == nil {
		return nil, errors.New(fmt.Sprintf("Storage path '%s' gave no hits on certificates", certPath))
	} else if len(matches) > 1 {
		return nil, errors.New(fmt.Sprintf("Argument path: \"%s\" contains more than one private-key", certPath))
	}

	selfCert, err := loadCert(matches[0])
	if err != nil {
		return nil, err
	}

	log.Info("own-certificate loaded", "path", matches[0])

	/*
	 * Neighbour certificates.
	 */
	matches, err = filepath.Glob(filepath.Join(certPath, "g-*.pem"))
	if err != nil {
		return nil, err
	} else if matches == nil {
		return nil, errors.New(fmt.Sprintf("Storage path '%s' gave no hits on certificates", certPath))
	}

	knownCerts := make([]*x509.Certificate, 0, len(matches))

	for i, path := range matches {
		gCert, err := loadCert(path)
		if err != nil {
			return nil, err
		}

		knownCerts = append(knownCerts, gCert)

		log.Info(fmt.Sprintf("known certificate #%d loaded", i+1), "path", path)
	}

	/*
	 * CA certificate.
	 */
	matches, err = filepath.Glob(filepath.Join(certPath, "ca-*.pem"))
	if err != nil {
		return nil, err
	} else if len(matches) > 1 {
		return nil, errors.New(fmt.Sprintf("Argument path: \"%s\" contains more than CA", certPath))
	}

	caCert, err := loadCert(matches[0])
	if err != nil {
		return nil, err
	}

	log.Info("ca-certificate loaded", "path", matches[0])

	return &certSet{
		ownCert:    selfCert,
		knownCerts: knownCerts,
		caCert:     caCert,
		trusted:    true,
	}, nil
}

func loadCert(path string) (*x509.Certificate, error) {
	certPem, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	certBlock, _ := pem.Decode(certPem)
	if certBlock == nil {
		return nil, errPemDecode
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

func loadPrivKey(certPath string) (*ecdsa.PrivateKey, error) {
	if certPath == "" {
		return nil, errInvlPath
	}

	path := filepath.Join(certPath, "key.pem")

	keyPem, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	keyBlock, _ := pem.Decode(keyPem)
	if keyBlock == nil {
		return nil, errPemDecode
	}

	privKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, err
	}

	log.Info("private-key loaded", "path", path)

	return privKey, nil
}

func sendCertRequest(privKey *ecdsa.PrivateKey, caAddr string, pk pkix.Name, dnsLabel string) (*certSet, error) {
	var certs certResponse
	set := &certSet{}

	template := x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
		Subject:            pk,
		DNSNames:           []string{dnsLabel},
	}

	certReqBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, privKey)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(caAddr, "text", bytes.NewBuffer(certReqBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&certs)
	if err != nil {
		return nil, err
	}

	set.ownCert, err = x509.ParseCertificate(certs.OwnCert)
	if err != nil {
		return nil, err
	}

	set.caCert, err = x509.ParseCertificate(certs.CaCert)
	if err != nil {
		return nil, err
	}

	for _, b := range certs.KnownCerts {
		c, err := x509.ParseCertificate(b)
		if err != nil {
			return nil, err
		}
		set.knownCerts = append(set.knownCerts, c)
	}

	set.trusted = certs.Trusted

	return set, nil
}

func selfSignedCert(priv *ecdsa.PrivateKey, pk pkix.Name) (*certSet, error) {
	ringBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(ringBytes[0:], uint32(32))

	ext := pkix.Extension{
		Id:       []int{2, 5, 13, 37},
		Critical: false,
		Value:    ringBytes,
	}

	serial, err := genSerialNumber()
	if err != nil {
		return nil, err
	}

	serviceAddr := strings.Split(pk.Locality[0], ":")
	if len(serviceAddr) <= 0 {
		return nil, errNoIp
	}

	ip := net.ParseIP(serviceAddr[0])
	if ip == nil {
		return nil, errNoIp
	}

	// TODO generate ids and serial numbers differently
	newCert := &x509.Certificate{
		SerialNumber:          serial,
		SubjectKeyId:          genId(),
		Subject:               pk,
		BasicConstraintsValid: true,
		NotBefore:             time.Now().AddDate(-10, 0, 0),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		ExtraExtensions:       []pkix.Extension{ext},
		PublicKey:             priv.PublicKey,
		IPAddresses:           []net.IP{ip},
		IsCA:                  true,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment |
			x509.KeyUsageCertSign,
	}

	signedCert, err := x509.CreateCertificate(rand.Reader, newCert,
		newCert, priv.Public(), priv)
	if err != nil {
		return nil, err
	}

	parsed, err := x509.ParseCertificate(signedCert)
	if err != nil {
		return nil, err
	}

	return &certSet{ownCert: parsed}, nil
}

func genId() []byte {
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

func genKeys() (*ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, err
	}

	return privKey, nil
}

func hashContent(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}
