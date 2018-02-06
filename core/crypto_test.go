package core

import (
	"crypto/ecdsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CryptoTestSuite struct {
	suite.Suite
	privKey *ecdsa.PrivateKey
	pubKey  *ecdsa.PublicKey
}

func (suite *CryptoTestSuite) SetupTest() {
	priv, err := genKeys()
	require.NoError(suite.T(), err, "Failed to generate keys")

	suite.privKey = priv
	suite.pubKey = &priv.PublicKey
}

func TestCryptoTestSuite(t *testing.T) {
	suite.Run(t, new(CryptoTestSuite))
}
func (suite *CryptoTestSuite) TestSignAndVerify() {
	data := genNonce()

	signature, err := signContent(data, suite.privKey)
	require.NoError(suite.T(), err, "Failed to sign")

	valid, err := validateSignature(signature.r, signature.s, data, suite.pubKey)
	require.NoError(suite.T(), err, "Failed to validate signature")

	assert.True(suite.T(), valid, "signature is invalid")
}
