package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	sendInlineConfirmMessage      = "Send inline confirm"
	sendInlineCancelMessage       = "Send inline confirm"
	sendInlineUpdateMessageAccept = "💸 %d sat send from %s to %s."
	sendInlineCreateWalletMessage = "Chat with %s 👈 to manage your wallet."
	sendYourselfMessage           = "📖 You can't pay to yourself."
)

// tipTooltipExists checks if this tip is already known
func (bot *TipBot) getInlineSend(c *tb.Callback) (*InlineSend, error) {
	message := NewInlineSend("")
	message.ID = c.Data
	err := bot.bunt.Get(message)
	if err != nil {
		return nil, fmt.Errorf("could not get inline send message")
	}
	return message, nil

}

func (bot *TipBot) sendInlineHandler(c *tb.Callback) {
	message, err := bot.getInlineSend(c)
	if err != nil {
		log.Errorf("[getInlineSendMessageOfCallback] %s", err)
		return
	}

	amount := message.Amount
	to := c.Sender
	from := message.From

	if from.ID == to.ID {
		bot.trySendMessage(from, sendYourselfMessage)
		return
	}

	toUserStrMd := GetUserStrMd(to)
	fromUserStrMd := GetUserStrMd(from)
	toUserStr := GetUserStr(to)
	fromUserStr := GetUserStr(from)

	// check if user exists and create a wallet if not
	_, exists := bot.UserExists(to)
	if !exists {
		log.Infof("[sendInline] User %s has no wallet.", toUserStr)
		err = bot.CreateWalletForTelegramUser(to)
		if err != nil {
			errmsg := fmt.Errorf("[sendInline] Error: Could not create wallet for %s", toUserStr)
			log.Errorln(errmsg)
			return
		}
	}

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("Tip from %s to %s (%d sat).", fromUserStr, toUserStr, amount)
	t := NewTransaction(bot, from, to, amount, TransactionType("inline send"))
	t.Memo = transactionMemo
	success, err := t.Send()
	if !success {
		if err != nil {
			bot.trySendMessage(from, fmt.Sprintf(tipErrorMessage, err))
		} else {
			bot.trySendMessage(from, fmt.Sprintf(tipErrorMessage, tipUndefinedErrorMsg))
		}
		errMsg := fmt.Sprintf("[sendInline] Transaction failed: %s", err)
		log.Errorln(errMsg)
		return
	}

	log.Infof("[sendInline] %d sat from %s to %s", amount, fromUserStr, toUserStr)

	message.Message = fmt.Sprintf("%s", fmt.Sprintf(sendInlineUpdateMessageAccept, amount, fromUserStrMd, toUserStrMd))

	if !exists {
		message.Message += " " + fmt.Sprintf(sendInlineCreateWalletMessage, GetUserStrMd(bot.telegram.Me))
	}

	bot.tryEditMessage(c.Message, message.Message, &tb.ReplyMarkup{})

	// notify users
	_, err = bot.telegram.Send(from, fmt.Sprintf(tipSentMessage, amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[sendInline] Error: Send message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}

}

func (bot *TipBot) cancelSendInlineHandler(c *tb.Callback) {
	bot.tryEditMessage(c.Message, &tb.ReplyMarkup{})
	bot.trySendMessage(c.Message.Chat, paymentCancelledMessage)
}
