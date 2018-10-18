package discovery

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"os"
	"testing"

	log "github.com/inconshreveable/log15"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RingsTestSuite struct {
	suite.Suite

	*rings
}

type RingTestSuite struct {
	suite.Suite

	*ring
}

func TestRingTestSuite(t *testing.T) {
	r := log.Root()

	//r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	r.SetHandler(log.DiscardHandler())
	suite.Run(t, new(RingTestSuite))
}

func TestRingsTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	suite.Run(t, new(RingsTestSuite))
}

func (suite *RingTestSuite) SetupTest() {
	var ringNum uint32 = 1

	self := &Peer{
		Id: "selfPeer",
	}

	suite.ring = newRing(ringNum, self)
}

func (suite *RingsTestSuite) SetupTest() {
	var numRings uint32 = 5

	self := &Peer{
		Id: "selfPeer",
	}

	rs, err := createRings(self, numRings)
	require.NoError(suite.T(), err, "Could not create rings.")

	for _, r := range rs.ringMap {
		require.Equal(suite.T(), 1, len(r.succList), "Self peer not in rings.")
	}

	suite.rings = rs
}

func (suite *RingsTestSuite) TestCompare() {
	id := &ringId{
		hash: []byte("testId"),
	}

	lower := &ringId{
		hash: genLowerId(id.hash, 1),
	}

	higher := &ringId{
		hash: genHigherId(id.hash, 1),
	}

	assert.Equal(suite.T(), id.compare(lower), 1, "Lower id should return 1.")
	assert.Equal(suite.T(), id.compare(higher), -1, "Lower id should return -1.")
	assert.Equal(suite.T(), id.compare(id), 0, "equal id should return 0.")
}

func (suite *RingsTestSuite) TestEqual() {
	id := &ringId{
		hash: []byte("testId"),
	}

	lower := &ringId{
		hash: genLowerId(id.hash, 1),
	}

	assert.True(suite.T(), id.equal(id), "Returns false on equal ids.")
	assert.False(suite.T(), id.equal(lower), "Returns true on different ids.")
}

func (suite *RingsTestSuite) TestCreateRings() {
	var numRings uint32 = 5

	p := &Peer{
		Id: "testid",
	}

	rings, err := createRings(p, numRings)
	require.NoError(suite.T(), err, "Returned error on valid create.")
	assert.Equal(suite.T(), numRings, rings.numRings)
	require.NotNil(suite.T(), rings.self, "Self not stored in rings.")
	assert.Equal(suite.T(), p, rings.self, "Self self not correct address.")
	assert.Equal(suite.T(), int(numRings), len(rings.ringMap), "Did not create the specified amount of rings.")

	_, err = createRings(nil, numRings)
	assert.EqualError(suite.T(), err, errNoSelf.Error(), "Returned no error with self set to nil.")

	p2 := &Peer{}
	_, err = createRings(p2, numRings)
	assert.EqualError(suite.T(), err, errNoSelf.Error(), "Returned no error with self id set to empty string.")

	_, err = createRings(p, 0)
	assert.EqualError(suite.T(), err, errNoRings.Error(), "Returned no error with numRings set to 0.")
}

func (suite *RingsTestSuite) TestAdd() {
	p := &Peer{
		Id: "testId",
	}

	suite.rings.add(p)

	for _, r := range suite.rings.ringMap {
		require.Equal(suite.T(), 2, len(r.succList), "Peer not added to ring.")
	}

	suite.rings.add(p)

	for _, r := range suite.rings.ringMap {
		require.Equal(suite.T(), 2, len(r.succList), "Should not be able to add peer twice.")
	}

	suite.rings.add(suite.rings.self)

	for _, r := range suite.rings.ringMap {
		require.Equal(suite.T(), 2, len(r.succList), "Should not be able to add self.")
	}

}

func (suite *RingsTestSuite) TestRemove() {
	p := &Peer{
		Id: "testId",
	}

	suite.rings.add(p)

	for _, r := range suite.rings.ringMap {
		require.Equal(suite.T(), 2, len(r.succList), "Peer not added to ring.")
	}

	suite.rings.remove(p)

	for _, r := range suite.rings.ringMap {
		require.Equal(suite.T(), 1, len(r.succList), "Peer not removed from rings.")
	}

	suite.rings.remove(p)

	for _, r := range suite.rings.ringMap {
		require.Equal(suite.T(), 1, len(r.succList), "Removing peer twice alters ring state.")
	}

	suite.rings.remove(suite.rings.self)

	for _, r := range suite.rings.ringMap {
		require.Equal(suite.T(), 1, len(r.succList), "Removed self.")
	}

}

