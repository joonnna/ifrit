package node


import (


)



func sendCertRequest() {
	keyBytes, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return err
	}



	template := x509.CertificateRequest{
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	certReqBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, keyBytes)
}
