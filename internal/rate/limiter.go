package rate

import (
	"context"
	"golang.org/x/time/rate"
	tb "gopkg.in/lightningtipbot/telebot.v2"
	"strconv"
	"sync"
)

type LimiterWrapper struct {
	Global *Limiter
	ChatID *Limiter
}

// NewLimiterWrapper creates both chat and global rate limiters.
func NewLimiterWrapper() *LimiterWrapper {
	chatIdRateLimiter := NewRateLimiter(rate.Limit(0.3), 20)

	globalLimiter := NewRateLimiter(rate.Limit(30), 30)
	return &LimiterWrapper{Global: globalLimiter, ChatID: chatIdRateLimiter}
}

func CheckLimit(to interface{}, limiter *LimiterWrapper) {
	checkIdLimiter := func(id string) {
		limiter.ChatID.GetLimiter(id).Wait(context.Background())
	}
	checkGlobalLimiter := func() {
		limiter.Global.GetLimiter("global").Wait(context.Background())
	}
	checkGlobalLimiter()

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
		checkIdLimiter(id)
	}
}

// IPRateLimiter .
type Limiter struct {
	keys map[string]*rate.Limiter
	mu   *sync.RWMutex
	r    rate.Limit
	b    int
}

// NewRateLimiter .
func NewRateLimiter(r rate.Limit, b int) *Limiter {
	i := &Limiter{
		keys: make(map[string]*rate.Limiter),
		mu:   &sync.RWMutex{},
		r:    r,
		b:    b,
	}

	return i
}

// Add creates a new rate limiter and adds it to the keys map,
// using the IP address as the key
func (i *Limiter) Add(key string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter := rate.NewLimiter(i.r, i.b)

	i.keys[key] = limiter

	return limiter
}

// GetLimiter returns the rate limiter for the provided IP address if it exists.
// Otherwise calls AddIP to add IP address to the map
func (i *Limiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	limiter, exists := i.keys[ip]

	if !exists {
		i.mu.Unlock()
		return i.Add(ip)
	}

	i.mu.Unlock()

	return limiter
}
