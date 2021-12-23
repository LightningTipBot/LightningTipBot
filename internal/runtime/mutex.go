package runtime

import (
	cmap "github.com/orcaman/concurrent-map"
	log "github.com/sirupsen/logrus"
	"sync"
)

var mutexMap cmap.ConcurrentMap

func init() {
	mutexMap = cmap.New()
}

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

func Unlock(s string) {
	if m, ok := mutexMap.Get(s); ok {
		log.Tracef("[Mutex] Unlock %s", s)
		m.(*sync.Mutex).Unlock()
	}
}
