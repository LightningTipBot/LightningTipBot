package main

import (
	"fmt"

	tb "gopkg.in/tucnak/telebot.v2"
)

func (bot *TipBot) sendInlineHandler(c *tb.Callback) {
	fmt.Println("Pressed Send")
	bot.tryEditMessage(c.Message, c.Message.Text, &tb.ReplyMarkup{})
}

func (bot *TipBot) cancelSendInlineHandler(c *tb.Callback) {
	fmt.Println("Pressed Cancel")
	bot.tryEditMessage(c.Message, &tb.ReplyMarkup{})
	bot.tryDeleteMessage(c.Message)
	_, _ = bot.telegram.Send(c.Sender, paymentCancelledMessage)
}