func (suite *RingsTestSuite) TestIsPredecessor() {
	p := &Peer{
		Id: "testId",
	}

	suite.rings.add(p)

	for _, r := range suite.rings.ringMap {
		require.True(suite.T(), suite.rings.isPredecessor(suite.rings.self, p, r.ringNum), "Should be predecessor with only 2 in rings.")
	}

	assert.False(suite.T(), suite.rings.isPredecessor(suite.rings.self, p, suite.rings.numRings+1), "Should return false with invalid ringNum.")

}

func (suite *RingsTestSuite) TestFindNeighbours() {
	p := &Peer{
		Id: "testId",
	}

	suite.rings.add(p)

	neighbours := suite.rings.findNeighbours(p.Id)

	assert.Equal(suite.T(), 1, len(neighbours), "Only 2 in rings, should only have 1 unique neighbour.")

	for i := 0; i < 10; i++ {
		peer := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		suite.rings.add(peer)

		neighbours := suite.rings.findNeighbours(peer.Id)
		assert.NotZero(suite.T(), len(neighbours), "Should always have neighbours.")

		exists := make(map[string]bool)
		for _, n := range neighbours {
			_, ok := exists[n.Id]
			require.False(suite.T(), ok, "All neighbours should be unique and not repeated.")
			exists[n.Id] = true
		}
	}
}

func (suite *RingsTestSuite) TestAllMyNeighbours() {
	p := &Peer{
		Id: "testId",
	}

	suite.rings.add(p)

	neighbours := suite.rings.allMyNeighbours()

	assert.Equal(suite.T(), 1, len(neighbours), "Only 2 in rings, should only have 1 unique neighbour.")

	for i := 0; i < 10; i++ {
		peer := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		suite.rings.add(peer)

		neighbours := suite.rings.allMyNeighbours()
		assert.NotZero(suite.T(), len(neighbours), "Should always have neighbours.")

		exists := make(map[string]bool)
		for _, n := range neighbours {
			_, ok := exists[n.Id]
			require.False(suite.T(), ok, "All neighbours should be unique and not repeated.")
			exists[n.Id] = true
		}
	}
}

func (suite *RingsTestSuite) TestMyRingNeighbours() {
	p := &Peer{
		Id: "testId",
	}

	suite.rings.add(p)

	neighbours := suite.rings.myRingNeighbours(suite.rings.numRings)

	require.Equal(suite.T(), 1, len(neighbours), "Failed to return only unique neighbours.")
	assert.Equal(suite.T(), neighbours[0].Id, p.Id, "Returned wrong neighbour.")

	neighbours2 := suite.rings.myRingNeighbours(suite.rings.numRings + 1)
	assert.Nil(suite.T(), neighbours2, "Should return nil with invalid ring number.")

	for i := 0; i < 10; i++ {
		peer := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		suite.rings.add(peer)
	}

	for _, r := range suite.rings.ringMap {
		ringNeighbours := suite.rings.myRingNeighbours(r.ringNum)
		succ := r.successor().p
		prev := r.predecessor().p

		for _, n := range ringNeighbours {
			if succ.Id == n.Id {
				assert.Equal(suite.T(), n, succ, "Retrieved wrong neighbour.")
			} else {
				assert.Equal(suite.T(), n, prev, "Retrieved wrong neighbour.")
			}
		}

	}

}

func (suite *RingsTestSuite) TestMyRingSuccessor() {
	p := &Peer{
		Id: "testId",
	}

	suite.rings.add(p)

	myRingSucc := suite.rings.myRingSuccessor(suite.rings.numRings)
	succ := suite.rings.ringMap[suite.rings.numRings].successor().p

	assert.Equal(suite.T(), succ.Id, myRingSucc.Id, "Retreieved wrong successor.")

	assert.Nil(suite.T(), suite.rings.myRingSuccessor(suite.rings.numRings+1), "Should return nil with invalid ring number.")
}

