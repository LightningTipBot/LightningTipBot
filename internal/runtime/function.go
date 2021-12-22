package runtime

import (
	"time"
)

var defaultTickerCoolDown = time.Second * 10

// ResettableFunctionTicker will reset the user state as soon as tick is delivered.
type ResettableFunctionTicker struct {
	Ticker    *time.Ticker
	ResetChan chan struct{} // channel used to reset the ticker
	duration  time.Duration
}
type ResettableFunctionTickerOption func(*ResettableFunctionTicker)

func WithDuration(d time.Duration) ResettableFunctionTickerOption {
	return func(a *ResettableFunctionTicker) {
		a.duration = d
	}
}

func NewResettableFunctionTicker(option ...ResettableFunctionTickerOption) *ResettableFunctionTicker {
	t := &ResettableFunctionTicker{
		ResetChan: make(chan struct{}, 1),
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

func (t ResettableFunctionTicker) Do(f func()) {
	for {
		select {
		case <-t.Ticker.C:
			// ticker delivered signal. do function f
			f()
			return
		case <-t.ResetChan:
			// reset signal received. creating new ticker.
			t.Ticker = time.NewTicker(t.duration)
		}
	}
}
