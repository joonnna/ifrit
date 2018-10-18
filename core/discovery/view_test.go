package discovery

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"testing"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ViewTestSuite struct {
	suite.Suite

	v *View
}

func TestViewTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))
	//r.SetHandler(log.DiscardHandler())

	suite.Run(t, new(ViewTestSuite))
}

func (suite *ViewTestSuite) SetupTest() {
	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(suite.T(), err, "Failed to generate private key.")

	cert := validCert("selfId", privKey.Public())

	v, err := NewView(10, cert, &cmStub{}, &signerStub{})
	require.NoError(suite.T(), err, "Failed to create view.")

	suite.v = v
}

func (suite *ViewTestSuite) TestNewView() {
	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(suite.T(), err, "Failed to generate private key.")

	cert := validCert("selfId", privKey.Public())

	var numRings uint32 = 10

	expectedByz := uint32((float64(numRings) / 2.0) - 1)

	v, err := NewView(numRings, cert, &cmStub{}, &signerStub{})
	require.NoError(suite.T(), err, "Failed to create view.")
	require.Equal(suite.T(), expectedByz, v.maxByz, "Max byzantine not set correctly.")

	v2, err := NewView(2, cert, &cmStub{}, &signerStub{})
	require.NoError(suite.T(), err, "Failed to create view.")
	require.Equal(suite.T(), uint32(0), v2.maxByz, "Max byzantine should be zero with 2 rings.")

	v3, err := NewView(1, cert, &cmStub{}, &signerStub{})
	require.NoError(suite.T(), err, "Failed to create view.")
	require.Equal(suite.T(), uint32(0), v3.maxByz, "Max byzantine should be zero with 1 ring.")

	_, err = NewView(0, cert, &cmStub{}, &signerStub{})
	require.Error(suite.T(), err, "Should return error with 0 rings.")

	_, err = NewView(numRings, nil, &cmStub{}, &signerStub{})
	require.Error(suite.T(), err, "Should return error with no certificate.")

}

func (suite *ViewTestSuite) TestNumRings() {
	assert.Equal(suite.T(), suite.v.rings.numRings, suite.v.NumRings(), "Does not return the correct numrings.")
}

func (suite *ViewTestSuite) TestSelf() {
	assert.Equal(suite.T(), suite.v.self, suite.v.Self(), "Does not return the correct self instance.")
}

func (suite *ViewTestSuite) TestPeer() {
	p := &Peer{
		Id: "testId",
	}

	suite.v.viewMap[p.Id] = p

	assert.Equal(suite.T(), p, suite.v.Peer(p.Id), "Did not find correct peer.")

	assert.Nil(suite.T(), suite.v.Peer("non-existing-id"), "Non-exisiting id should return nil.")

}

func (suite *ViewTestSuite) TestFull() {
	emptyView := suite.v.Full()

	assert.NotNil(suite.T(), emptyView, "Empty view should not be nil.")

	assert.Zero(suite.T(), len(emptyView), "Should be zero with empty view.")

	for i := 0; i < 10; i++ {
		p := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		suite.v.viewMap[p.Id] = p

		view := suite.v.Full()

		require.NotNil(suite.T(), view, "Should not ever be nil.")
		require.Equal(suite.T(), i+1, len(view), "Does not return entire view.")
	}
}

func (suite *ViewTestSuite) TestExists() {
	assert.False(suite.T(), suite.v.Exists("non-existing-id"), "Non-exisiting id should return false.")

	p := &Peer{
		Id: "TestId",
	}

	suite.v.viewMap[p.Id] = p

	assert.True(suite.T(), suite.v.Exists(p.Id), "Should return true on existing id.")
}

func (suite *ViewTestSuite) TestLive() {
	emptyView := suite.v.Live()

	assert.NotNil(suite.T(), emptyView, "Empty view should not be nil.")

	assert.Zero(suite.T(), len(emptyView), "Should be zero with empty view.")

	for i := 0; i < 10; i++ {
		p := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		suite.v.liveMap[p.Id] = p

		view := suite.v.Live()

		require.NotNil(suite.T(), view, "Should not ever be nil.")
		require.Equal(suite.T(), i+1, len(view), "Does not return entire view.")
	}
}

