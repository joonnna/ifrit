package discovery

import (
	"errors"
	_ "fmt"
	_ "math/big"

	log "github.com/inconshreveable/log15"
)

var (
	errIdNotFound = errors.New("ring id not found.")
	errZeroLength = errors.New("Slice is of zero length.")
)

/*
type item interface {
	compare(item) int
}
*/

func insert(slice []*ringId, newId *ringId, length int) ([]*ringId, int) {
	var currIdx int
	maxIdx := int(length) - 1

	if length == 0 {
		return []*ringId{newId}, 0
	}

	for {
		if currIdx >= maxIdx {
			if slice[currIdx].compare(newId) == -1 {
				currIdx += 1
			}

			ret := append(slice[:currIdx], append([]*ringId{newId}, slice[currIdx:]...)...)

			return ret, currIdx
		}
		mid := (currIdx + maxIdx) / 2

		if cmp := slice[mid].compare(newId); cmp == -1 {
			currIdx = mid + 1
		} else if cmp == 1 {
			maxIdx = mid - 1
		} else {
			// TODO change this? (handled by higher lvl code)
			maxIdx = mid - 1
			log.Error("Found equal ids, system will behave undefined.")
		}
	}
}

func search(slice []*ringId, searchId *ringId, length int) (int, error) {
	var currIdx int

	if length == 0 {
		return 0, errZeroLength
	}

	maxIdx := length - 1

	for {
		if currIdx >= maxIdx {
			if slice[currIdx].compare(searchId) == 0 {
				return currIdx, nil
			} else {
				return 0, errIdNotFound
			}
		}
		mid := (currIdx + maxIdx) / 2

		cmp := slice[mid].compare(searchId)

		if cmp == -1 {
			currIdx = mid + 1
		} else if cmp == 1 {
			maxIdx = mid - 1
		} else {
			return mid, nil
		}
	}
}

func findSuccAndPrev(slice []*ringId, searchId *ringId, length int) (*ringId, *ringId) {
	var currIdx int
	var successor, predecessor *ringId

	maxIdx := length - 1

	for {
		if currIdx >= maxIdx {
			if cmp := slice[currIdx].compare(searchId); cmp == -1 {
				predecessor = slice[currIdx]
				successor = succ(slice, currIdx, length)
			} else if cmp == 1 {
				successor = slice[currIdx]
				predecessor = prev(slice, currIdx, length)
			} else {
				successor = succ(slice, currIdx, length)
				predecessor = prev(slice, currIdx, length)
			}

			return successor, predecessor
		}
		mid := (currIdx + maxIdx) / 2

		cmp := slice[mid].compare(searchId)

		if cmp == -1 {
			currIdx = mid + 1
		} else if cmp == 1 {
			maxIdx = mid - 1
		} else {
			successor = succ(slice, mid, length)
			predecessor = prev(slice, mid, length)
			return successor, predecessor
		}
	}
}

func succ(slice []*ringId, idx, length int) *ringId {
	return slice[(idx+1)%length]
}

func prev(slice []*ringId, idx, length int) *ringId {
	i := (idx - 1) % length
	if i < 0 {
		return slice[i+length]
	} else {
		return slice[i]
	}
}
