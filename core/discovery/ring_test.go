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

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

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
		require.True(suite.T(), suite.rings.isPredecessor(suite.rings.self.Id, p.Id, r.ringNum), "Should be predecessor with only 2 in rings.")
	}

	assert.False(suite.T(), suite.rings.isPredecessor(suite.rings.self.Id, p.Id, suite.rings.numRings+1), "Should return false with invalid ringNum.")

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
	p := &Peer{
		Id: "testId",
	}

	r := suite.ring

	r.add(p)

	assert.True(suite.T(), r.isPrev(r.selfId.p.Id, p.Id), "Should be prev with only 2 in rings.")

	assert.False(suite.T(), r.isPrev("", p.Id), "Should return false with empty id.")
	assert.False(suite.T(), r.isPrev(r.selfId.p.Id, ""), "Should return false with empty toCheck.")
	assert.False(suite.T(), r.isPrev("non-existing", p.Id), "Should return false with non-existing id.")
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

/*
func (suite *RingTestSuite) SetupTest() {
	var err error
	addr := "localhost:1234"
	id := genId()

	suite.ring, err = newRing(1, id, addr)
	require.NoError(suite.T(), err, "Failed to create ring")
	require.Equal(suite.T(), suite.ring.ownIdx, 0)
}

func TestRingTestSuite(t *testing.T) {
	suite.Run(t, new(RingTestSuite))
}

func (suite *RingTestSuite) TestValidAdd() {
	id := genId()
	addr := "localhost:8000"

	err := suite.add(id, addr)
	assert.NoError(suite.T(), err, "Valid add fails")
}

func (suite *RingTestSuite) TestAddSameIdTwice() {
	id := genId()
	addr := "localhost:8000"

	err := suite.add(id, addr)
	require.NoError(suite.T(), err, "Valid add returns error")

	err = suite.add(id, addr)
	assert.Error(suite.T(), err, "Adding same id twice returns non-nil error")
}

func (suite *RingTestSuite) TestAddSelf() {
	err := suite.add([]byte(suite.localRingId.key), suite.localRingId.addr)
	assert.Error(suite.T(), err, "Adding self returns non-nil error")
}

func (suite *RingTestSuite) TestOwnIdxOnFailedAdd() {
	val := suite.ownIdx

	err := suite.add([]byte(suite.localRingId.key), suite.localRingId.addr)
	require.Error(suite.T(), err, "Adding self returns non-nil error")

	assert.Equal(suite.T(), val, suite.ownIdx, "ownIdx changed on failed add")
}

func (suite *RingTestSuite) TestAddUpdatesExistMap() {
	id := genId()
	key := string(id[:])
	addr := "localhost:8000"

	err := suite.add(id, addr)
	require.NoError(suite.T(), err, "Valid add fails")

	assert.True(suite.T(), suite.existsMap[key], "Existmap not updated")
}

func (suite *RingTestSuite) TestValidRemove() {
	id := genId()
	addr := "localhost:8000"

	err := suite.add(id, addr)
	require.NoError(suite.T(), err, "Valid add fails")

	err = suite.remove(id)
	assert.NoError(suite.T(), err, "Valid remove fails")
}

func (suite *RingTestSuite) TestRemoveSelf() {
	err := suite.remove(suite.localRingId.id)
	assert.Error(suite.T(), err, "Removing self returns non-nil error")
}

func (suite *RingTestSuite) TestRemoveUpdatesExistMap() {
	id := genId()
	key := string(id[:])
	addr := "localhost:8000"

	err := suite.add(id, addr)
	require.NoError(suite.T(), err, "Valid add fails")

	require.True(suite.T(), suite.existsMap[key], "Existmap not updated on add")

	err = suite.remove(id)
	require.NoError(suite.T(), err, "Valid remove fails")

	assert.False(suite.T(), suite.existsMap[key], "Existmap not updated on remove")
}

func (suite *RingTestSuite) TestIsBetween() {
	node := &ringId{
		id: genId(),
	}

	prev := &ringId{
		id: genLowerId(node.id),
	}

	succ := &ringId{
		id: genHigherId(node.id),
	}

	assert.False(suite.T(), isBetween(node, succ, prev), "prev should not be between node and succ")
	assert.False(suite.T(), isBetween(prev, node, succ), "succ should not be between node and prev")
	assert.True(suite.T(), isBetween(node, prev, succ), "wrap around not working")
	assert.True(suite.T(), isBetween(succ, node, prev), "wrap around not working")
	assert.True(suite.T(), isBetween(succ, node, node), "equal end and new should return true")
	assert.True(suite.T(), isBetween(succ, succ, node), "equal start and end should return true")
}

func (suite *RingTestSuite) TestBetweenNeighboursWhenAlone() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")

	assert.True(suite.T(), suite.betweenNeighbours(p.id), "Should be between when alone")
}

func (suite *RingTestSuite) TestBetweenNeighboursWithTwo() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")

	err := suite.add(p.id, p.addr)
	require.NoError(suite.T(), err, "Valid add fails")

	assert.True(suite.T(), suite.betweenNeighbours(p.id), "Should be between when only 2 in ring")

}

func (suite *RingTestSuite) TestBetweenNeighbours() {
	pSlice := make([]*peer, 0)
	for i := 0; i < 10; i++ {
		id := genId()
		p, _ := newTestPeer(string(id[:]), 1, "")
		pSlice = append(pSlice, p)

		err := suite.add(p.id, p.addr)
		require.NoError(suite.T(), err, "Valid add fails")
	}

	succ, err := suite.getRingSucc()
	require.NoError(suite.T(), err, "Can't find successor")

	prev, err := suite.getRingPrev()
	require.NoError(suite.T(), err, "Can't find prev")

	for _, p := range pSlice {
		if p.key != succ.key && p.key != prev.key {
			assert.False(suite.T(), suite.betweenNeighbours(p.id), "Shouldn't be neighbours")
		} else {
			assert.True(suite.T(), suite.betweenNeighbours(p.id), "Should be neighbours")
		}
	}
}

func (suite *RingTestSuite) TestFindNeighboursWhenAlone() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")

	succ, prev, err := suite.findNeighbours(p.id)
	require.NoError(suite.T(), err, "Failed to find neighbours when alone")
	assert.Equal(suite.T(), succ, suite.localRingId.key, "I should be successor when alone")
	assert.Equal(suite.T(), prev, suite.localRingId.key, "I should be prev when alone")
}

func (suite *RingTestSuite) TestFindNeighbours() {
	peers := make(map[string]bool)
	peers[suite.localRingId.key] = true

	pSlice := make([]*peer, 0)
	for i := 0; i < 10; i++ {
		id := genId()
		p, _ := newTestPeer(string(id[:]), 1, "")
		pSlice = append(pSlice, p)
		peers[p.key] = true

		err := suite.add(p.id, p.addr)
		require.NoError(suite.T(), err, "Valid add fails")
	}

	for _, p := range pSlice {
		succ, prev, err := suite.findNeighbours(p.id)
		require.NoError(suite.T(), err, "Failed to find neighbours")

		_, succExist := peers[succ]
		_, prevExist := peers[prev]

		assert.True(suite.T(), succExist, "found invalid successor neighbour")
		assert.True(suite.T(), prevExist, "found invalid prev neighbour")
	}
}

func (suite *RingTestSuite) TestFindPrevWhenAlone() {
	id := []byte(suite.localRingId.key)
	prev, err := suite.findPrev(id)
	require.NoError(suite.T(), err, "Failed to find prev when alone")
	assert.Equal(suite.T(), prev, suite.localRingId.key, "I should be prev when alone")
}

func (suite *RingTestSuite) TestFindPrevWhenTwoInRing() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")
	err := suite.add(p.id, p.addr)
	require.NoError(suite.T(), err, "Valid add fails")

	prev, err := suite.findPrev([]byte(suite.localRingId.key))
	require.NoError(suite.T(), err, "Failed to find prev when 2 in ring")
	assert.Equal(suite.T(), prev, p.key, "Should be prev when only 2 in ring")
}

func (suite *RingTestSuite) TestFindPrevWhenNotInRing() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")

	_, err := suite.findPrev(p.id)
	require.Error(suite.T(), err, "Should fail to find prev of non-existing id")
}

func (suite *RingTestSuite) TestFindPrev() {
	peers := make(map[string]bool)
	peers[suite.localRingId.key] = true

	pSlice := make([]*peer, 0)
	for i := 0; i < 10; i++ {
		id := genId()
		p, _ := newTestPeer(string(id[:]), 1, "")
		pSlice = append(pSlice, p)
		peers[p.key] = true

		err := suite.add(p.id, p.addr)
		require.NoError(suite.T(), err, "Valid add fails")
	}

	for _, p := range pSlice {
		prev, err := suite.findPrev(p.id)
		require.NoError(suite.T(), err, "Failed to find prev")

		_, prevExist := peers[prev]

		assert.True(suite.T(), prevExist, "found invalid prev")
	}
}

func (suite *RingTestSuite) TestIsPrevMyself() {
	id := []byte(suite.localRingId.key)
	prev, err := suite.isPrev(id, id)
	require.NoError(suite.T(), err, "Failed to find prev when alone")
	assert.True(suite.T(), prev, "I should be prev when alone")
}

func (suite *RingTestSuite) TestIsPrevWhenNotInRing() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")
	_, err := suite.isPrev(p.id, p.id)
	require.Error(suite.T(), err, "Should fail to find prev when id is not in ring")
}

func (suite *RingTestSuite) TestIsPrevWhenTwoInRing() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")
	err := suite.add(p.id, p.addr)
	require.NoError(suite.T(), err, "Valid add fails")

	prev, err := suite.isPrev([]byte(suite.localRingId.key), p.id)
	require.NoError(suite.T(), err, "Failed to find prev when 2 in ring")
	assert.True(suite.T(), prev, "Should be prev when only 2 in ring")
}

func (suite *RingTestSuite) TestIsPrev() {
	pSlice := make([]*peer, 0)
	for i := 0; i < 10; i++ {
		id := genId()
		p, _ := newTestPeer(string(id[:]), 1, "")
		pSlice = append(pSlice, p)

		err := suite.add(p.id, p.addr)
		require.NoError(suite.T(), err, "Valid add fails")
	}

	for _, p := range pSlice {
		prev, err := suite.findPrev(p.id)
		require.NoError(suite.T(), err, "Couldn't find prev")

		for _, p2 := range pSlice {
			isPrev, err := suite.isPrev(p.id, p2.id)
			require.NoError(suite.T(), err, "Couldn't check who is prev")
			if p2.key == prev {
				assert.True(suite.T(), isPrev, "Should be prev")
			} else {
				assert.False(suite.T(), isPrev, "Should not be prev")
			}

		}
	}
}
*/
