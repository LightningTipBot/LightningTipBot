package rate

import (
	"context"
	"strconv"
	"time"

	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

type Limiter struct {
	Global limiter.Store
	ChatID limiter.Store
}

// NewLimiter creates both chat and global rate limiters.
func NewLimiter() *Limiter {
	chatRateLimiter, err := memorystore.New(&memorystore.Config{Interval: time.Minute, Tokens: 20, SweepMinTTL: time.Minute})
	if err != nil {
		panic(err)
	}

	globalLimiter, err := memorystore.New(&memorystore.Config{Interval: time.Second, Tokens: 30, SweepMinTTL: 10 * time.Second})
	if err != nil {
		panic(err)
	}
	return &Limiter{Global: globalLimiter, ChatID: chatRateLimiter}
}

func isAllowed(l limiter.Store, key string) bool {
	_, _, _, ok, _ := l.Take(context.Background(), key)
	return ok
}
func CheckLimit(to interface{}, limiter *Limiter) {
	retryLimit := func() {
		time.Sleep(time.Second)
		CheckLimit(to, limiter)
	}
	checkChatLimiter := func(id string) {
		if !isAllowed(limiter.ChatID, id) {
			retryLimit()
		}
	}
	checkGlobalLimiter := func() {
		if !isAllowed(limiter.Global, "global") {
			retryLimit()
		}
	}
	switch to.(type) {
	case *tb.Chat:
		checkChatLimiter(strconv.FormatInt(to.(*tb.Chat).ID, 10))
	case *tb.User:
		checkChatLimiter(strconv.FormatInt(to.(*tb.User).ID, 10))
	case tb.Recipient:
		checkChatLimiter(to.(tb.Recipient).Recipient())
	case *tb.Message:
		checkChatLimiter(strconv.FormatInt(to.(*tb.Message).Chat.ID, 10))
	default:
		checkGlobalLimiter()
	}
}
