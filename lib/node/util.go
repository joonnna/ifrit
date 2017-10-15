package node

import (
	_ "bytes"
	_ "crypto/sha256"
	_ "encoding/json"
	_ "math/big"
)

/*
func newMsg(code uint8, payload []byte) ([]byte, error) {
	msg := &message{
		Code: code,
		Payload: payload,
	}

	marsh, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(marsh)

	err = json.NewEncoder(buf).Encode(msg)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
func idSliceDiff(s1, s2 []*id) []*id {
	var retSlice []*id
	m := make(map[string]bool)

	for _, s := range s1 {
		m[s.Addr] = true
	}

	for _, s := range s2 {
		if _, ok := m[s.Addr]; !ok {
			retSlice = append(retSlice, s)
		}
	}

	return retSlice
}


func newNodeId (addr string) *nodeId {
	var hashVal big.Int

	h := sha256.New()
	h.Write([]byte(addr))

	hashVal.SetBytes(h.Sum(nil))

	id := &nodeId{
		Addr: addr,
		Hash: hashVal,
	}
	return id
}
*/
