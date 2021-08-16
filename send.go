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
	sendIsNotAUsser            = "%s is not a user"
	sendUserHasNoWalletMessage = "🚫 User %s hasn't created a wallet yet."
	sendSentMessage            = "💸 %d sat sent to %s."
	sendReceivedMessage        = "🏅 %s has sent you %d sat."
	sendErrorMessage           = "🚫 Transaction failed: %s"
	confirmSendInvoiceMessage  = "Do you want to send this to %s?\n💸 Amount: %d sat\n✉️ %s"
	sendCancelledMessage       = "🚫 Sending cancelled."
	sendHelpText               = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/send <amount> <user> [<memo>]`\n" +
		"*Example:* `/send 1000 @LightningTipBot I just like the bot ❤️`"
)

func helpSendUsage(errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(sendHelpText, fmt.Sprintf("_%s_", errormsg))
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

type SendCallbackData struct {
	ToID               int
	ToUserStrWithoutAt string
	Amount             int
	Memo               string
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
	amount, err := DecodeAmountFromCommand(m.Text)
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
			return
		}
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
			return
		}
		NewMessage(m).Dispose(0, bot.telegram)
		errmsg := fmt.Sprintf("Error: %s is not a user", arg)
		bot.telegram.Send(m.Sender, fmt.Sprintf(sendIsNotAUsser, arg))
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

	// build json that will be sent on button press
	// callbackData := SendCallbackData{ToID: toUserDb.Telegram.ID, ToUserStrWithoutAt: toUserStrWithoutAt, Amount: amount, Memo: sendMemo}
	// bs, _ := json.Marshal(callbackData)
	btnSend.Data = strconv.Itoa(toUserDb.Telegram.ID) + "|" +
		toUserStrWithoutAt + "|" +
		strconv.Itoa(amount) + "|" +
		sendMemo

	sendConfirmationMenu.Inline(sendConfirmationMenu.Row(btnSend, btnCancelSend))
	bot.telegram.Send(m.Sender,
		fmt.Sprintf(confirmSendInvoiceMessage, MarkdownEscape(toUserStrMention), amount, MarkdownEscape(sendMemo)),
		sendConfirmationMenu)
}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot *TipBot) cancelSendHandler(c *tb.Callback) {
	defer func() {
		bot.telegram.Delete(c.Message)
	}()
	_, err := bot.telegram.Send(c.Sender, sendCancelledMessage)
	if err != nil {
		log.WithField("message", sendCancelledMessage).WithField("user", c.Sender.ID).Printf("[Send] %s", err.Error())
		return
	}

}

// sendHandler invoked when user clicked send on payment confirmation
func (bot *TipBot) sendHandler(c *tb.Callback) {
	defer func() {
		bot.telegram.Edit(c.Message, c.Message.Text, &tb.ReplyMarkup{})
	}()
	// // If the send is a reply, then trigger /tip handler
	// if m.IsReply() {
	// 	bot.tipHandler(m)
	// 	return
	// }

	// if ok, err := SendCheckSyntax(m); !ok {
	// 	bot.telegram.Send(m.Sender, helpSendUsage(err))
	// 	NewMessage(m).Dispose(0, bot.telegram)
	// 	return
	// }

	// // get send amount
	// amount, err := DecodeAmountFromCommand(m.Text)
	// if err != nil || amount < 1 {
	// 	errmsg := fmt.Sprintf("[/send] Error: Send amount not valid.")
	// 	log.Errorln(errmsg)
	// 	// immediately delete if the amount is bullshit
	// 	NewMessage(m).Dispose(0, bot.telegram)
	// 	bot.telegram.Send(m.Sender, helpSendUsage(sendValidAmountMessage))
	// 	return
	// }

	// // SEND COMMAND IS VALID

	// // check for memo in command
	// sendMemo := ""
	// if len(strings.Split(m.Text, " ")) > 3 {
	// 	sendMemo = strings.SplitN(m.Text, " ", 4)[3]
	// 	if len(sendMemo) > 200 {
	// 		sendMemo = sendMemo[:200]
	// 		sendMemo = sendMemo + "..."
	// 	}
	// }

	// get *to* user
	// user must be mentioned in telegram as @username
	// if len(m.Entities) < 2 {
	// 	arg, err := getArgumentFromCommand(m.Text, 2)
	// 	if err != nil {
	// 		return
	// 	}
	// 	NewMessage(m).Dispose(0, bot.telegram)
	// 	errmsg := fmt.Sprintf("Error: User %s could not be found", arg)
	// 	bot.telegram.Send(m.Sender, helpSendUsage(fmt.Sprintf(sendUserNotFoundMessage, arg, arg)))
	// 	log.Errorln(errmsg)

	// 	return
	// }
	// if m.Entities[1].Type != "mention" {
	// 	arg, err := getArgumentFromCommand(m.Text, 2)
	// 	if err != nil {
	// 		NewMessage(m).Dispose(0, bot.telegram)
	// 		return
	// 	}
	// 	NewMessage(m).Dispose(0, bot.telegram)
	// 	errmsg := fmt.Sprintf("Error: %s is not a user", arg)
	// 	bot.telegram.Send(m.Sender, fmt.Sprintf(sendIsNotAUsser, arg))
	// 	log.Errorln(errmsg)
	// 	return
	// }

	// toUserStrMention := m.Text[m.Entities[1].Offset : m.Entities[1].Offset+m.Entities[1].Length]
	// toUserStrWithoutAt := strings.TrimPrefix(toUserStrMention, "@")

	// toUserDb := &lnbits.User{}
	// tx := bot.database.Where("telegram_username = ?", toUserStrWithoutAt).First(toUserDb)
	// if tx.Error != nil || toUserDb.Wallet == nil || toUserDb.Initialized == false {
	// 	NewMessage(m).Dispose(0, bot.telegram)
	// 	errmsg := fmt.Sprintf(sendUserHasNoWalletMessage, MarkdownEscape(toUserStrMention))
	// 	log.Println("[/send] Error: " + errmsg)
	// 	bot.telegram.Send(m.Sender, errmsg)
	// 	return
	// }

	log.Infof("Callback: %s", c.Data)
	splits := strings.Split(c.Data, "|")
	if len(splits) < 3 {
		log.Error("[sendHandler] Not enough arguments in callback data")
		return
	}
	toId, err := strconv.Atoi(splits[0])
	toUserStrWithoutAt := splits[1]
	amount, err := strconv.Atoi(splits[2])
	sendMemo := ""
	if len(splits) > 3 {
		sendMemo = strings.Join(splits[3:], "|")
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
	if !success {
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
		bot.telegram.Send(to, fmt.Sprintf("✉️ %s", sendMemo))
	}
	return
}