func (suite *RingsTestSuite) TestMyRingPredecessor() {
	p := &Peer{
		Id: "testId",
	}

	suite.rings.add(p)

	myRingPrev := suite.rings.myRingPredecessor(suite.rings.numRings)
	prev := suite.rings.ringMap[suite.rings.numRings].predecessor().p

	assert.Equal(suite.T(), prev.Id, myRingPrev.Id, "Retreieved wrong predecessor.")

	assert.Nil(suite.T(), suite.rings.myRingPredecessor(suite.rings.numRings+1), "Should return nil with invalid ring number.")
}

func (suite *RingsTestSuite) TestShouldBeMyNeighbour() {
	p := &Peer{
		Id: "testId",
	}

	suite.rings.add(p)

	assert.True(suite.T(), suite.rings.shouldBeMyNeighbour(p.Id), "Should be neighbour with the only existing peer.")
}

func (suite *RingsTestSuite) TestNewRing() {
	var ringNum uint32 = 1

	self := &Peer{
		Id: "selfId",
	}

	r := newRing(ringNum, self)

	assert.Zero(suite.T(), r.selfIdx, "Selfidx should start at 0.")
	assert.Equal(suite.T(), ringNum, r.ringNum, "Incorrect ringNumber.")
	require.NotNil(suite.T(), r.succList, "Successor list not allocated..")
	require.NotNil(suite.T(), r.peerToRing, "Peer to ring map not allocated.")
	assert.Equal(suite.T(), self, r.selfId.p, "Self is incorrect.")

	require.Equal(suite.T(), 1, len(r.succList), "Self not added to successor list.")
	require.Equal(suite.T(), len(r.succList), int(r.length), "Self not added to successor list.")
	assert.Equal(suite.T(), r.succList[0].p, self, "Self not added to internal ringId.")

	assert.Equal(suite.T(), 1, len(r.peerToRing), "Self not added to peerToRing map.")
}

func (suite *RingTestSuite) TestRingAdd() {
	found := false

	p := &Peer{
		Id: "testId",
	}

	r := suite.ring

	r.add(p)

	assert.Equal(suite.T(), 2, len(r.succList), "Peer not added to successor list.")
	assert.Equal(suite.T(), 2, r.length, "Peer not added to successor list.")
	assert.Equal(suite.T(), 2, len(r.peerToRing), "Peer not added to peerToRing map.")

	for idx, rId := range r.succList {
		if rId.p.Id == p.Id {
			require.Equal(suite.T(), rId.p, p, "Wrong peer representation stored.")
			found = true
			if idx == 0 {
				require.Equal(suite.T(), r.selfIdx, uint32(1), "Self index not incremented.")
			} else {
				require.Equal(suite.T(), r.selfIdx, uint32(0), "Self index should not have been incremented.")
			}
		}
	}

	require.True(suite.T(), found, "Did not find peerId in succList.")

	mapPeer, ok := r.peerToRing[p.Id]
	require.True(suite.T(), ok, "Peer not found in map.")
	require.Equal(suite.T(), mapPeer.p, p, "Wrong peer representation stored.")

	r.add(p)
	assert.Equal(suite.T(), 2, len(r.succList), "Adding peer twice should fail.")
	assert.Equal(suite.T(), 2, r.length, "Adding peer twice should fail.")
	assert.Equal(suite.T(), 2, len(r.peerToRing), "Adding peer twice should fail.")

	// TODO test for accusation removal.
}

func (suite *RingTestSuite) TestRingRemove() {
	p := &Peer{
		Id: "testId",
	}

	r := suite.ring

	r.add(p)

	r.remove(p)
	assert.Zero(suite.T(), r.selfIdx, "Self index changed incorrectly.")

	for _, rId := range r.succList {
		require.NotEqual(suite.T(), rId.p.Id, p.Id, "Peer still present in ring after being removed.")
	}

	assert.Equal(suite.T(), 1, r.length, "SuccList has wrong number of elements.")
	assert.Equal(suite.T(), 1, len(r.succList), "SuccList has wrong number of elements.")
	assert.Equal(suite.T(), 1, len(r.peerToRing), "PeerToRing map has wrong number of elements.")

	r.remove(p)
	assert.Equal(suite.T(), 1, r.length, "Removing self should fail.")
	assert.Equal(suite.T(), 1, len(r.succList), "Removing same peer twice should fail.")
	assert.Equal(suite.T(), 1, len(r.peerToRing), "Removing same peer twice should fail.")
	assert.Zero(suite.T(), r.selfIdx, "Self index changed incorrectly.")

	r.remove(r.selfId.p)
	assert.Equal(suite.T(), 1, r.length, "Removing self should fail.")
	assert.Equal(suite.T(), 1, len(r.succList), "Removing self should fail.")
	assert.Equal(suite.T(), 1, len(r.peerToRing), "Removing self should fail.")
	assert.Zero(suite.T(), r.selfIdx, "Self index changed incorrectly.")
}

