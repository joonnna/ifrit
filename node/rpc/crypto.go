package rpc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"net/http"
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

func genServerConfig(certs *certSet, key *ecdsa.PrivateKey) (*tls.Config, *tls.Certificate, error) {
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certs.ownCert.Raw},
		PrivateKey:  key,
	}

	pool := x509.NewCertPool()
	pool.AddCert(certs.caCert)

	c := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	return c, &tlsCert, nil
}

func genKeys() (*ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	if err != nil {
		return nil, err
	}

	return privKey, nil
}
