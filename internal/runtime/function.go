package runtime

import (
	cmap "github.com/orcaman/concurrent-map"
	"sync"
	"time"
)

var tickerMap cmap.ConcurrentMap

func init() {
	tickerMap = cmap.New()
}

var defaultTickerCoolDown = time.Second * 10

// ResettableFunctionTicker will reset the user state as soon as tick is delivered.
type ResettableFunctionTicker struct {
	Ticker    *time.Ticker
	ResetChan chan struct{} // channel used to reset the ticker
	duration  time.Duration
	Started   bool
	name      string
	Quit      chan struct{}
	Wg        sync.WaitGroup
}
type ResettableFunctionTickerOption func(*ResettableFunctionTicker)

func WithDuration(d time.Duration) ResettableFunctionTickerOption {
	return func(a *ResettableFunctionTicker) {
		a.duration = d
	}
}

func RemoveTicker(name string) {
	tickerMap.Remove(name)
}
func GetTicker(name string, option ...ResettableFunctionTickerOption) *ResettableFunctionTicker {
	if t, ok := tickerMap.Get(name); ok {
		return t.(*ResettableFunctionTicker)
	} else {
		t := NewResettableFunctionTicker(name, option...)
		tickerMap.Set(name, t)
		return t
	}
}
func NewResettableFunctionTicker(name string, option ...ResettableFunctionTickerOption) *ResettableFunctionTicker {
	t := &ResettableFunctionTicker{
		ResetChan: make(chan struct{}, 1),
		Quit:      make(chan struct{}, 1),
		Wg:        sync.WaitGroup{},
		name:      name,
	}

	for _, opt := range option {
		opt(t)
	}
	if t.duration == 0 {
		t.duration = defaultTickerCoolDown
	}
	t.Ticker = time.NewTicker(t.duration)
	return t
}

func (t *ResettableFunctionTicker) DoWait(f func()) {
	t.Do(f)
	t.Wg.Wait()
}
func (t *ResettableFunctionTicker) Do(f func()) {
	t.Started = true
	tickerMap.Set(t.name, t)
	t.Wg.Add(1)
	go func() {
		defer t.Wg.Done()
		for {
			select {
			case <-t.Quit:
				return
			case <-t.Ticker.C:
				// ticker delivered signal. do function f
				f()
				return
			case <-t.ResetChan:
				// reset signal received. creating new ticker.
				t.Ticker = time.NewTicker(t.duration)
			}
		}
	}()
}
