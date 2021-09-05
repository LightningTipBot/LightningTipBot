package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	sendInlineConfirmMessage = "Send inline confirm"
	sendInlineCancelMessage  = "Send inline confirm"
)

// tipTooltipExists checks if this tip is already known
func (bot *TipBot) getInlineSendMessageOfCallback(c *tb.Callback) (string, error) {
	message := NewInlineSend("")
	message.ID = c.Data
	err := bot.bunt.Get(message)
	if err != nil {
		return "", fmt.Errorf("could not get inline send message")
	}
	return message.Message, nil

}

func (bot *TipBot) sendInlineHandler(c *tb.Callback) {
	message, err := bot.getInlineSendMessageOfCallback(c)
	if err != nil {
		log.Errorf("[getInlineSendMessageOfCallback] %s", err)
		return
	}
	bot.tryEditMessage(c.Message, message, &tb.ReplyMarkup{})
	bot.trySendMessage(c.Sender, fmt.Sprintf("Pressed Send by user %s", GetUserStr(c.Sender)))
}

func (bot *TipBot) cancelSendInlineHandler(c *tb.Callback) {
	bot.tryEditMessage(c.Message, &tb.ReplyMarkup{})
	bot.trySendMessage(c.Message.Chat, paymentCancelledMessage)
}