func (suite *RingTestSuite) TestIsPrev() {
	r := suite.ring

	for i := 0; i < 30; i++ {
		p := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		r.add(p)
	}

	p1 := r.succList[0].p
	p1Succ := r.succList[1].p
	p1Prev := r.succList[len(r.succList)-1].p

	r.remove(p1)

	p2 := r.succList[3].p
	p2Succ := r.succList[4].p
	p2Prev := r.succList[2].p

	r.remove(p2)
	r.remove(p2Succ)
	r.remove(p2Prev)

	p3 := r.succList[6]
	p3Prev := r.succList[5].p
	r.remove(p3.p)
	r.peerToRing[p3.p.Id] = p3

	selfPrev := r.predecessor().p

	tests := []struct {
		acc     *Peer
		accuser *Peer
		out     bool
	}{
		{
			acc:     r.succList[r.selfIdx].p,
			accuser: selfPrev,
			out:     true,
		},

		{
			acc:     p3.p,
			accuser: p3Prev,
			out:     false,
		},

		{
			acc:     p1,
			accuser: p1Prev,
			out:     true,
		},

		{
			acc:     p1,
			accuser: p1Succ,
			out:     false,
		},

		{
			acc:     p1Succ,
			accuser: p1,
			out:     true,
		},

		{
			acc:     p1Prev,
			accuser: p1,
			out:     false,
		},

		{
			acc:     p2,
			accuser: p2Prev,
			out:     true,
		},

		{
			acc:     p2,
			accuser: p2Succ,
			out:     false,
		},
	}

	for i, t := range tests {
		require.Equalf(suite.T(), t.out, r.isPrev(t.acc, t.accuser),
			"Invalid output for test %d.", i)
	}
}

func (suite *RingTestSuite) TestBetweenNeighbours() {
	p := &Peer{
		Id: "testId",
	}

	r := suite.ring

	assert.True(suite.T(), r.betweenNeighbours("test"), "Should return true with only myself in ring.")

	r.add(p)

	assert.True(suite.T(), r.betweenNeighbours(p.Id), "Should return true when already neighbour.")

	for i := 0; i < 10; i++ {
		peer := &Peer{
			Id: fmt.Sprintf("testId%d", i),
		}

		r.add(peer)
	}

	succ := r.successor()
	prev := r.predecessor()
	for _, rId := range r.succList {
		if eq := rId.equal(r.selfId); eq {
			assert.True(suite.T(), r.betweenNeighbours(rId.p.Id), "Should return true with self id.")
		} else if !succ.equal(rId) && !prev.equal(rId) {
			assert.False(suite.T(), r.betweenNeighbours(rId.p.Id), "Non-neighbour should return false.")
		} else {
			assert.True(suite.T(), r.betweenNeighbours(rId.p.Id), "Neighbour should return false.")
		}
	}

}

func (suite *RingTestSuite) TestNeigbours() {
	var ids []string

	r := suite.ring

	succ, prev := r.neighbours("test")
	assert.Equal(suite.T(), succ.p, r.selfId.p, "Should return myself as successor with only myself in ring.")
	assert.Equal(suite.T(), prev.p, r.selfId.p, "Should return myself as predecessor with only myself in ring.")

	p := &Peer{
		Id: "testId",
	}

	r.add(p)

	for _, rId := range r.succList {
		ids = append(ids, rId.p.Id)
	}

	succ, prev = r.neighbours("non-existing-id")
	assert.ElementsMatch(suite.T(), ids, []string{succ.p.Id, prev.p.Id}, "Should find neighbours of non-existing id.")

	succ, prev = r.neighbours(p.Id)
	assert.Equal(suite.T(), succ.p, r.selfId.p, "Should return myself with only 1 other in ring.")
	assert.Equal(suite.T(), prev.p, r.selfId.p, "Should return myself with only 1 other in ring.")

}