func (suite *ViewTestSuite) TestAddFull() {
	p := &Peer{
		Id: "TestId",
	}

	suite.v.viewMap[p.Id] = p

	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(suite.T(), err, "Failed to generate private key.")

	certId := "selfId"
	cert := validCert(certId, privKey.Public())

	assert.EqualError(suite.T(), suite.v.AddFull(p.Id, cert), errPeerAlreadyExists.Error(), "Should return error when adding duplicate peer.")

	assert.Error(suite.T(), suite.v.AddFull(certId, nil), "Should return error with nil certr.")

	assert.NoError(suite.T(), suite.v.AddFull(certId, cert), "Should not return error with valid parameters.")
	assert.True(suite.T(), suite.v.Exists(certId), "Peer not added to map.")
}

func (suite *ViewTestSuite) TestMyNeighbours() {
	neighbours := suite.v.MyNeighbours()
	require.NotNil(suite.T(), neighbours, "Returned nil slice.")
	require.Zero(suite.T(), len(neighbours), "Should return none when only myself in view.")

	for i := 0; i < 10; i++ {
		p := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		suite.v.AddLive(p)
	}

	neighbours = suite.v.MyNeighbours()
	require.NotNil(suite.T(), neighbours, "Returned nil slice.")
	require.True(suite.T(), len(neighbours) >= 2, "Should always have 2 or more neighbours.")

	for _, peer := range neighbours {
		require.NotEqual(suite.T(), suite.v.self, peer, "Should not return self as neighbour with peers present in the live view.")
	}
}

func (suite *ViewTestSuite) TestGossipPartners() {
	var j uint32

	view := suite.v

	// Test behaviour when alone, should never return myself.

	require.Equal(suite.T(), uint32(1), view.currGossipRing, "Should start with gossip partners on ring 1.")
	for j = 0; j < view.rings.numRings; j++ {
		prev := view.currGossipRing

		partners := view.GossipPartners()
		require.NotNil(suite.T(), partners, "Should never return nil slice.")
		require.Zero(suite.T(), len(partners), "Should be empty when only myself in view.")

		if j == view.rings.numRings-1 {
			require.Equal(suite.T(), uint32(1), view.currGossipRing, "Gossip ring not reset to 1 upon reaching numRings.")
		} else {
			require.Equal(suite.T(), uint32(prev+1), view.currGossipRing, "Gossip ring not incremented.")
		}

	}

	for i := 0; i < 10; i++ {
		p := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		view.AddLive(p)
	}

	require.Equal(suite.T(), uint32(1), view.currGossipRing, "Should start with gossip partners on ring 1.")
	for j = 0; j < view.rings.numRings; j++ {
		prev := view.currGossipRing

		partners := view.GossipPartners()
		require.NotNil(suite.T(), partners, "Should never return nil slice.")
		require.Equal(suite.T(), 2, len(partners), "Should always have 2 neigbours per ring.")

		if j == view.rings.numRings-1 {
			require.Equal(suite.T(), uint32(1), view.currGossipRing, "Gossip ring not reset to 1 upon reaching numRings.")
		} else {
			require.Equal(suite.T(), uint32(prev+1), view.currGossipRing, "Gossip ring not incremented.")
		}

	}
}

