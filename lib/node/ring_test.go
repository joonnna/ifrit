package node

import (
	"math/big"
	"testing"

	"github.com/satori/go.uuid"
)

func genId() []byte {
	return uuid.NewV4().Bytes()
}

func genHigherId(id []byte) []byte {
	var val big.Int

	tmp := make([]byte, len(id))

	one := big.NewInt(1)

	copy(tmp, id)
	tmpVal := val.SetBytes(tmp)

	val.Add(tmpVal, one)

	return val.Bytes()
}

func genLowerId(id []byte) []byte {
	var val big.Int

	tmp := make([]byte, len(id))

	one := big.NewInt(1)

	copy(tmp, id)

	tmpVal := val.SetBytes(tmp)

	val.Sub(tmpVal, one)

	return val.Bytes()
}

func createRing() *ring {
	var ringNum uint8

	ringNum = 1

	addr := "localhost:1234"
	id := genId()

	return newRing(ringNum, id, string(id[:]), addr)
}

func TestAdd(t *testing.T) {
	r := createRing()

	if r.ownIdx != 0 {
		t.Errorf("Own index is not initially 0")
	}

	err := r.add([]byte(r.localRingId.peerKey), r.localRingId.peerKey, r.localRingId.addr)
	if err == nil {
		t.Errorf("Adding same id twice to ring returns non-nil error")
	}

	if r.ownIdx != 0 {
		t.Errorf("Own index changed after failed add")
	}

	id := genId()
	key := string(id[:])
	addr := "localhost:8000"

	err = r.add(id, key, addr)
	if err != nil {
		t.Errorf("Non-nil error on regular add")
	}

	if _, ok := r.existsMap[key]; !ok {
		t.Errorf("Existmap not updated properly")
	}
}

func TestRemove(t *testing.T) {
	r := createRing()

	err := r.remove([]byte(r.localRingId.peerKey), r.localRingId.peerKey)
	if err == nil {
		t.Errorf("Removing self results in non-nil error")
	}

	id := genId()
	key := string(id[:])
	addr := "localhost:8000"

	err = r.add(id, key, addr)
	if err != nil {
		t.Errorf("Non-nil error on regular add")
	}

	err = r.remove(id, key)
	if err != nil {
		t.Errorf("Non-nil error on regular remove")
	}

	if _, ok := r.existsMap[key]; ok {
		t.Errorf("Existmap not updated properly")
	}

	notExistId := genId()
	notExistKey := string(id[:])

	err = r.remove(notExistId, notExistKey)
	if err == nil {
		t.Errorf("Removing non-existing id returns a nil error")
	}
}

func TestIsBetween(t *testing.T) {
	node := &ringId{
		id: genId(),
	}

	prev := &ringId{
		id: genLowerId(node.id),
	}

	succ := &ringId{
		id: genHigherId(node.id),
	}

	if isBetween(node, succ, prev) {
		t.Errorf("prev should not be between node and succ")
	}

	if isBetween(prev, node, succ) {
		t.Errorf("succ should not be between node and prev")
	}

	if !isBetween(node, prev, succ) {
		t.Errorf("wrap around not working")
	}

	if !isBetween(succ, node, prev) {
		t.Errorf("wrap around not working")
	}

	if !isBetween(succ, node, node) {
		t.Errorf("equal end and new should return true")
	}

	if !isBetween(succ, succ, node) {
		t.Errorf("equal start and end should return true")
	}
}

func TestBetweenNeighbours(t *testing.T) {

}

func TestFindNeighbour(t *testing.T) {

}
