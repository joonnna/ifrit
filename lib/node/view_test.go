package node

import (
	"testing"

	"github.com/joonnna/firechain/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ViewTestSuite struct {
	suite.Suite
	*view
	*peer
}

func (suite *ViewTestSuite) SetupTest() {
	var numRings uint32

	numRings = 3

	logger := logger.CreateLogger("test", "testlog")

	suite.peer, _ = newTestPeer("test1234", numRings, "localhost:3123")

	suite.view = newView(numRings, logger, suite.peerId, suite.addr)
}

func TestViewTestSuite(t *testing.T) {
	suite.Run(t, new(ViewTestSuite))
}

func (suite *ViewTestSuite) TestAddSamePeerTwice() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	suite.addViewPeer(p)
	suite.addViewPeer(p)

	assert.NotNil(suite.T(), suite.getViewPeer(p.key), "Peer doesn't exist after double add")

	suite.addLivePeer(p)
	suite.addLivePeer(p)

	assert.NotNil(suite.T(), suite.getLivePeer(p.key), "Peer doesn't exist after double add")
}

func (suite *ViewTestSuite) TestAddRemoveLivePeer() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	suite.addLivePeer(p)

	assert.NotNil(suite.T(), suite.getLivePeer(p.key), "Add failed")

	suite.removeLivePeer(p.key)

	assert.Nil(suite.T(), suite.getLivePeer(p.key), "Remove failed")
}

func (suite *ViewTestSuite) TestGetLivePeers() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	suite.addLivePeer(p)

	p2, _ := newTestPeer("test12", suite.numRings, "localhost:124")

	suite.addLivePeer(p2)

	assert.Equal(suite.T(), len(suite.getLivePeers()), 2)
}

func (suite *ViewTestSuite) TestGetLivePeerAddrs() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	suite.addLivePeer(p)

	p2, _ := newTestPeer("test12", suite.numRings, "localhost:124")

	suite.addLivePeer(p2)

	assert.Equal(suite.T(), len(suite.getLivePeerAddrs()), 2)
}

func (suite *ViewTestSuite) TestStartValidTimer() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	suite.startTimer(p.key, p.getNote(), nil, p.addr)

	assert.True(suite.T(), suite.timerExist(p.key), "Valid timer was not started")
}

func (suite *ViewTestSuite) TestDeleteTimer() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	suite.startTimer(p.key, p.getNote(), nil, p.addr)

	suite.deleteTimeout(p.key)

	assert.False(suite.T(), suite.timerExist(p.key), "Timer was not deleted")
}

func (suite *ViewTestSuite) TestStartExistingTimer() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	suite.startTimer(p.key, p.getNote(), nil, p.addr)

	assert.True(suite.T(), suite.timerExist(p.key), "Valid timer was not started")

	t := suite.getTimeout(p.key)

	suite.startTimer(p.key, p.getNote(), nil, p.addr)

	t2 := suite.getTimeout(p.key)

	assert.True(suite.T(), t.timeStamp.Equal(t2.timeStamp), "New timeout was started when an existing one was already started")
}

func (suite *ViewTestSuite) TestGetAllTimeouts() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")
	p2, _ := newTestPeer("test12", suite.numRings, "localhost:1234")

	suite.startTimer(p.key, p.getNote(), nil, p.addr)
	suite.startTimer(p2.key, p2.getNote(), nil, p2.addr)

	assert.Equal(suite.T(), len(suite.getAllTimeouts()), 2, "Get timeouts does not retrieve all timeouts")
}

func (suite *ViewTestSuite) TestShouldBeNeighbours() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	suite.addLivePeer(p)

	assert.True(suite.T(), suite.shouldBeNeighbours(p.peerId), "Should be neighbours when only two in rings")
}

func (suite *ViewTestSuite) TestFindNeighboursWhenAlone() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	assert.Equal(suite.T(), len(suite.findNeighbours(p.peerId)), 1, "Should find 1 neighbours when only two peers in rings")
}

