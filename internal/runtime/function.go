package runtime

import (
	cmap "github.com/orcaman/concurrent-map"
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
	StopChan  chan struct{} // channel used to stop the ticker
	duration  time.Duration
	Started   bool
	name      string
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
		StopChan:  make(chan struct{}, 1),
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

func (t *ResettableFunctionTicker) Do(f func()) {
	t.Started = true
	tickerMap.Set(t.name, t)
	go func() {
		for {
			select {
			case <-t.Ticker.C:
				// ticker delivered signal. do function f
				f()
				return
			case <-t.ResetChan:
				// reset signal received. creating new ticker.
				t.Ticker = time.NewTicker(t.duration)
			case <-t.StopChan:
				// reset signal received. creating new ticker.
				t.Ticker = nil
				tickerMap.Remove(t.name)
				return
			}
		}
	}()
}
