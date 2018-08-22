package comm

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"os"
	"testing"

	log "github.com/inconshreveable/log15"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ClientTestSuite struct {
	suite.Suite
	c *gRPCClient
}

func TestClientTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	suite.Run(t, new(ClientTestSuite))
}

func (suite *ClientTestSuite) SetupTest() {
	conf, err := validClientConfig()
	require.NoError(suite.T(), err, "Failed to generate config")

	c, err := newClient(conf)
	require.NoError(suite.T(), err, "Failed to create client")

	suite.c = c
}

func (suite *ClientTestSuite) TestNewClient() {
	validConf, err := validClientConfig()
	require.NoError(suite.T(), err, "Failed to generate config")

	tests := []struct {
		config *tls.Config
		out    error
	}{
		{
			config: nil,
			out:    errNilConfig,
		},

		{
			config: validConf,
			out:    nil,
		},
	}

	for i, t := range tests {
		c, err := newClient(t.config)
		require.Equalf(suite.T(), t.out, err, "Invalid error output for test %d", i)

		if t.out == nil {
			require.NotNilf(suite.T(), c, "Invalid client output for test %d", i)
		} else {
			require.Nilf(suite.T(), c, "Invalid client output for test %d", i)
		}

	}
}

func validClientConfig() (*tls.Config, error) {
	priv, err := genKeys()
	if err != nil {
		return nil, err
	}

	pk := pkix.Name{
		Locality: []string{"127:0.0.1:8000", "pingAddr"},
	}

	certs, err := selfSignedCert(priv, pk)
	if err != nil {
		return nil, err
	}

	return clientConfig(certs.ownCert, certs.caCert, priv), nil
}
