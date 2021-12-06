package telegram

import (
	"fmt"
	"github.com/eko/gocache/store"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
	"time"
)

func (bot TipBot) tryForwardMessage(to tb.Recipient, what tb.Editable, options ...interface{}) (msg *tb.Message) {
	msg, err := bot.Telegram.Forward(to, what, options...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return
}
func (bot TipBot) trySendMessage(to tb.Recipient, what interface{}, options ...interface{}) (msg *tb.Message) {
	msg, err := bot.Telegram.Send(to, what, options...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return
}

func (bot TipBot) tryReplyMessage(to *tb.Message, what interface{}, options ...interface{}) (msg *tb.Message) {
	msg, err := bot.Telegram.Reply(to, what, options...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return
}

func (bot TipBot) tryEditMessage(to tb.Editable, what interface{}, options ...interface{}) (msg *tb.Message) {
	if rightsToDoAction(bot, msg, isAdminAndCanEdit) {
		var err error
		msg, err = bot.Telegram.Edit(to, what, options...)
		if err != nil {
			log.Warnln(err.Error())
		}
		return msg
	}
	return
}

func (bot TipBot) tryDeleteMessage(msg tb.Editable) {
	if rightsToDoAction(bot, msg, isAdminAndCanDelete) {
		err := bot.Telegram.Delete(msg)
		if err != nil {
			log.Warnln(err.Error())
		}
	}
}

func rightsToDoAction(bot TipBot, editable tb.Editable, action func(members []tb.ChatMember, me *tb.User) bool) bool {
	switch editable.(type) {
	case *tb.Message:
		message := editable.(*tb.Message)
		if message.Sender == bot.Telegram.Me {
			break
		}
		chat := message.Chat
		if chat.Type == tb.ChatSuperGroup || chat.Type == tb.ChatGroup {
			admins, err := bot.Cache.Get(fmt.Sprintf("admins-%d", chat.ID))
			if err != nil {
				admins, err = bot.Telegram.AdminsOf(message.Chat)
				if err != nil {
					log.Warnln(err.Error())
					return false
				}
				bot.Cache.Set(fmt.Sprintf("admins-%d", chat.ID), admins, &store.Options{Expiration: 5 * time.Minute})
			}
			if action(admins.([]tb.ChatMember), bot.Telegram.Me) {
				return true
			}
			return false
		}
	}
	return true
}

func isAdminAndCanDelete(members []tb.ChatMember, me *tb.User) bool {
	for _, admin := range members {
		if admin.User.ID == me.ID {
			return admin.CanDeleteMessages
		}
	}
	return false
}

func isAdminAndCanEdit(members []tb.ChatMember, me *tb.User) bool {
	for _, admin := range members {
		if admin.User.ID == me.ID {
			return admin.CanEditMessages
		}
	}
	return false
}
