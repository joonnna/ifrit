package node

import "sync"

type view struct {
	fullViewMap   map[string]*peer
	fullViewMutex sync.RWMutex

	liveMap   map[string]*peer
	liveMutex sync.RWMutex

	timeoutMap   map[string]*timeout
	timeoutMutex sync.RWMutex
}
