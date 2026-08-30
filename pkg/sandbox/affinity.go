package sandbox

import (
	"runtime"
	"sync"
)

var (
	corePoolMu sync.Mutex
	nextCore   int
)

// AllocateNextCore returns the next available CPU core index in a round-robin pool.
func AllocateNextCore() int {
	corePoolMu.Lock()
	defer corePoolMu.Unlock()

	numCPU := runtime.NumCPU()
	if numCPU <= 1 {
		return 0
	}

	core := nextCore % numCPU
	nextCore++
	return core
}
