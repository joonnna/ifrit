package node

import ( 
	"math/rand"
)

func (n *Node) getNeighbours() []string {
	n.neighbourMutex.RLock()
	
	ret := make([]string, len(n.neighbours))
	copy(ret, n.neighbours)

	n.neighbourMutex.RUnlock()
	return ret
}


func (n *Node) getRandomNeighbour() string {
	n.neighbourMutex.RLock()
	
	idx := rand.Int() % len(n.neighbours)
	ret := n.neighbours[idx]

	n.neighbourMutex.RUnlock()
	return ret
}


func (n *Node) addNeighbour(addr string) {
	n.neighbourMutex.Lock()

	n.neighbours = append(n.neighbours, addr)

	n.neighbourMutex.Unlock()
}