func (suite *ViewTestSuite) TestMonitorTarget() {
	var j uint32

	view := suite.v

	// Test behaviour when alone, should never return myself.

	require.Equal(suite.T(), uint32(1), view.currMonitorRing, "Should start monitoring on ring 1.")
	for j = 0; j < view.rings.numRings; j++ {
		prev := view.currMonitorRing

		target, ringNum := view.MonitorTarget()
		require.Nil(suite.T(), target, "Should return nil when alone, the target was myself.")
		require.Equal(suite.T(), j+1, ringNum, "Should return ringNumber even if target was nil.")

		if j == view.rings.numRings-1 {
			require.Equal(suite.T(), uint32(1), view.currMonitorRing, "Monitor ring not reset to 1 upon reaching numRings.")
		} else {
			require.Equal(suite.T(), uint32(prev+1), view.currMonitorRing, "Monitor ring not incremented.")
		}

	}

	for i := 0; i < 10; i++ {
		p := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		view.AddLive(p)
	}

	require.Equal(suite.T(), uint32(1), view.currMonitorRing, "Should start monitoring on ring 1.")
	for j = 0; j < view.rings.numRings; j++ {
		prev := view.currMonitorRing

		target, ringNum := view.MonitorTarget()
		require.NotNil(suite.T(), target, "Should never return nil with peers present in live view.")
		require.NotEqual(suite.T(), view.self, target, "Should never return myself as monitor target.")
		require.Equal(suite.T(), j+1, ringNum, "Returned ringNumber does not match current ring.")

		if j == view.rings.numRings-1 {
			require.Equal(suite.T(), uint32(1), view.currMonitorRing, "Monitor ring not reset to 1 upon reaching numRings.")
		} else {
			require.Equal(suite.T(), uint32(prev+1), view.currMonitorRing, "Monitor ring not incremented.")
		}
	}
}

func (suite *ViewTestSuite) TestAddLive() {
	view := suite.v

	p := &Peer{
		Id: "testId",
	}

	view.AddLive(p)

	assert.Equal(suite.T(), 1, len(view.liveMap), "Peer not added to liveMap.")

	_, ok := view.liveMap[p.Id]
	assert.True(suite.T(), ok, "Peer does is not present in liveMap.")

	view.AddLive(p)
	assert.Equal(suite.T(), 1, len(view.liveMap), "Adding peer twice does not alter state.")

	_, ok = view.liveMap[p.Id]
	assert.True(suite.T(), ok, "Adding peer twice does not alter state.")
}

func (suite *ViewTestSuite) TestRingNeighbours() {
	var i uint32

	view := suite.v

	for i = 1; i <= view.rings.numRings; i++ {
		succ, prev := view.MyRingNeighbours(i)
		require.Nil(suite.T(), succ, "Should return nil successor with empty view.")
		require.Nil(suite.T(), prev, "Should return nil predecessor with empty view.")
	}

	for j := 0; j < 10; j++ {
		p := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		view.AddLive(p)
	}

	for i = 1; i <= view.rings.numRings; i++ {
		succ, prev := view.MyRingNeighbours(i)
		require.NotNil(suite.T(), succ, "Should never return nil successor with non-empty view.")
		require.NotNil(suite.T(), prev, "Should never return nil predecessor with non-empty view.")
		require.NotEqual(suite.T(), succ, view.self, "Should never return self as neighbour.")
		require.NotEqual(suite.T(), prev, view.self, "Should never return self as neighbour.")
	}

}

func (suite *ViewTestSuite) TestLivePeer() {
	view := suite.v

	assert.Nil(suite.T(), view.LivePeer("non-existing-id"), "Should return nil with non-exisiting id.")

	p := &Peer{
		Id: "testId",
	}

	view.liveMap[p.Id] = p

	assert.Equal(suite.T(), p, view.LivePeer(p.Id), "Returned incorrect peer of existing id.")
}

func (suite *ViewTestSuite) TestRemoveLive() {
	view := suite.v

	p := &Peer{
		Id: "testId",
	}

	view.liveMap[p.Id] = p

	view.RemoveLive(p.Id)

	assert.Zero(suite.T(), len(view.liveMap), "Peer not removed from liveMap.")

	_, ok := view.liveMap[p.Id]
	assert.False(suite.T(), ok, "Peer still in liveMap after being removed.")

	view.RemoveLive(p.Id)
	assert.Zero(suite.T(), len(view.liveMap), "Removing peer twice alters state.")

	_, ok = view.liveMap[p.Id]
	assert.False(suite.T(), ok, "Removing peer twice alters state.")

	nonExistingId := "does-not-exist"

	view.RemoveLive(nonExistingId)
	assert.Zero(suite.T(), len(view.liveMap), "Removing non-existing id alters state.")

	_, ok = view.liveMap[nonExistingId]
	assert.False(suite.T(), ok, "Removing non-existing id alters state.")
}

