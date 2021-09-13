package main

import (
	"math/rand"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func GetMemoFromCommand(command string, from_word int) string {
	// check for memo in command
	memo := ""
	if len(strings.Split(command, " ")) > from_word {
		memo = strings.SplitN(command, " ", from_word+1)[from_word]
		memoMaxLen := 159
		if len(memo) > memoMaxLen {
			memo = memo[:memoMaxLen]
		}
	}
	return memo
}
