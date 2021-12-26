package telegram

import (
	"fmt"
	"strconv"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/rate"
	"github.com/eko/gocache/store"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

// appendMainMenu will check if to (recipient) ID is from private or group chat.
// this function will only add a keyboard if this is a private chat and no force reply.
func (bot *TipBot) appendMainMenu(to int64, recipient interface{}, options []interface{}) []interface{} {

	var user *lnbits.User
	var err error
	if user, err = getCachedUser(&tb.User{ID: to}, *bot); err != nil {
		user, err = GetLnbitsUser(&tb.User{ID: to}, *bot)
		if err != nil {
			return options
		}
		updateCachedUser(user, *bot)
	}
	if user.Wallet != nil {
		amount, err := bot.GetUserBalanceCached(user)
		if err == nil {
			log.Infof("[appendMainMenu] user %s balance %d sat", GetUserStr(user.Telegram), amount)
			CommandBalance := fmt.Sprintf("%s %d sat", CommandBalance, amount)
			btnBalanceMainMenu = mainMenu.Text(CommandBalance)
			mainMenu.Reply(
				mainMenu.Row(btnBalanceMainMenu),
				mainMenu.Row(btnInvoiceMainMenu, btnSendMainMenu, btnHelpMainMenu),
			)
		}
	}

	appendKeyboard := true
	for _, option := range options {
		if option == tb.ForceReply {
			appendKeyboard = false
		}
		switch option.(type) {
		case *tb.ReplyMarkup:
			appendKeyboard = false
			//option.(*tb.ReplyMarkup).ReplyKeyboard = mainMenu.ReplyKeyboard
			//if option.(*tb.ReplyMarkup).InlineKeyboard == nil {
			//	options = append(options[:i], options[i+1:]...)
			//}
		}
	}
	// to > 0 is private chats
	if to > 0 && appendKeyboard {
		options = append(options, mainMenu)
	}
	return options
}

// getChatIdFromRecipient will parse the recipient to int64
func (bot *TipBot) getChatIdFromRecipient(to tb.Recipient) (int64, error) {
	chatId, err := strconv.ParseInt(to.Recipient(), 10, 64)
	if err != nil {
		return 0, err
	}
	return chatId, nil
}

func (bot TipBot) tryForwardMessage(to tb.Recipient, what tb.Editable, options ...interface{}) (msg *tb.Message) {
	rate.CheckLimit(to)
	// ChatId is used for the keyboard
	chatId, err := bot.getChatIdFromRecipient(to)
	if err != nil {
		log.Errorf("[tryForwardMessage] error converting message recipient to int64: %v", err)
		return
	}
	msg, err = bot.Telegram.Forward(to, what, bot.appendMainMenu(chatId, to, options)...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return
}
func (bot TipBot) trySendMessage(to tb.Recipient, what interface{}, options ...interface{}) (msg *tb.Message) {
	rate.CheckLimit(to)
	// ChatId is used for the keyboard
	chatId, err := bot.getChatIdFromRecipient(to)
	if err != nil {
		log.Errorf("[trySendMessage] error converting message recipient to int64: %v", err)
		return
	}
	msg, err = bot.Telegram.Send(to, what, bot.appendMainMenu(chatId, to, options)...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return
}

func (bot TipBot) tryReplyMessage(to *tb.Message, what interface{}, options ...interface{}) (msg *tb.Message) {
	rate.CheckLimit(to)
	msg, err := bot.Telegram.Reply(to, what, bot.appendMainMenu(to.Chat.ID, to, options)...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return
}

func (bot TipBot) tryEditMessage(to tb.Editable, what interface{}, options ...interface{}) (msg *tb.Message) {
	rate.CheckLimit(to)
	var err error
	_, chatId := to.MessageSig()
	msg, err = bot.Telegram.Edit(to, what, bot.appendMainMenu(chatId, to, options)...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return

}

func (bot TipBot) tryDeleteMessage(msg tb.Editable) {
	if !allowedToPerformAction(bot, msg, isAdminAndCanDelete) {
		return
	}
	rate.CheckLimit(msg)
	err := bot.Telegram.Delete(msg)
	if err != nil {
		log.Warnln(err.Error())
	}
	return

}

// allowedToPerformAction will check if bot is allowed to perform an action on the tb.Editable.
// this function will persist the admins list in cache for 5 minutes.
// if no admins list is found for this group, bot will always fetch a fresh list.
func allowedToPerformAction(bot TipBot, editable tb.Editable, action func(members []tb.ChatMember, me *tb.User) bool) bool {
	switch editable.(type) {
	case *tb.Message:
		message := editable.(*tb.Message)
		if message.Sender.ID == bot.Telegram.Me.ID {
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

// isAdminAndCanDelete will check if me is in members list and allowed to delete messages
func isAdminAndCanDelete(members []tb.ChatMember, me *tb.User) bool {
	for _, admin := range members {
		if admin.User.ID == me.ID {
			return admin.CanDeleteMessages
		}
	}
	return false
}
