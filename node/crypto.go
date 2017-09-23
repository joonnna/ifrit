package node


import (


)



func validateSignature() {

}


func

func sendCertRequest() {
	template := x509.CertificateRequest{
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	certReqBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, keyBytes)
}


func genKeys() (*crypto.PrivateKey, *crypto.PublicKey, error ){
	privKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, nil, err
	}

	return privKey, privKey.Public(), nil
}
