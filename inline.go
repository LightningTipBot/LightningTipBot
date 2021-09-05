package main

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	sendInlineConfirmMessage      = "Send inline confirm"
	sendInlineCancelMessage       = "Send inline confirm"
	sendInlineUpdateMessageAccept = "%s claimed this payment."
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
	message.Message = fmt.Sprintf("%s\n%s", message, fmt.Sprintf(sendInlineUpdateMessageAccept, GetUserStr(c.Sender)))
	bot.tryEditMessage(c.Message, message.Message, &tb.ReplyMarkup{})

	to := c.Sender
	from := message.From

	if from.ID == to.ID {
		bot.trySendMessage(&from, tipYourselfMessage)
		return
	}

	toUserStrMd := GetUserStrMd(m.ReplyTo.Sender)
	fromUserStrMd := GetUserStrMd(from)
	toUserStr := GetUserStr(m.ReplyTo.Sender)
	fromUserStr := GetUserStr(from)

	if _, exists := bot.UserExists(to); !exists {
		log.Infof("[/tip] User %s has no wallet.", toUserStr)
		err = bot.CreateWalletForTelegramUser(to)
		if err != nil {
			errmsg := fmt.Errorf("[/tip] Error: Could not create wallet for %s", toUserStr)
			log.Errorln(errmsg)
			return
		}
	}

	// check for memo in command
	tipMemo := ""
	if len(strings.Split(m.Text, " ")) > 2 {
		tipMemo = strings.SplitN(m.Text, " ", 3)[2]
		if len(tipMemo) > 200 {
			tipMemo = tipMemo[:200]
			tipMemo = tipMemo + "..."
		}
	}

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("Tip from %s to %s (%d sat).", fromUserStr, toUserStr, amount)
	t := NewTransaction(bot, from, to, amount, TransactionType("tip"), TransactionChat(m.Chat))
	t.Memo = transactionMemo
	success, err := t.Send()
	if !success {
		NewMessage(m, WithDuration(0, bot.telegram))
		if err != nil {
			bot.trySendMessage(m.Sender, fmt.Sprintf(tipErrorMessage, err))
		} else {
			bot.trySendMessage(m.Sender, fmt.Sprintf(tipErrorMessage, tipUndefinedErrorMsg))
		}
		errMsg := fmt.Sprintf("[/tip] Transaction failed: %s", err)
		log.Errorln(errMsg)
		return
	}

	// update tooltip if necessary
	messageHasTip := tipTooltipHandler(m, bot, amount, bot.UserInitializedWallet(to))

	log.Infof("[tip] %d sat from %s to %s", amount, fromUserStr, toUserStr)

	// notify users
	_, err = bot.telegram.Send(from, fmt.Sprintf(tipSentMessage, amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[/tip] Error: Send message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}

}

func (bot *TipBot) cancelSendInlineHandler(c *tb.Callback) {
	bot.tryEditMessage(c.Message, &tb.ReplyMarkup{})
	bot.trySendMessage(c.Message.Chat, paymentCancelledMessage)
}
