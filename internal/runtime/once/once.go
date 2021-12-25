package once

import (
	"fmt"
	cmap "github.com/orcaman/concurrent-map"
	"strconv"
)

var onceMap cmap.ConcurrentMap

func init() {
	onceMap = cmap.New()
}

func New(objectKey string) {
	onceMap.Set(objectKey, cmap.New())
}

func Once(key string, telegramUserId int64) error {
	i, ok := onceMap.Get(key)
	if ok {
		return writeOnceOrQuit(i.(cmap.ConcurrentMap), telegramUserId)
	}
	userMap := cmap.New()
	onceMap.Set(key, userMap)
	return writeOnceOrQuit(userMap, telegramUserId)
}

func writeOnceOrQuit(objectMap cmap.ConcurrentMap, telegramUserId int64) error {
	telegramUserString := strconv.FormatInt(telegramUserId, 10)
	if _, ok := objectMap.Get(telegramUserString); ok {
		return fmt.Errorf("user already consumed object")
	}
	objectMap.Set(telegramUserString, true)
	return nil
}

func Remove(key string) {
	onceMap.Remove(key)
}