func (suite *RingTestSuite) TestIsBetween() {
	r := suite.ring

	rId := &ringId{
		hash: hashId(r.ringNum, []byte("testId")),
	}

	rId2 := &ringId{
		hash: hashId(r.ringNum, []byte("testId2")),
	}

	assert.True(suite.T(), isBetween(rId, rId2, rId), "Should return true if start equals new.")
	assert.True(suite.T(), isBetween(rId2, rId, rId), "Should return true if end equals new.")

	mid := &ringId{
		hash: hashId(r.ringNum, []byte("testId12345")),
	}

	high := &ringId{
		hash: genHigherId(mid.hash, 1),
	}

	low := &ringId{
		hash: genLowerId(mid.hash, 1),
	}

	assert.True(suite.T(), isBetween(low, high, mid), "Should return true when new id is between start and end.")
	assert.False(suite.T(), isBetween(low, mid, high), "Should return false when new id is greater than end without wrap-around.")
	assert.False(suite.T(), isBetween(mid, high, low), "Should return false when new id is less than start without wrap-around.")

	assert.True(suite.T(), isBetween(high, mid, low), "Should return true when new id is lower than start and end with wrap-around.")
	assert.False(suite.T(), isBetween(high, low, mid), "Should return false when new id is higher than end with wrap-around.")

	assert.True(suite.T(), isBetween(mid, mid, high), "Should return true when start and end are equal.")

}

func (suite *RingTestSuite) TestSuccessor() {
	var succ *ringId

	r := suite.ring

	p := &Peer{
		Id: "testId",
	}

	r.add(p)

	if r.selfIdx == 0 {
		succ = r.succList[1]
	} else {
		succ = r.succList[0]
	}

	assert.Equal(suite.T(), succ, r.successor(), "Returned wrong successor.")

}

func (suite *RingTestSuite) TestPredecessor() {
	r := suite.ring

	var prev *ringId

	p := &Peer{
		Id: "testId",
	}

	r.add(p)

	if r.selfIdx == 0 {
		prev = r.succList[1]
	} else {
		prev = r.succList[0]
	}

	assert.Equal(suite.T(), prev, r.predecessor(), "Returned wrong predecessor.")
}

func (suite *RingTestSuite) TestSuccAtIdx() {
	var idx int

	r := suite.ring

	p := &Peer{
		Id: "testId",
	}

	r.add(p)

	if r.selfIdx == 0 {
		idx = 1
	} else {
		idx = 0
	}

	assert.Equal(suite.T(), r.succList[idx], r.succAtIdx(int(r.selfIdx)), "Returned wrong successor.")
}

func (suite *RingTestSuite) TestPrevAtIdx() {
	var idx int

	r := suite.ring

	p := &Peer{
		Id: "testId",
	}

	r.add(p)

	if r.selfIdx == 0 {
		idx = 1
	} else {
		idx = 0
	}

	assert.Equal(suite.T(), r.succList[idx], r.prevAtIdx(int(r.selfIdx)), "Returned wrong predecessor.")
}

func (suite *RingTestSuite) TestHashId() {
	r := suite.ring

	p := &Peer{
		Id: "testId",
	}

	hash := hashId(r.ringNum, []byte(p.Id))
	require.NotNil(suite.T(), hash, "Returned hash was nil.")

	assert.Equal(suite.T(), sha256.Size, len(hash), "Hash was of wrong length.")

}

func genHigherId(id []byte, val int) []byte {
	var prev big.Int

	tmp := make([]byte, len(id))

	increment := big.NewInt(int64(val))

	copy(tmp, id)
	tmpVal := prev.SetBytes(tmp)

	prev.Add(tmpVal, increment)

	return prev.Bytes()
}

func genLowerId(id []byte, val int) []byte {
	var prev big.Int

	tmp := make([]byte, len(id))

	decrement := big.NewInt(int64(val))

	copy(tmp, id)

	tmpVal := prev.SetBytes(tmp)

	prev.Sub(tmpVal, decrement)

	return prev.Bytes()
}
