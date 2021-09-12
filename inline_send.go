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
	inlineSendFailedMessage       = "🚫 Send failed."
)

var (
	inlineQuerySendTitle        = "Send sats to a chat."
	inlineQuerySendDescription  = "Usage: @%s send <amount> [<memo>]"
	inlineResultSendTitle       = "💸 Send %d sat."
	inlineResultSendDescription = "👉 Click here to send %d sat to this chat."
	sendInlineMenu              = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelSendInline         = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_send_inline")
	btnSendInline               = paymentConfirmationMenu.Data("✅ Receive", "confirm_send_inline")
)

type InlineSend struct {
	Message string   `json:"inline_send_message"`
	Amount  int      `json:"inline_send_amount"`
	From    *tb.User `json:"inline_send_from"`
	To      *tb.User `json:"inline_send_to"`
	Memo    string
	ID      string `json:"inline_send_id"`
	Active  bool   `json:"inline_send_active"`
}

func NewInlineSend(m string) *InlineSend {
	inlineSend := &InlineSend{
		Message: m,
	}
	return inlineSend

}

func (msg InlineSend) Key() string {
	return msg.ID
}

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
	if !inlineSend.Active {
		log.Errorf("[sendInlineHandler] inline send not active anymore")
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
	transactionMemo := fmt.Sprintf("Send from %s to %s (%d sat).", fromUserStr, toUserStr, amount)
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
		bot.tryEditMessage(c.Message, inlineSendFailedMessage, &tb.ReplyMarkup{})
		return
	}

	log.Infof("[sendInline] %d sat from %s to %s", amount, fromUserStr, toUserStr)

	inlineSend.Message = fmt.Sprintf("%s", fmt.Sprintf(sendInlineUpdateMessageAccept, amount, fromUserStrMd, toUserStrMd))
	memo := inlineSend.Memo
	if len(memo) > 0 {
		inlineSend.Message = inlineSend.Message + fmt.Sprintf(inlineSendAppendMemo, memo)
	}

	if !bot.UserInitializedWallet(to) {
		inlineSend.Message += "\n\n" + fmt.Sprintf(sendInlineCreateWalletMessage, GetUserStrMd(bot.telegram.Me))
	}

	bot.tryEditMessage(c.Message, inlineSend.Message, &tb.ReplyMarkup{})
	inlineSend.Active = false
	// notify users
	_, err = bot.telegram.Send(to, fmt.Sprintf(sendReceivedMessage, fromUserStrMd, amount))
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
		// set the inlineSend inactive
		inlineSend.Active = false
		runtime.IgnoreError(bot.bunt.Set(inlineSend))
	}
	return
}
