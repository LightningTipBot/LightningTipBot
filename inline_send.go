package main

import (
	"fmt"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	inlineSendMessage             = "Press ✅ to receive payment.\n\n💸 Amount: %d sat"
	inlineSendAppendMemo          = "\n✉️ %s"
	sendInlineUpdateMessageAccept = "💸 %d sat sent from %s to %s."
	sendInlineCreateWalletMessage = "Chat with %s 👈 to manage your wallet."
	sendYourselfMessage           = "📖 You can't pay to yourself."
)

// tipTooltipExists checks if this tip is already known
func (bot *TipBot) getInlineSend(c *tb.Callback) (*InlineSend, error) {
	inlineSend := NewInlineSend("")
	inlineSend.ID = c.Data
	err := bot.bunt.Get(inlineSend)
	if err != nil {
		return nil, fmt.Errorf("could not get inline send message")
	}
	return inlineSend, nil

}

func (bot *TipBot) sendInlineHandler(c *tb.Callback) {
	inlineSend, err := bot.getInlineSend(c)
	if err != nil {
		log.Errorf("[sendInlineHandler] %s", err)
		return
	}

	amount := inlineSend.Amount
	to := c.Sender
	from := inlineSend.From

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

	inlineSend.Message = fmt.Sprintf("%s", fmt.Sprintf(sendInlineUpdateMessageAccept, amount, fromUserStrMd, toUserStrMd))
	memo := inlineSend.Memo
	if len(memo) > 0 {
		inlineSend.Message = inlineSend.Message + fmt.Sprintf(inlineSendAppendMemo, MarkdownEscape(memo))
	}

	if !bot.UserInitializedWallet(to) {
		inlineSend.Message += "\n\n" + fmt.Sprintf(sendInlineCreateWalletMessage, GetUserStrMd(bot.telegram.Me))
	}

	bot.tryEditMessage(c.Message, inlineSend.Message, &tb.ReplyMarkup{})

	// notify users
	_, err = bot.telegram.Send(from, fmt.Sprintf(tipSentMessage, amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[sendInline] Error: Send message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}

	// edit persistent object and store it
	inlineSend.To = to
	runtime.IgnoreError(bot.bunt.Set(inlineSend))

}

func (bot *TipBot) cancelSendInlineHandler(c *tb.Callback) {
	inlineSend, err := bot.getInlineSend(c)
	if err != nil {
		log.Errorf("[cancelSendInlineHandler] %s", err)
		return
	}
	if c.Sender.ID == inlineSend.From.ID {
		bot.tryEditMessage(c.Message, sendCancelledMessage, &tb.ReplyMarkup{})
	}
	return
}