func (suite *ViewTestSuite) TestStartTimer() {
	view := suite.v

	accused := &Peer{
		Id: "testId",
	}

	accuser := &Peer{
		Id: "testId2",
	}

	note := &Note{
		id: accused.Id,
	}

	assert.EqualError(suite.T(), view.StartTimer(nil, note, accuser), errAccusedIsNil.Error(), "Should return error when accused is nil.")

	assert.EqualError(suite.T(), view.StartTimer(accused, nil, accuser), errNoNote.Error(), "Should return error when note is nil.")

	assert.EqualError(suite.T(), view.StartTimer(accused, note, nil), errObsIsNil.Error(), "Should return error when accuser is nil.")

	wrongNote := &Note{
		id: "non-existing-id",
	}

	assert.EqualError(suite.T(), view.StartTimer(accused, wrongNote, accuser), errWrongNote.Error(), "Should return error when the note does not belong to the accused.")

	assert.NoError(suite.T(), view.StartTimer(accused, note, accuser), "Failed to start timer with valid parameters.")

	prevTimeStamp, ok := view.timeoutMap[accused.Id]
	require.True(suite.T(), ok, "Timestamp not added.")

	assert.NoError(suite.T(), view.StartTimer(accused, note, accuser), "Starting same timer twice should not return error.")

	newTimeStamp, ok := view.timeoutMap[accused.Id]
	require.True(suite.T(), ok, "Timestamp not added.")

	assert.Equal(suite.T(), prevTimeStamp, newTimeStamp, "Should not start a new timer when starting the same timer twice.")

	// Clean before testing multiple timeouts.
	view.timeoutMap = make(map[string]*timeout)

	for i := 0; i < 100; i++ {
		accused := &Peer{
			Id: fmt.Sprintf("testAccused%d", i),
		}

		accuser := &Peer{
			Id: fmt.Sprintf("testAccuser%d", i),
		}

		note := &Note{
			id: accused.Id,
		}

		require.NoError(suite.T(), view.StartTimer(accused, note, accuser), "Should not return error when starting timer with correct parameters.")
		require.Equal(suite.T(), i+1, len(view.timeoutMap), "Timeouts not added correctly.")
	}
}

func (suite *ViewTestSuite) TestHasTimer() {
	view := suite.v

	assert.False(suite.T(), view.HasTimer("non-existing-id"), "Should return false when there exists no timeout for the given id.")

	t := &timeout{
		accused: &Peer{
			Id: "testId",
		},
	}

	view.timeoutMap[t.accused.Id] = t

	assert.True(suite.T(), view.HasTimer(t.accused.Id), "Should return true when there exists a timeout for the given id.")
}

func (suite *ViewTestSuite) TestDeleteTimeout() {
	view := suite.v

	t := &timeout{
		accused: &Peer{
			Id: "testId",
		},
	}

	view.timeoutMap[t.accused.Id] = t

	view.DeleteTimeout(t.accused.Id)

	_, ok := view.timeoutMap[t.accused.Id]
	assert.False(suite.T(), ok, "Timeout not removed.")
	assert.Zero(suite.T(), len(view.timeoutMap), "Map still has entries.")

	view.DeleteTimeout("non-existing-id")
	assert.Zero(suite.T(), len(view.timeoutMap), "Deleting non-existing id changes state.")
}

func (suite *ViewTestSuite) TestAllTimeouts() {
	view := suite.v

	timeouts := view.allTimeouts()
	require.NotNil(suite.T(), timeouts, "Should never return nil, even if there are no timeouts.")
	assert.Zero(suite.T(), len(timeouts), "Should be zero when there are no timeouts.")

	for i := 0; i < 10; i++ {
		t := &timeout{
			accused: &Peer{
				Id: fmt.Sprintf("testId%d", i),
			},
		}

		view.timeoutMap[t.accused.Id] = t

		ts := view.allTimeouts()
		require.NotNil(suite.T(), ts, "Should never be nil.")
		require.Equal(suite.T(), i+1, len(ts), "Timeouts not added correctly.")
	}
}