func (suite *ViewTestSuite) TestFindNeighboursWhenNoteAlone() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")
	p2, _ := newTestPeer("test12", suite.numRings, "localhost:1234")
	p3, _ := newTestPeer("test13", suite.numRings, "localhost:1235")
	p4, _ := newTestPeer("test14", suite.numRings, "localhost:1236")

	suite.addLivePeer(p)
	suite.addLivePeer(p2)
	suite.addLivePeer(p3)
	suite.addLivePeer(p4)

	assert.NotEqual(suite.T(), len(suite.findNeighbours(p.peerId)), 1, "Should find more than 1 neighbour with more populated rings")
	assert.NotEqual(suite.T(), len(suite.findNeighbours(p.peerId)), 0, "Should find more than 0 neighbour with more populated rings")
}

func (suite *ViewTestSuite) TestIsPrev() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	for num, _ := range suite.ringMap {
		assert.True(suite.T(), suite.isPrev(p, suite.peer, num), "Should be prev when only 2 peers present in rings")

	}
}

func (suite *ViewTestSuite) TestIsPrevWhenAlone() {
	for num, _ := range suite.ringMap {
		assert.True(suite.T(), suite.isPrev(suite.peer, suite.peer, num), "Should have myself as prev on all rings when alone")

	}
}

func (suite *ViewTestSuite) TestIsPrevOnInvalidRing() {
	p, _ := newTestPeer("test1", suite.numRings, "localhost:123")

	assert.False(suite.T(), suite.isPrev(p, suite.peer, 50), "Should fail when specifying invalid ringNum")
}

func (suite *ViewTestSuite) TestAddLivePeerRemovesInvalidAccusation() {
	p1, _ := newTestPeer("test1", suite.numRings, "localhost:123")
	p2, _ := newTestPeer("test12", suite.numRings, "localhost:1234")
	p3, _ := newTestPeer("test14", suite.numRings, "localhost:1235")
	p4, _ := newTestPeer("test15", suite.numRings, "localhost:1236")
	p5, _ := newTestPeer("test16", suite.numRings, "localhost:1237")
	p6, _ := newTestPeer("test17", suite.numRings, "localhost:1238")
	p7, _ := newTestPeer("test18", suite.numRings, "localhost:1239")

	suite.addViewPeer(p1)
	suite.addViewPeer(p2)
	suite.addViewPeer(p3)
	suite.addViewPeer(p4)
	suite.addViewPeer(p5)
	suite.addViewPeer(p6)
	suite.addViewPeer(p7)

	suite.addLivePeer(p1)
	suite.addLivePeer(p2)
	suite.addLivePeer(p3)
	suite.addLivePeer(p4)
	suite.addLivePeer(p5)
	suite.addLivePeer(p6)
	suite.addLivePeer(p7)

	view := suite.getView()

	for _, p := range view {
		for num, r := range suite.ringMap {
			key, err := r.getSucc(p.id)
			require.Nil(suite.T(), err, "Should not fail this call")
			if key == suite.key {
				continue
			}

			succ := suite.getViewPeer(key)
			require.NotNil(suite.T(), succ, "Should not fail this call")

			n := succ.getNote()
			//signature is not important now
			a := &accusation{
				peerId:  succ.peerId,
				accuser: p.peerId,
				mask:    n.mask,
				ringNum: num,
				epoch:   n.epoch,
			}
			err = succ.setAccusation(a)
			require.Nil(suite.T(), err, "Should not fail this call")
		}
	}

	//add new peer and assert that all accusations that should be invalidated are removed
	p8, _ := newTestPeer("test19", suite.numRings, "localhost:1239")
	suite.addViewPeer(p8)
	suite.addLivePeer(p8)

	for num, r := range suite.ringMap {
		succKey, _, err := r.findNeighbours(p8.peerId)
		if succKey == suite.key {
			continue
		}
		require.Nil(suite.T(), err, "Should note fail this call")

		succ := suite.getViewPeer(succKey)
		require.NotNil(suite.T(), succ, "Should note fail this call")

		assert.Nil(suite.T(), succ.getRingAccusation(num), "Accusation should have been invalidated")
	}

}
