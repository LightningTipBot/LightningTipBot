package runtime

import (
	"context"
	cmap "github.com/orcaman/concurrent-map"
	"time"
)

var retryMap cmap.ConcurrentMap

func init() {
	retryMap = cmap.New()
}

// ResettableFunctionTicker will reset the user state as soon as tick is delivered.
type FunctionRetry struct {
	Ticker   *time.Ticker
	duration time.Duration
	ctx      context.Context
	name     string
}

type FunctionRetryOption func(*FunctionRetry)

func WithRetryDuration(d time.Duration) FunctionRetryOption {
	return func(a *FunctionRetry) {
		a.duration = d
	}
}
func NewRetryTicker(ctx context.Context, name string, option ...FunctionRetryOption) *FunctionRetry {
	t := &FunctionRetry{
		name: name,
		ctx:  ctx,
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

func (t *FunctionRetry) Do(f func()) {
	tickerMap.Set(t.name, t)
	go func() {
		for {
			select {
			case <-t.Ticker.C:
				// ticker delivered signal. do function f
				f()
			case <-t.ctx.Done():
				return
			}
		}
	}()
}