func (suite *ViewTestSuite) TestCheckTimeouts() {
	view := suite.v

	view.checkTimeouts()
	assert.Zero(suite.T(), len(view.timeoutMap), "Does not alter state when there are no timeouts.")

	accused := &Peer{
		Id: ("testAccused"),
	}

	accuser := &Peer{
		Id: "testAccuser",
	}

	note := &Note{
		id: accused.Id,
	}

	// 100.0 seconds
	view.removalTimeout = 100.0

	t := &timeout{
		accused:   accused,
		observer:  accuser,
		lastNote:  note,
		timeStamp: time.Now(),
	}

	view.timeoutMap[accused.Id] = t

	view.checkTimeouts()
	require.Equal(suite.T(), 1, len(view.timeoutMap), "Timeout removed before expiration.")

	_, ok := view.timeoutMap[accused.Id]
	require.True(suite.T(), ok, "Timeout removed from map before expiration.")

	// 10 years in the past
	t.timeStamp = t.timeStamp.AddDate(-10, 0, 0)

	view.checkTimeouts()
	require.Zero(suite.T(), len(view.timeoutMap), "Timeout not removed after expiration.")

	_, ok = view.timeoutMap[accused.Id]
	require.False(suite.T(), ok, "Timeout not removed from map after expiration.")
}

func (suite *ViewTestSuite) TestShouldRebuttal() {
	view := suite.v

	currentEpoch := view.self.note.epoch

	var accuseRing uint32 = 1

	assert.False(suite.T(), view.ShouldRebuttal(currentEpoch+1, accuseRing), "Should return false with invalid epoch.")

	assert.False(suite.T(), view.ShouldRebuttal(currentEpoch-1, accuseRing), "Should return false with invalid epoch.")

	prevNote := view.self.note

	assert.True(suite.T(), view.ShouldRebuttal(currentEpoch, accuseRing), "Should return false with invalid epoch.")

	assert.NotEqual(suite.T(), prevNote, view.self.note, "Old note not replaced.")
	assert.True(suite.T(), view.self.note.IsRingDisabled(accuseRing, view.NumRings()),
		"Ring not deactivated in mask.")
	assert.True(suite.T(), view.ValidMask(view.self.note.mask), "Mask is not valid after disabling.")
}

func (suite *ViewTestSuite) TestShouldBeNeighbour() {
	view := suite.v

	assert.True(suite.T(), view.ShouldBeNeighbour("testId"), "Should be neighbours with everyone when alone.")
}

func (suite *ViewTestSuite) TestFindNeighbours() {
	view := suite.v

	neighbours := view.FindNeighbours("testId")
	require.NotNil(suite.T(), neighbours, "Should never return nil.")
	require.Equal(suite.T(), 1, len(neighbours), "Should only return myself when alone.")
	assert.Equal(suite.T(), view.self, neighbours[0], "Did not return myself as neighbour.")

	for i := 0; i < 10; i++ {
		p := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		view.AddLive(p)

		n := view.FindNeighbours(p.Id)
		require.NotNil(suite.T(), n, "Should never return nil.")

		if i == 0 {
			require.Equal(suite.T(), 1, len(n), "Should only return myself when we are 2 in live view.")
		} else {
			require.True(suite.T(), len(n) >= 2, "Should always have 2 or more neighbours when there are more than 2 in the live view.")
		}
	}
}

func (suite *ViewTestSuite) TestValidAccuser() {
	view := suite.v

	p := &Peer{
		Id: "testId",
	}

	view.AddLive(p)

	assert.False(suite.T(), view.ValidAccuser(p, view.self, view.rings.numRings+1),
		"Should return false with invalid ring number.")

	assert.True(suite.T(), view.ValidAccuser(p, view.self, view.rings.numRings),
		"Should return true with valid parameters.")

}

func (suite *ViewTestSuite) TestIsAlive() {
	view := suite.v

	assert.False(suite.T(), view.IsAlive("non-existing-id"), "Non-existing id should return false.")

	p := &Peer{
		Id: "testId",
	}

	view.liveMap[p.Id] = p

	assert.True(suite.T(), view.IsAlive(p.Id), "Id of live peer should return true.")
}

func (suite *ViewTestSuite) TestIncrementGossipRing() {
	var i uint32
	view := suite.v

	require.Equal(suite.T(), uint32(1), view.currGossipRing, "Should start at 1.")

	for i = 1; i <= view.rings.numRings; i++ {
		view.incrementGossipRing()

		if i == view.rings.numRings {
			require.Equal(suite.T(), uint32(1), view.currGossipRing, "Wrap around not working.")
		} else {
			require.Equal(suite.T(), i+1, view.currGossipRing, "Gossip ring not incremented.")
		}
	}
}

