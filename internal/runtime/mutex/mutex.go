package mutex

import (
	"fmt"
	cmap "github.com/orcaman/concurrent-map"
	log "github.com/sirupsen/logrus"
	"sync"
)

var mutexMap cmap.ConcurrentMap

func init() {
	mutexMap = cmap.New()
}

// checkSoftLock checks in mutexMap how often an existing mutex was already SoftLocked.
// The counter is there to avoid multiple recursive locking of an object in the mutexMap.
// This happens if multiple handlers call each other and try to lock/unlock multiple times
// the same mutex.
func checkSoftLock(s string) int {
	if v, ok := mutexMap.Get(fmt.Sprintf("nLocks:%s", s)); ok {
		return v.(int)
	}
	return 0
}

// LockSoft locks a mutex only if it hasn't been locked before. If it has, it increments the
// nLocks in the mutexMap. This is supposed to lock only if nLock == 0.
func LockSoft(s string) {
	var nLocks = checkSoftLock(s)
	if nLocks == 0 {
		Lock(s)
	} else {
		log.Tracef("[Mutex] skipping LockSoft with nLocks: %d ", nLocks)
	}
	nLocks++
	mutexMap.Set(fmt.Sprintf("nLocks:%s", s), nLocks)

}

// UnlockSoft unlock a mutex only if it has been locked once. If it has been locked more than once
// it only decrements nLocks and skips the unlock of the mutex. This is supposed to unlock only for
// nLocks == 1
func UnlockSoft(s string) {
	var nLocks = checkSoftLock(s)
	if nLocks == 1 {
		Unlock(s)
	} else {
		log.Tracef("[Mutex] skipping UnlockSoft with nLocks: %d ", nLocks)
	}
	nLocks--
	mutexMap.Set(fmt.Sprintf("nLocks:%s", s), nLocks)
}

// Lock locks a mutex in the mutexMap.
func Lock(s string) {
	if m, ok := mutexMap.Get(s); ok {
		m.(*sync.Mutex).Lock()
	} else {
		m := &sync.Mutex{}
		m.Lock()
		mutexMap.Set(s, m)
	}
	log.Tracef("[Mutex] Lock %s", s)
}

// Unlock unlocks a mutex in the mutexMap.
func Unlock(s string) {
	if m, ok := mutexMap.Get(s); ok {
		log.Tracef("[Mutex] Unlock %s", s)
		m.(*sync.Mutex).Unlock()

	}
}
