package core

import (
	"crypto/ecdsa"
	"testing"

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
func (suite *CryptoTestSuite) TestSignContent() {
	data := genNonce()

	_, _, err := signContent(data, suite.privKey)
	require.NoError(suite.T(), err, "Failed to sign")
}