func (suite *ViewTestSuite) TestIncrementMonitorRing() {
	var i uint32
	view := suite.v

	require.Equal(suite.T(), uint32(1), view.currMonitorRing, "Should start at 1.")

	for i = 1; i <= view.rings.numRings; i++ {
		view.incrementMonitorRing()

		if i == view.rings.numRings {
			require.Equal(suite.T(), uint32(1), view.currMonitorRing, "Wrap around not working.")
		} else {
			require.Equal(suite.T(), i+1, view.currMonitorRing,
				"Monitor ring not incremented.")
		}
	}
}

func (suite *ViewTestSuite) TestSelfNote() {
	view := suite.v

	localNote := view.self.note

	note := view.selfNote()
	require.NotNil(suite.T(), note, "Should never be nil.")

	assert.Equal(suite.T(), localNote.epoch, note.GetEpoch(), "Not equal epochs.")

	assert.Equal(suite.T(), localNote.epoch, note.GetEpoch(), "Not equal epochs.")
	assert.Equal(suite.T(), localNote.id, string(note.GetId()), "Not equal ids.")
	assert.Equal(suite.T(), localNote.mask, note.GetMask(), "Not equal masks.")
	assert.Equal(suite.T(), localNote.signature.r, note.GetSignature().GetR(), "Not equal signature component r.")
	assert.Equal(suite.T(), localNote.signature.s, note.GetSignature().GetS(), "Not equal signature component s.")
}

func (suite *ViewTestSuite) TestState() {
	view := suite.v

	emptyState := view.State()
	require.NotNil(suite.T(), emptyState, "Should never be nil.")

	hosts := emptyState.GetExistingHosts()
	note := emptyState.GetOwnNote()

	require.NotNil(suite.T(), hosts, "Should never be nil.")
	require.NotNil(suite.T(), note, "Should never be nil.")

	require.Zero(suite.T(), len(hosts), "Should be no hosts when alone.")

	for i := 0; i < 20; i++ {
		p := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		hasNote := false

		if i%2 == 0 {
			hasNote = true
			p.note = &Note{
				id:    p.Id,
				epoch: 1,
			}
		}

		view.viewMap[p.Id] = p

		s := view.State()

		hosts := s.GetExistingHosts()
		note := s.GetOwnNote()
		require.NotNil(suite.T(), hosts, "Should never be nil.")
		require.NotNil(suite.T(), note, "Should never be nil.")

		require.Equal(suite.T(), i+1, len(hosts), "Peers not added to state message.")

		epoch, ok := hosts[p.Id]
		require.True(suite.T(), ok, "Peer not added to state message.")

		if hasNote {
			require.Equal(suite.T(), uint64(1), epoch, "Wrong epoch added.")
		} else {
			require.Zero(suite.T(), epoch, "Epoch not set to zero with no note.")
		}
	}
}

func (suite *ViewTestSuite) TestValidMask() {
	view := suite.v

	assert.True(suite.T(), view.ValidMask(view.self.note.mask), "Returned false with valid mask.")

	assert.False(suite.T(), view.ValidMask(view.self.note.mask+1), "Returned true with invalid mask.")
}

func (suite *ViewTestSuite) TestIsRingDisabled() {
	var i, disableIdx uint32

	view := suite.v

	for i = 1; i <= view.rings.numRings; i++ {
		require.False(suite.T(), view.self.note.IsRingDisabled(i, view.NumRings()),
			"Returned true on an active ring.")
	}

	disableIdx = 0

	view.self.note.mask = clearBit(view.self.note.mask, disableIdx)

	assert.True(suite.T(), view.self.note.IsRingDisabled(disableIdx+1, view.NumRings()), "Returned false on a disabled ring.")
}

