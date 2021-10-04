package limiter

import (
	"golang.org/x/time/rate"
	"sync"
)

// IPRateLimiter .
type ChatIDRateLimiter struct {
	ids map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

// NewIPRateLimiter .
func NewChatIDRateLimiter(r rate.Limit, b int) *ChatIDRateLimiter {
	i := &ChatIDRateLimiter{
		ids: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}

	return i
}

// AddChatId creates a new rate limiter and adds it to the chat id map,
// using the chat id address as the key
func (i *ChatIDRateLimiter) AddChatId(id string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter := rate.NewLimiter(i.r, i.b)

	i.ids[id] = limiter

	return limiter
}

// GetLimiter returns the rate limiter for the provided IP address if it exists.
// Otherwise calls AddIP to add IP address to the map
func (i *ChatIDRateLimiter) GetLimiter(id string) *rate.Limiter {
	i.mu.Lock()
	limiter, exists := i.ids[id]

	if !exists {
		i.mu.Unlock()
		return i.AddChatId(id)
	}

	i.mu.Unlock()

	return limiter
}
