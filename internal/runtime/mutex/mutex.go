package mutex

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"

	cmap "github.com/orcaman/concurrent-map"
	log "github.com/sirupsen/logrus"
)

var mutexMap cmap.ConcurrentMap
var mutexMapSync sync.Mutex

func init() {
	mutexMap = cmap.New()
	mutexMapSync = sync.Mutex{}
}
func IsEmpty() bool {
	return mutexMap.Count() == 0
}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(fmt.Sprintf("Current number of locks: %d\nLocks: %+v\nUse /mutex/unlock/{id} endpoint to mutex", len(mutexMap.Keys()), mutexMap.Keys())))
}

func UnlockHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if m, ok := mutexMap.Get(vars["id"]); ok {
		m.(*sync.Mutex).Unlock()
		w.Write([]byte(fmt.Sprintf("Unlocked mutex %s.\nCurrent number of locks: %d\nLocks: %+v",
			vars["id"], len(mutexMap.Keys()), mutexMap.Keys())))
		return
	}
	w.Write([]byte(fmt.Sprintf("Mutex %s not found!", vars["id"])))
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

// LockWithContext locks a mutex only if it hasn't been locked before in a context.
// LockWithContext should be used to lock objects like faucets etc.
// The context carries a uid that is unique the each request (message, button press, etc.).
// If the uid has a lock already *for a certain object*, it increments the
// nLocks in the mutexMap. If not, it locks the object. This is supposed to lock only if nLock == 0.
func LockWithContext(ctx context.Context, s string) {
	uid := ctx.Value("uid").(string)
	if len(uid) == 0 {
		log.Error("[Mutex] LockWithContext: uid is empty!")
	}
	if len(s) == 0 {
		log.Error("[Mutex] LockWithContext: s is empty!")
	}
	// sync mutex to sync checkSoftLock with the increment of nLocks
	// same user can't lock the same object multiple times
	Lock(fmt.Sprintf("mutex-sync:%s:%s", s, uid))
	var nLocks = checkSoftLock(uid)
	if nLocks == 0 {
		Lock(s)
	} else {
		log.Tracef("[Mutex] Skip lock (nLocks: %d)", nLocks)
	}
	nLocks++
	mutexMap.Set(fmt.Sprintf("nLocks:%s", uid), nLocks)
	Unlock(fmt.Sprintf("mutex-sync:%s:%s", s, uid))
}

// UnlockWithContext unlock a mutex only if it has been locked once within a context.
// If it has been locked more than once
// it only decrements nLocks and skips the unlock of the mutex. This is supposed to unlock only for
// nLocks == 1
func UnlockWithContext(ctx context.Context, s string) {
	uid := ctx.Value("uid").(string)
	if len(uid) == 0 {
		log.Error("[Mutex] UnlockWithContext: uid is empty!")
		return
	}
	if len(s) == 0 {
		log.Error("[Mutex] UnlockWithContext: s is empty!")
		return
	}
	Lock(fmt.Sprintf("mutex-sync:%s:%s", s, uid))
	var nLocks = checkSoftLock(uid)
	nLocks--
	mutexMap.Set(fmt.Sprintf("nLocks:%s", uid), nLocks)
	if nLocks == 0 {
		Unlock(s)
		mutexMap.Remove(fmt.Sprintf("nLocks:%s", uid))
	} else {
		log.Tracef("[Mutex] Skip unlock (nLocks: %d)", nLocks)
	}
	Unlock(fmt.Sprintf("mutex-sync:%s:%s", s, uid))
	//mutexMap.Remove(fmt.Sprintf("mutex-sync:%s:%s", s, uid))
}

// Lock locks a mutex in the mutexMap. If the mutex is already in the map, it locks the current call.
// After it another call unlocks the mutex (and deletes it from the mutexMap) the mutex written again into the mutexMap.
// If the mutex was not in the mutexMap before, a new mutext is created and locked and written into the mutexMap.
func Lock(s string) {
	log.Tracef("[Mutex] Attempt Lock %s", s)
	if m, ok := mutexMap.Get(s); ok {
		m.(*sync.Mutex).Lock()
		// write into mutex map
		mutexMapSync.Lock()
		mutexMap.Set(s, m)
		mutexMapSync.Unlock()
	} else {
		m := &sync.Mutex{}
		m.Lock()
		tick := runtime.NewResettableFunctionTicker(s, runtime.WithDuration(time.Hour))
		tick.Do(
			func() {
				Unlock(s)
			})
		// write into mutex map
		mutexMapSync.Lock()
		mutexMap.Set(fmt.Sprintf("ticker_%s", s), tick)
		mutexMap.Set(s, m)
		mutexMapSync.Unlock()
	}
	log.Tracef("[Mutex] Locked %s", s)
}

// Unlock unlocks a mutex in the mutexMap.
func Unlock(s string) {
	mutexMapSync.Lock()
	if m, ok := mutexMap.Get(s); ok {
		mutexMap.Remove(s)
		tickerKey := fmt.Sprintf("ticker_%s", s)
		if tick, ok := mutexMap.Get(tickerKey); ok {
			tick.(*runtime.ResettableFunctionTicker).StopChan <- struct{}{}
			mutexMap.Remove(tickerKey)
		}
		m.(*sync.Mutex).Unlock()
		log.Tracef("[Mutex] Unlocked %s", s)
	} else {
		// this should never happen. Mutex should have been in the mutexMap.
		log.Errorf("[Mutex] ⚠️⚠️⚠️ Unlock %s not in mutexMap. Skip.", s)
	}
	mutexMapSync.Unlock()
}
