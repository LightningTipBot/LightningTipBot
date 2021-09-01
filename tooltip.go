package main

import (
	"encoding/json"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"github.com/tidwall/buntdb"
	"github.com/tidwall/gjson"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/tucnak/telebot.v2"
)

// updateToolTip updates existing tip tool tip in telegram
func (x *Message) updateTooltip(bot *TipBot, user *tb.User, amount int, notInitializedWallet bool) error {
	x.TipAmount += amount
	x.Ntips += 1
	x.Tippers = appendUinqueUsersToSlice(x.Tippers, user)
	x.LastTip = time.Now()
	err := x.editTooltip(bot, notInitializedWallet)
	if err != nil {
		return err
	}
	return bot.bunt.Set(x)
}

func tipTooltipInitializedHandler(user *tb.User, bot TipBot) {
	runtime.IgnoreError(bot.bunt.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend(storage.MessageOrderedByReplyToFrom, func(key, value string) bool {
			replyToUserId := gjson.Get(value, storage.MessageOrderedByReplyToFrom)
			if replyToUserId.String() == strconv.Itoa(user.ID) {
				log.Infoln("loading persisted tip tool tip messages")
				message := &Message{}
				err := json.Unmarshal([]byte(value), message)
				if err != nil {
					log.Println(err)
				}
				err = message.editTooltip(&bot, false)
				if err != nil {
					log.Printf("[tipTooltipInitializedHandler] could not edit tooltip: %s", err.Error())
				}
			}

			return true
		})
		return err
	}))
}

func (x *Message) editTooltip(bot *TipBot, notInitializedWallet bool) error {
	tipToolTip := x.getTooltipMessage(GetUserStrMd(bot.telegram.Me), notInitializedWallet)
	m, err := bot.telegram.Edit(x.Message, tipToolTip)
	if err != nil {
		return err
	}
	x.Message.Text = m.Text
	return nil
}

// getTippersString joins all tippers username or telegram id's as mentions (@username or [inline mention of a user](tg://user?id=123456789))
func getTippersString(tippers []*tb.User) string {
	var tippersStr string
	for _, uniqueUser := range tippers {
		userStr := GetUserStrMd(uniqueUser)
		tippersStr += fmt.Sprintf("%s, ", userStr)
	}
	// get rid of the trailing comma
	if len(tippersStr) > 2 {
		tippersStr = tippersStr[:len(tippersStr)-2]
	}
	tippersSlice := strings.Split(tippersStr, " ")
	// crop the message to the max length
	if len(tippersSlice) > maxNamesInTipperMessage {
		// tippersStr = tippersStr[:50]
		tippersStr = strings.Join(tippersSlice[:maxNamesInTipperMessage], " ")
		tippersStr = tippersStr + " ... and others"
	}
	return tippersStr
}

// getTooltipMessage will return the full tip tool tip
func (x Message) getTooltipMessage(botUserName string, notInitializedWallet bool) string {
	tippersStr := getTippersString(x.Tippers)
	tipToolTipMessage := fmt.Sprintf("🏅 %d sat", x.TipAmount)
	if len(x.Tippers) > 1 {
		tipToolTipMessage = fmt.Sprintf("%s (%d tips by %s)", tipToolTipMessage, x.Ntips, tippersStr)
	} else {
		tipToolTipMessage = fmt.Sprintf("%s (by %s)", tipToolTipMessage, tippersStr)
	}

	if notInitializedWallet {
		tipToolTipMessage = tipToolTipMessage + fmt.Sprintf("\n🗑 Chat with %s to manage your wallet.", botUserName)
	}
	return tipToolTipMessage
}

// tipTooltipExists checks if this tip is already known
func tipTooltipExists(replyToId int, bot *TipBot) (bool, *Message) {
	message := &Message{Message: &tb.Message{ReplyTo: &tb.Message{ID: replyToId}}}
	err := bot.bunt.Get(message)
	if err != nil {
		return false, message
	}
	return true, message

}

// tipTooltipHandler function to update the tooltip below a tipped message. either updates or creates initial tip tool tip
func tipTooltipHandler(m *tb.Message, bot *TipBot, amount int, notInitializedWallet bool) (hasTip bool) {
	// todo: this crashes if the tooltip message (maybe also the original tipped message) was deleted in the mean time!!! need to check for existence!
	hasTip, tipMessage := tipTooltipExists(m.ReplyTo.ID, bot)
	if hasTip {
		// update the tooltip with new tippers
		err := tipMessage.updateTooltip(bot, m.Sender, amount, notInitializedWallet)
		if err != nil {
			log.Println(err)
			// could not update the message (return false to )
			return false
		}
	} else {
		tipmsg := fmt.Sprintf("🏅 %d sat", amount)
		userStr := GetUserStrMd(m.Sender)
		tipmsg = fmt.Sprintf("%s (by %s)", tipmsg, userStr)

		if notInitializedWallet {
			tipmsg = tipmsg + fmt.Sprintf("\n🗑 Chat with %s to manage your wallet.", GetUserStrMd(bot.telegram.Me))
		}
		msg, err := bot.telegram.Reply(m.ReplyTo, tipmsg, tb.Silent)
		if err != nil {
			print(err)
		}
		message := NewMessage(msg, TipAmount(amount), Tips(1))
		message.Tippers = appendUinqueUsersToSlice(message.Tippers, m.Sender)
		runtime.IgnoreError(bot.bunt.Set(message))
	}
	// first call will return false, every following call will return true
	return hasTip
}
