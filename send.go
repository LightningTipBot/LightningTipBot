package main

import (
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	sendValidAmountMessage     = "Did you use a valid amount?"
	sendUserNotFoundMessage    = "User %s could not be found. You can /send only to Telegram tags like @%s."
	sendIsNotAUsser            = "🚫 %s is not a username. You can /send only to Telegram tags like @%s."
	sendUserHasNoWalletMessage = "🚫 User %s hasn't created a wallet yet."
	sendSentMessage            = "💸 %d sat sent to %s."
	sendReceivedMessage        = "🏅 %s has sent you %d sat."
	sendErrorMessage           = "🚫 Transaction failed: %s"
	confirmSendInvoiceMessage  = "Do you want to pay to %s?\n\n💸 Amount: %d sat"
	confirmSendAppendMemo      = "\n✉️ %s"
	sendCancelledMessage       = "🚫 Sending cancelled."
	errorTryLaterMessage       = "🚫 Internal error. Please try again later.."
	sendHelpText               = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/send <amount> <user> [<memo>]`\n" +
		"*Example:* `/send 1000 @LightningTipBot I just like the bot ❤️`"
)

func helpSendUsage(errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(sendHelpText, fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(sendHelpText, "")
	}
}

func SendCheckSyntax(m *tb.Message) (bool, string) {
	arguments := strings.Split(m.Text, " ")
	if len(arguments) < 2 {
		return false, "Did you enter an amount and a recipient?"
	}
	if len(arguments) < 3 {
		return false, "Did you enter a recipient?"
	}
	if !strings.HasPrefix(arguments[0], "/send") {
		return false, "Did you enter a valid command?"
	}
	return true, ""
}

// confirmPaymentHandler invoked on "/send 123 @user" command
func (bot *TipBot) confirmSendHandler(m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	// If the send is a reply, then trigger /tip handler
	if m.IsReply() {
		bot.tipHandler(m)
		return
	}

	if ok, err := SendCheckSyntax(m); !ok {
		bot.telegram.Send(m.Sender, helpSendUsage(err))
		NewMessage(m).Dispose(0, bot.telegram)
		return
	}

	// get send amount
	amount, err := decodeAmountFromCommand(m.Text)
	if err != nil || amount < 1 {
		errmsg := fmt.Sprintf("[/send] Error: Send amount not valid.")
		log.Errorln(errmsg)
		// immediately delete if the amount is bullshit
		NewMessage(m).Dispose(0, bot.telegram)
		bot.telegram.Send(m.Sender, helpSendUsage(sendValidAmountMessage))
		return
	}

	// SEND COMMAND IS VALID

	// check for memo in command
	sendMemo := ""
	if len(strings.Split(m.Text, " ")) > 3 {
		sendMemo = strings.SplitN(m.Text, " ", 4)[3]
		if len(sendMemo) > 200 {
			sendMemo = sendMemo[:200]
			sendMemo = sendMemo + "..."
		}
	}

	if len(m.Entities) < 2 {
		arg, err := getArgumentFromCommand(m.Text, 2)
		if err != nil {
			log.Errorln(err.Error())
			return
		}
		arg = MarkdownEscape(arg)
		NewMessage(m).Dispose(0, bot.telegram)
		errmsg := fmt.Sprintf("Error: User %s could not be found", arg)
		bot.telegram.Send(m.Sender, helpSendUsage(fmt.Sprintf(sendUserNotFoundMessage, arg, arg)))
		log.Errorln(errmsg)

		return
	}
	if m.Entities[1].Type != "mention" {
		arg, err := getArgumentFromCommand(m.Text, 2)
		if err != nil {
			NewMessage(m).Dispose(0, bot.telegram)
			log.Errorln(err.Error())
			return
		}
		arg = MarkdownEscape(arg)
		NewMessage(m).Dispose(0, bot.telegram)
		errmsg := fmt.Sprintf("Error: %s is not a user", arg)
		bot.telegram.Send(m.Sender, fmt.Sprintf(sendIsNotAUsser, arg, arg))
		log.Errorln(errmsg)
		return
	}

	toUserStrMention := m.Text[m.Entities[1].Offset : m.Entities[1].Offset+m.Entities[1].Length]
	toUserStrWithoutAt := strings.TrimPrefix(toUserStrMention, "@")

	toUserDb := &lnbits.User{}
	tx := bot.database.Where("telegram_username = ?", toUserStrWithoutAt).First(toUserDb)
	if tx.Error != nil || toUserDb.Wallet == nil || toUserDb.Initialized == false {
		NewMessage(m).Dispose(0, bot.telegram)
		errmsg := fmt.Sprintf(sendUserHasNoWalletMessage, MarkdownEscape(toUserStrMention))
		log.Println("[/send] Error: " + errmsg)
		bot.telegram.Send(m.Sender, errmsg)
		return
	}

	// string that holds all information about the send payment
	sendData := strconv.Itoa(toUserDb.Telegram.ID) + "|" + toUserStrWithoutAt + "|" +
		strconv.Itoa(amount)
	if len(sendMemo) > 0 {
		sendData = sendData + "|" + sendMemo
	}

	// old callback method
	// // this is the maximum length of what the callback supports
	// buttonMaxDataLength := 58
	// if len(btnSend.Data) > buttonMaxDataLength {
	// 	btnSend.Data = btnSend.Data[:buttonMaxDataLength]
	// }

	// save the send data to the database
	log.Debug(sendData)
	user, err := GetUser(m.Sender, *bot)
	if err != nil {
		NewMessage(m).Dispose(0, bot.telegram)
		log.Printf("[/send] Error: %s\n", err.Error())
		bot.telegram.Send(m.Sender, fmt.Sprint(errorTryLaterMessage))
		return
	}
	user.StateKey = lnbits.UserStateConfirmSend
	user.StateData = sendData
	err = UpdateUserRecord(user, *bot)
	if err != nil {
		log.Printf("[UpdateUserRecord] User: %s Error: %s", GetUserStr(m.Sender), err.Error())
		return
	}

	sendConfirmationMenu.Inline(sendConfirmationMenu.Row(btnSend, btnCancelSend))
	confirmText := fmt.Sprintf(confirmSendInvoiceMessage, MarkdownEscape(toUserStrMention), amount)
	if len(sendMemo) > 0 {
		confirmText = confirmText + fmt.Sprintf(confirmSendAppendMemo, MarkdownEscape(sendMemo))
	}
	_, err = bot.telegram.Send(m.Sender, confirmText, sendConfirmationMenu)
	if err != nil {
		log.Error("[confirmSendHandler]" + err.Error())
		return
	}
}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot *TipBot) cancelSendHandler(c *tb.Callback) {
	// reset state immediately
	user, err := GetUser(c.Sender, *bot)
	if err != nil {
		log.Errorln(err.Error())
		return
	}
	user.ResetState()
	err = UpdateUserRecord(user, *bot)
	if err != nil {
		log.Errorln(err.Error())
		return
	}
	// delete the confirmation message
	err = bot.telegram.Delete(c.Message)
	if err != nil {
		log.Errorln("[cancelSendHandler] " + err.Error())
	}
	// notify the user
	_, err = bot.telegram.Send(c.Sender, sendCancelledMessage)
	if err != nil {
		log.WithField("message", sendCancelledMessage).WithField("user", c.Sender.ID).Printf("[Send] %s", err.Error())
		return
	}
}

// sendHandler invoked when user clicked send on payment confirmation
func (bot *TipBot) sendHandler(c *tb.Callback) {
	// remove buttons from confirmation message
	_, err := bot.telegram.Edit(c.Message, MarkdownEscape(c.Message.Text), &tb.ReplyMarkup{})
	if err != nil {
		log.Errorln("[sendHandler] " + err.Error())
	}
	// decode callback data
	// log.Debug("[sendHandler] Callback: %s", c.Data)
	user, err := GetUser(c.Sender, *bot)
	if err != nil {
		log.Printf("[GetUser] User: %d: %s", c.Sender.ID, err.Error())
		return
	}
	if user.StateKey != lnbits.UserStateConfirmSend {
		log.Errorf("[sendHandler] User StateKey does not match! User: %d: StateKey: %d", c.Sender.ID, user.StateKey)
		return
	}

	// decode StateData in which we have information about the send payment
	splits := strings.Split(user.StateData, "|")
	if len(splits) < 3 {
		log.Error("[sendHandler] Not enough arguments in callback data")
		log.Errorf("user.StateData: %s", user.StateData)
		return
	}
	toId, err := strconv.Atoi(splits[0])
	if err != nil {
		log.Errorln("[sendHandler] " + err.Error())
	}
	toUserStrWithoutAt := splits[1]
	amount, err := strconv.Atoi(splits[2])
	if err != nil {
		log.Errorln("[sendHandler] " + err.Error())
	}
	sendMemo := ""
	if len(splits) > 3 {
		sendMemo = strings.Join(splits[3:], "|")
	}

	// reset state
	user.ResetState()
	err = UpdateUserRecord(user, *bot)
	if err != nil {
		log.Errorln(err.Error())
		return
	}
	// we can now get the wallets of both users
	to := &tb.User{ID: toId, Username: toUserStrWithoutAt}
	from := c.Sender
	toUserStrMd := GetUserStrMd(to)
	fromUserStrMd := GetUserStrMd(from)
	toUserStr := GetUserStr(to)
	fromUserStr := GetUserStr(from)

	transactionMemo := fmt.Sprintf("Send from %s to %s (%d sat).", fromUserStr, toUserStr, amount)
	t := NewTransaction(bot, from, to, amount, TransactionType("send"))
	t.Memo = transactionMemo

	success, err := t.Send()
	if !success || err != nil {
		// NewMessage(m).Dispose(0, bot.telegram)
		bot.telegram.Send(c.Sender, fmt.Sprintf(sendErrorMessage, err))
		errmsg := fmt.Sprintf("[/send] Error: Transaction failed. %s", err)
		log.Errorln(errmsg)
		return
	}

	bot.telegram.Send(from, fmt.Sprintf(sendSentMessage, amount, toUserStrMd))
	bot.telegram.Send(to, fmt.Sprintf(sendReceivedMessage, fromUserStrMd, amount))
	// send memo if it was present
	if len(sendMemo) > 0 {
		bot.telegram.Send(to, fmt.Sprintf("✉️ %s", MarkdownEscape(sendMemo)))
	}

	return
}
