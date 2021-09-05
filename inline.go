package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/buntdb"
	"github.com/tidwall/gjson"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	sendInlineConfirmMessage = "Send inline confirm"
	sendInlineCancelMessage  = "Send inline confirm"
)

func updateSendInlineMessage(user *tb.User, bot TipBot) {
	runtime.IgnoreError(bot.bunt.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend(storage.MessageOrderedByReplyToFrom, func(key, value string) bool {
			replyToUserId := gjson.Get(value, storage.MessageOrderedByReplyToFrom)
			if replyToUserId.String() == strconv.Itoa(user.ID) {
				log.Infoln("loading persisted tip tool tip messages")
				ttt := &TipTooltip{}
				err := json.Unmarshal([]byte(value), ttt)
				if err != nil {
					log.Println(err)
				}
				err = ttt.editTooltip(&bot, false)
				if err != nil {
					log.Printf("[tipTooltipInitializedHandler] could not edit tooltip: %s", err.Error())
				}
			}

			return true
		})
		return err
	}))
}

func (bot *TipBot) sendInlineHandler(c *tb.Callback) {
	bot.tryEditMessage(c.Message, &tb.ReplyMarkup{})
	bot.trySendMessage(c.Sender, fmt.Sprintf("Pressed Send by user %s", GetUserStr(c.Sender)))
}

func (bot *TipBot) cancelSendInlineHandler(c *tb.Callback) {
	bot.tryEditMessage(c.Message, &tb.ReplyMarkup{})
	bot.trySendMessage(c.Message.Chat, paymentCancelledMessage)
}
