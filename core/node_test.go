package core

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"os"
	"testing"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"

	pb "github.com/joonnna/ifrit/protobuf"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type NodeTestSuite struct {
	suite.Suite
	nodes []*Node
}

func TestNodeTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	viper.Set("use_ca", false)
	viper.Set("use_viz", false)

	suite.Run(t, new(NodeTestSuite))
}

func (suite *NodeTestSuite) SetupTest() {
	for i := 0; i < 30; i++ {

		priv, err := genKeys()
		require.NoError(suite.T(), err, "Failed to generate keys")

		ownCert := genCert(priv, 10)
		n, err := NewNode(&commStub{}, &pingStub{}, &cmStub{cert: ownCert}, &cryptoStub{priv: priv})
		require.NoError(suite.T(), err, "Failed to create node.")

		suite.nodes = append(suite.nodes, n)
	}
}

func (suite *NodeTestSuite) TestGossip() {
	// Everyone gossips with everyone,
	// then assert that everyone has the same view.

	for _, n := range suite.nodes {
		for _, n2 := range suite.nodes {
			reply, err := n2.Spread(context.Background(), n.collectGossipContent())
			if err != nil {
				continue
			}
			n.mergeCertificates(reply.GetCertificates())
			n.mergeNotes(reply.GetNotes())
			n.mergeAccusations(reply.GetAccusations())
		}
	}

	for _, n := range suite.nodes {
		for _, n2 := range suite.nodes {
			require.NoError(suite.T(), n.view.Compare(n2.view), "Views are not equal.")
		}
	}

}

type clientStub struct {
}

func (cs *clientStub) Init(config *tls.Config) {
}

func (cs *clientStub) Gossip(addr string, args *pb.State) (*pb.StateResponse, error) {
	return nil, nil
}

func (cs *clientStub) SendMsg(addr string, args *pb.Msg) (*pb.MsgResponse, error) {
	return nil, nil
}

func (cs *clientStub) CloseConn(addr string) {
}

type serverStub struct {
}

func (ss *serverStub) Init(config *tls.Config, n interface{}, maxConcurrent uint32) error {
	return nil
}

func (ss *serverStub) Addr() string {
	return "127.0.0.1:8000"
}

func (ss *serverStub) Start() error {
	return nil
}

func (ss *serverStub) ShutDown() {
}

type commStub struct {
}

func (cs *commStub) Register(p pb.GossipServer) {
}

func (cs *commStub) CloseConn(addr string) {
}

func (cs *commStub) Addr() string {
	return "addr"
}

func (cs *commStub) Start() {
}

func (cs *commStub) Stop() {
}

func (cs *commStub) Gossip(addr string, m *pb.State) (*pb.StateResponse, error) {
	return &pb.StateResponse{}, nil
}

func (cs *commStub) Send(addr string, m *pb.Msg) (*pb.MsgResponse, error) {
	return &pb.MsgResponse{}, nil
}

type pingStub struct {
}

func (ps *pingStub) Pause(t time.Duration) {
}

func (ps *pingStub) Start() {
}

func (ps *pingStub) Stop() {
}

func (ps *pingStub) Ping(addr string, m *pb.Ping) (*pb.Pong, error) {
	return &pb.Pong{}, nil
}

//TODO we need to decide upon stubs or not stubs etc, not just copy stuff, this is really ugly
type cryptoStub struct {
	priv *ecdsa.PrivateKey
}

func (cs *cryptoStub) Verify(data, r, s []byte, pub *ecdsa.PublicKey) bool {
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

func (cs *cryptoStub) Sign(data []byte) ([]byte, []byte, error) {
	hash := hashContent(data)

	r, s, err := ecdsa.Sign(rand.Reader, cs.priv, hash)
	if err != nil {
		return nil, nil, err
	}

	return r.Bytes(), s.Bytes(), nil
}

type cmStub struct {
	cert *x509.Certificate
}

func (cm *cmStub) Certificate() *x509.Certificate {
	return cm.cert
}

func (cm *cmStub) CaCertificate() *x509.Certificate {
	return nil
}

func (cm *cmStub) ContactList() []*x509.Certificate {
	return nil
}

func (cm *cmStub) NumRings() uint32 {
	return 32
}

func (cm *cmStub) Trusted() bool {
	return false
}
