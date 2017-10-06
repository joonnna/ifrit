package node

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
)

var (
	errSignature = errors.New("Can't unmarshal publickey")
)

type certResponse struct {
	OwnCert    []byte
	KnownCerts [][]byte
	CaCert     []byte
}

type certSet struct {
	ownCert    *x509.Certificate
	caCert     *x509.Certificate
	knownCerts []*x509.Certificate
}

func hashContent(data []byte) []byte {
	h := sha256.New224()
	//h := md5.New()
	h.Write(data)
	return h.Sum(nil)
}

func signContent(data []byte, privKey *ecdsa.PrivateKey) (*signature, error) {
	b := hashContent(data)

	r, s, err := ecdsa.Sign(rand.Reader, privKey, b)
	if err != nil {
		return nil, err
	}

	return &signature{r: r.Bytes(), s: s.Bytes()}, nil
}

func validateSignature(r, s, data []byte, pub *ecdsa.PublicKey) (bool, error) {
	var rInt, sInt big.Int

	b := hashContent(data)

	rInt.SetBytes(r)
	sInt.SetBytes(s)

	return ecdsa.Verify(pub, b, &rInt, &sInt), nil
}

func sendCertRequest(caAddr string, privKey *ecdsa.PrivateKey, localAddr string) (*certSet, error) {
	var certs certResponse

	set := &certSet{}

	s := pkix.Name{
		Locality: []string{localAddr},
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

	return set, nil
}

func genServerConfig(certs *certSet, key *ecdsa.PrivateKey) *tls.Config {
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certs.ownCert.Raw},
		PrivateKey:  key,
	}

	pool := x509.NewCertPool()
	pool.AddCert(certs.caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
}

func genClientConfig(certs *certSet, key *ecdsa.PrivateKey) *tls.Config {
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certs.ownCert.Raw},
		PrivateKey:  key,
	}

	pool := x509.NewCertPool()
	pool.AddCert(certs.caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		RootCAs:      pool,
	}
}

func genKeys() (*ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	if err != nil {
		return nil, err
	}

	return privKey, nil
}
