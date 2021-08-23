package main

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	tipDidYouReplyMessage = "Did you reply to a message to tip? To reply to a message, right-click -> Reply on your computer or swipe the message on your phone. If you want to send directly to another Telegram user, use the /send command."
	tipEnterAmountMessage = "Did you enter an amount?"
	tipValidAmountMessage = "Did you enter a valid amount?"
	tipYourselfMessage    = "📖 You can't tip yourself."
	tipSentMessage        = "💸 %d sat sent to %s."
	tipReceivedMessage    = "🏅 You've received a %d sat tip from %s."
	tipErrorMessage       = "🚫 Transaction failed: %s"
	tipHelpText           = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/tip <amount> [<memo>]`\n" +
		"*Example:* `/tip 1000 Dank meme!`"
)

func helpTipUsage(errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(tipHelpText, fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(tipHelpText, "")
	}
}

func TipCheckSyntax(m *tb.Message) (bool, string) {
	arguments := strings.Split(m.Text, " ")
	if len(arguments) < 2 {
		return false, tipEnterAmountMessage
	}
	return true, ""
}

func (bot *TipBot) tipHandler(m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	// only if message is a reply
	if !m.IsReply() {
		NewMessage(m).Dispose(0, bot.telegram)
		bot.telegram.Send(m.Sender, helpTipUsage(fmt.Sprintf(tipDidYouReplyMessage)))
		return
	}

	if ok, err := TipCheckSyntax(m); !ok {
		bot.telegram.Send(m.Sender, helpTipUsage(err))
		NewMessage(m).Dispose(0, bot.telegram)
		return
	}

	// get tip amount
	amount, err := decodeAmountFromCommand(m.Text)
	if err != nil || amount < 1 {
		errmsg := fmt.Sprintf("[/tip] Error: Tip amount not valid.")
		// immediately delete if the amount is bullshit
		NewMessage(m).Dispose(0, bot.telegram)
		bot.telegram.Send(m.Sender, helpTipUsage(tipValidAmountMessage))
		log.Errorln(errmsg)
		return
	}

	// TIP COMMAND IS VALID

	to := m.ReplyTo.Sender
	from := m.Sender

	if from.ID == to.ID {
		NewMessage(m).Dispose(0, bot.telegram)
		bot.telegram.Send(m.Sender, tipYourselfMessage)
		return
	}

	toUserStrMd := GetUserStrMd(m.ReplyTo.Sender)
	fromUserStrMd := GetUserStrMd(from)
	toUserStr := GetUserStr(m.ReplyTo.Sender)
	fromUserStr := GetUserStr(from)

	if !bot.UserHasWallet(to) {
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
		NewMessage(m).Dispose(0, bot.telegram)
		if err != nil {
			bot.telegram.Send(m.Sender, fmt.Sprintf(tipErrorMessage, err))
		} else {
			bot.telegram.Send(m.Sender, fmt.Sprintf(tipErrorMessage, "please try again later"))
		}
		errMsg := fmt.Sprintf("[/tip] Transaction failed: %s", err)
		log.Errorln(errMsg)
		return
	}

	// delete the tip message after a few seconds, this is default behaviour
	NewMessage(m).Dispose(time.Second*time.Duration(Configuration.MessageDisposeDuration), bot.telegram)

	// update tooltip if necessary
	messageHasTip := tipTooltipHandler(m, from, bot, amount, bot.UserInitializedWallet(to))

	log.Infof("[tip] %d sat from %s to %s", amount, fromUserStr, toUserStr)

	// notify users
	_, err = bot.telegram.Send(from, fmt.Sprintf(tipSentMessage, amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[/tip] Error: Send message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}

	// forward tipped message to user once
	if !messageHasTip {
		bot.telegram.Forward(to, m.ReplyTo, tb.Silent)
	}
	bot.telegram.Send(to, fmt.Sprintf(tipReceivedMessage, amount, fromUserStrMd))

	if len(tipMemo) > 0 {
		bot.telegram.Send(to, fmt.Sprintf("✉️ %s", MarkdownEscape(tipMemo)))
	}

	return
}