func (suite *ViewTestSuite) TestDeactivateRing() {
	var i, j, deactivated, ringNum uint32

	view := suite.v

	_, err := view.deactivateRing(view.rings.numRings + 1)
	require.EqualError(suite.T(), err, errNonExistingRing.Error(), "Should fail with invalid ring number.")

	ringNum = 1
	view.self.note.mask = clearBit(view.self.note.mask, ringNum)
	_, err = view.deactivateRing(ringNum + 1)
	require.EqualError(suite.T(), err, errAlreadyDeactivated.Error(), "Should fail when deactivating an already deactivated ring.")

	require.Equal(suite.T(), uint32(0), view.deactivatedRings, "Should start at 0.")
	view.self.note.mask = setBit(view.self.note.mask, ringNum)

	mask := view.self.note.mask
	for i = 1; i <= view.rings.numRings; i++ {
		mask, err = view.deactivateRing(i)
		require.NoError(suite.T(), err, "Valid disables should not fail.")

		if i > view.maxByz {
			require.Equal(suite.T(), view.maxByz, view.deactivatedRings, "Deactivated rings not equal to maxbyz.")
			require.False(suite.T(), hasBit(mask, i-1), "Bit not deactivated.")

			deactivated = 0
			for j = 0; j < view.rings.numRings; j++ {
				if !hasBit(mask, j) {
					deactivated++
				}
			}

			require.Equal(suite.T(), view.deactivatedRings, deactivated, "Deactivated counter does not reflect actual value.")
			require.True(suite.T(), deactivated <= view.maxByz, "Deactivated too many rings.")

		} else {
			require.Equal(suite.T(), i, view.deactivatedRings, "Deactivated rings not incremented.")
			require.False(suite.T(), hasBit(mask, i-1), "Bit not deactivated.")
		}
		view.self.note.mask = mask
	}

	// Reset before maxByz zero test
	for i = 0; i < view.rings.numRings; i++ {
		view.self.note.mask = setBit(view.self.note.mask, i)
	}
	view.deactivatedRings = 0

	view.maxByz = 0

	_, err = view.deactivateRing(0)
	assert.EqualError(suite.T(), err, errZeroDeactivate.Error(), "Should fail with maxbyz set to zero.")
}

func (suite *ViewTestSuite) TestValidMaskInternal() {
	var i uint32

	mask := suite.v.self.note.mask
	numRings := suite.v.rings.numRings
	maxByz := suite.v.maxByz

	assert.NoError(suite.T(), validMask(mask, numRings, maxByz),
		"Should not fail with valid mask.")
	assert.Error(suite.T(), validMask(0, numRings, maxByz),
		"Should always fail with mask set to zero.")

	for i = 0; i < numRings; i++ {
		mask = clearBit(mask, i)
		if i >= maxByz {
			require.Error(suite.T(), validMask(mask, numRings, maxByz),
				"Should fail with too many deactivations.")
		} else {
			require.NoError(suite.T(), validMask(mask, numRings, maxByz),
				"Should not fail with valid amounts of deactivations.")
		}
	}
}

func (suite *ViewTestSuite) TestSetBit() {
	var mask, i uint32

	for i = 0; i < 32; i++ {
		mask = setBit(mask, i)
		require.True(suite.T(), hasBit(mask, i), "Should return true on set bit.")
	}

}

func (suite *ViewTestSuite) TestClearBit() {
	var i uint32

	mask := suite.v.self.note.mask

	for i = 0; i < 32; i++ {
		mask = clearBit(mask, i)
		require.False(suite.T(), hasBit(mask, i), "Should return false on cleared bit.")
	}
}

func (suite *ViewTestSuite) TestHasBit() {
	var mask, i uint32

	for i = 0; i < 32; i++ {
		mask = setBit(mask, i)
		require.True(suite.T(), hasBit(mask, i), "Should return true on set bit.")
	}
}

func validCert(id string, pub crypto.PublicKey) *x509.Certificate {
	return &x509.Certificate{
		SubjectKeyId: []byte(id),
		Subject: pkix.Name{
			Locality: []string{"rpcAddr", "pingAddr", "httpAddr"},
		},
		PublicKey: pub,
	}
}

type cmStub struct {
}

func (cm *cmStub) CloseConn(addr string) {

}

type signerStub struct {
}

func (s *signerStub) Sign(data []byte) ([]byte, []byte, error) {
	return nil, nil, nil
}
