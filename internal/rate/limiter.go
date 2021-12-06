package rate

import (
	"context"
	"golang.org/x/time/rate"
	tb "gopkg.in/lightningtipbot/telebot.v2"
	"strconv"
	"sync"
)

// Limiter
type Limiter struct {
	keys map[string]*rate.Limiter
	mu   *sync.RWMutex
	r    rate.Limit
	b    int
}

var idLimiter *Limiter
var globalLimiter *rate.Limiter

// NewLimiter creates both chat and global rate limiters.
func Start() {
	idLimiter = newIdRateLimiter(rate.Limit(0.3), 20)
	globalLimiter = rate.NewLimiter(rate.Limit(30), 30)
}

// NewRateLimiter .
func newIdRateLimiter(r rate.Limit, b int) *Limiter {
	i := &Limiter{
		keys: make(map[string]*rate.Limiter),
		mu:   &sync.RWMutex{},
		r:    r,
		b:    b,
	}

	return i
}

func CheckLimit(to interface{}) {
	globalLimiter.Wait(context.Background())
	var id string
	switch to.(type) {
	case *tb.Chat:
		id = strconv.FormatInt(to.(*tb.Chat).ID, 10)
	case *tb.User:
		id = strconv.FormatInt(to.(*tb.User).ID, 10)
	case tb.Recipient:
		id = to.(tb.Recipient).Recipient()
	case *tb.Message:
		if to.(*tb.Message).Chat != nil {
			id = strconv.FormatInt(to.(*tb.Message).Chat.ID, 10)
		}
	}
	if len(id) > 0 {
		idLimiter.GetLimiter(id).Wait(context.Background())
	}
}

// Add creates a new rate limiter and adds it to the keys map,
// using the key
func (i *Limiter) Add(key string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter := rate.NewLimiter(i.r, i.b)

	i.keys[key] = limiter

	return limiter
}

// GetLimiter returns the rate limiter for the provided key if it exists.
// Otherwise, calls Add to add key address to the map
func (i *Limiter) GetLimiter(key string) *rate.Limiter {
	i.mu.Lock()
	limiter, exists := i.keys[key]

	if !exists {
		i.mu.Unlock()
		return i.Add(key)
	}

	i.mu.Unlock()

	return limiter
}
