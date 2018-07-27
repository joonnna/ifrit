package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/core/discovery"
	"github.com/joonnna/ifrit/protobuf"
)

var (
	errSignature = errors.New("Can't unmarshal publickey")
	errNoIp      = errors.New("Can't find ip in serviceAddr")
)

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

func hashContent(data []byte) []byte {
	h := sha256.New224()
	h.Write(data)
	return h.Sum(nil)
}

func signContent(data []byte, privKey *ecdsa.PrivateKey) ([]byte, []byte, error) {
	b := hashContent(data)

	r, s, err := ecdsa.Sign(rand.Reader, privKey, b)
	if err != nil {
		return nil, nil, err
	}

	return r.Bytes(), s.Bytes(), nil
}

func sendCertRequest(privKey *ecdsa.PrivateKey, caAddr, serviceAddr, pingAddr, httpAddr string) (*certSet, error) {
	var certs certResponse
	set := &certSet{}

	s := pkix.Name{
		Locality: []string{serviceAddr, pingAddr, httpAddr},
	}

	template := x509.CertificateRequest{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
		Subject:            s,
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

func genServerConfig(certs *certSet, key *ecdsa.PrivateKey) *tls.Config {
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certs.ownCert.Raw},
		PrivateKey:  key,
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	if certs.caCert == nil {
		conf.ClientAuth = tls.RequestClientCert
	} else {
		pool := x509.NewCertPool()
		pool.AddCert(certs.caCert)
		conf.ClientCAs = pool
		conf.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return conf
}

func genClientConfig(certs *certSet, key *ecdsa.PrivateKey) *tls.Config {
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certs.ownCert.Raw},
		PrivateKey:  key,
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	if certs.caCert != nil {
		pool := x509.NewCertPool()
		pool.AddCert(certs.caCert)
		conf.RootCAs = pool
	} else {
		conf.InsecureSkipVerify = true
	}

	return conf
}

func genKeys() (*ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	if err != nil {
		return nil, err
	}

	return privKey, nil
}

func genNonce() []byte {
	nonce := make([]byte, 32)
	rand.Read(nonce)
	return nonce
}

func checkCertificateSignature(c *x509.Certificate, ca *x509.Certificate) error {
	if ca == nil {
		//return c.CheckSignature(c.SignatureAlgorithm, c.Raw, c.Signature)
		return c.CheckSignatureFrom(c)
	} else {
		return c.CheckSignatureFrom(ca)
	}
}

func selfSignedCert(priv *ecdsa.PrivateKey, serviceAddr, pingAddr, httpAddr string, numRings uint32) (*certSet, error) {
	ringBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(ringBytes[0:], numRings)

	ext := pkix.Extension{
		Id:       []int{2, 5, 13, 37},
		Critical: false,
		Value:    ringBytes,
	}

	s := pkix.Name{
		Locality: []string{serviceAddr, pingAddr, httpAddr},
	}

	str := strings.Split(serviceAddr, ":")
	if len(str) <= 0 {
		return nil, errNoIp
	}

	ip := net.ParseIP(str[0])
	if ip == nil {
		return nil, errNoIp
	}

	serial, err := genSerialNumber()
	if err != nil {
		return nil, err
	}

	// TODO generate ids and serial numbers differently
	newCert := &x509.Certificate{
		SerialNumber:          serial,
		SubjectKeyId:          genId(),
		Subject:               s,
		BasicConstraintsValid: true,
		NotBefore:             time.Now().AddDate(-10, 0, 0),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		ExtraExtensions:       []pkix.Extension{ext},
		PublicKey:             priv.PublicKey,
		IPAddresses:           []net.IP{ip},
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
	}

	signedCert, err := x509.CreateCertificate(rand.Reader, newCert, newCert, priv.Public(), priv)
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

func checkAccusationSignature(a *gossip.Accusation, accuser *discovery.Peer) bool {
	tmp := &gossip.Accusation{
		Epoch:   a.GetEpoch(),
		Accuser: a.GetAccuser(),
		Accused: a.GetAccused(),
		Mask:    a.GetMask(),
		RingNum: a.GetRingNum(),
	}

	b, err := proto.Marshal(tmp)
	if err != nil {
		log.Error(err.Error())
		return false
	}

	sign := a.GetSignature()

	if valid := accuser.ValidateSignature(sign.GetR(), sign.GetS(), b); !valid {
		log.Debug("Invalid signature on accusation, ignoring")
		return false
	}

	return true
}

func checkNoteSignature(n *gossip.Note, p *discovery.Peer) bool {
	tmp := &gossip.Note{
		Epoch: n.GetEpoch(),
		Mask:  n.GetMask(),
		Id:    n.GetId(),
	}

	b, err := proto.Marshal(tmp)
	if err != nil {
		log.Error(err.Error())
		return false
	}

	sign := n.GetSignature()

	if valid := p.ValidateSignature(sign.GetR(), sign.GetS(), b); !valid {
		log.Debug("Invalid signature on note, ignoring")
		return false
	}

	return true
}
