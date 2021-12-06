package rate

import (
	"context"
	log "github.com/sirupsen/logrus"
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
	checkIdLimiter := func(id string) {
		log.Printf("[limiter] checking chat limiter for %s", id)
		if !isAllowed(limiter.ChatID, id) {
			log.Printf("[limiter] rate limit reached")
			retryLimit()
		}
	}
	checkGlobalLimiter := func() {
		log.Printf("[limiter] checking global limiter")
		if !isAllowed(limiter.Global, "global") {
			log.Printf("[limiter] rate limit reached")
			retryLimit()
		}
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
	checkIdLimiter(id)
}
