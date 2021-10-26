package telegram

import (
	"math"
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

func GetMemoFromCommand(command string, fromWord int) string {
	// check for memo in command
	memo := ""
	if len(strings.Split(command, " ")) > fromWord {
		memo = strings.SplitN(command, " ", fromWord+1)[fromWord]
		memoMaxLen := 159
		if len(memo) > memoMaxLen {
			memo = memo[:memoMaxLen]
		}
	}
	return memo
}

func MakeProgressbar(current int, total int) string {
	MAX_BARS := 16
	progress := math.Round((float64(current) / float64(total)) * float64(MAX_BARS))
	progressbar := strings.Repeat("🟩", int(progress))
	progressbar += strings.Repeat("⬜️", MAX_BARS-int(progress))
	return progressbar
}

func MakeTipjarbar(current int, total int) string {
	MAX_BARS := 16
	progress := math.Round((float64(current) / float64(total)) * float64(MAX_BARS))
	progressbar := strings.Repeat("🍯", int(progress))
	progressbar += strings.Repeat("⬜️", MAX_BARS-int(progress))
	return progressbar
}
