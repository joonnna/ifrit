package node

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"
)

var (
	errSignature = errors.New("Can't unmarshal publickey")
)

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

	/*
		//testing
		//bytes := elliptic.Marshal(elliptic.P224(), r, s)
		//bytes := asn1.Marshal(elliptic.P224(), r, s)
		valid, err := validateSignature(r, s, data, &privKey.PublicKey)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(valid)
		}
	*/
	return &signature{r: r.Bytes(), s: s.Bytes()}, nil
}

func validateSignature(r, s, data []byte, pub *ecdsa.PublicKey) (bool, error) {
	var rInt, sInt big.Int
	/*
		r, s := elliptic.Unmarshal(elliptic.P224(), signature)
		if r == nil {
			return false, errPubKey
		}
	*/

	b := hashContent(data)

	rInt.SetBytes(r)
	sInt.SetBytes(s)

	return ecdsa.Verify(pub, b, &rInt, &sInt), nil
}
