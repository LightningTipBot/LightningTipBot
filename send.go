package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	sendValidAmountMessage     = "Did you enter a valid amount?"
	sendUserNotFoundMessage    = "User %s could not be found. You can /send only to Telegram tags like @%s."
	sendIsNotAUsser            = "🚫 %s is not a username. You can /send only to Telegram tags like @%s."
	sendUserHasNoWalletMessage = "🚫 User %s hasn't created a wallet yet."
	sendSentMessage            = "💸 %d sat sent to %s."
	sendReceivedMessage        = "🏅 %s sent you %d sat."
	sendErrorMessage           = "🚫 Send failed."
	confirmSendInvoiceMessage  = "Do you want to pay to %s?\n\n💸 Amount: %d sat"
	confirmSendAppendMemo      = "\n✉️ %s"
	sendCancelledMessage       = "🚫 Send cancelled."
	errorTryLaterMessage       = "🚫 Error. Please try again later."
	sendHelpText               = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/send <amount> <user> [<memo>]`\n" +
		"*Example:* `/send 1000 @LightningTipBot I just like the bot ❤️`\n" +
		"*Example:* `/send 1234 LightningTipBot@ln.tips`"
)

func helpSendUsage(errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(sendHelpText, fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(sendHelpText, "")
	}
}

func (bot *TipBot) SendCheckSyntax(m *tb.Message) (bool, string) {
	arguments := strings.Split(m.Text, " ")
	if len(arguments) < 2 {
		return false, fmt.Sprintf("Did you enter an amount and a recipient? You can use the /send command to either send to Telegram users like @%s or to a Lightning address like LightningTipBot@ln.tips.", bot.telegram.Me.Username)
	}
	// if len(arguments) < 3 {
	// 	return false, "Did you enter a recipient?"
	// }
	// if !strings.HasPrefix(arguments[0], "/send") {
	// 	return false, "Did you enter a valid command?"
	// }
	return true, ""
}

// confirmPaymentHandler invoked on "/send 123 @user" command
func (bot *TipBot) confirmSendHandler(ctx context.Context, m *tb.Message) {
	// reset state immediately
	user := LoadUser(ctx)
	ResetUserState(user, *bot)

	// check and print all commands
	bot.anyTextHandler(ctx, m)
	// If the send is a reply, then trigger /tip handler
	if m.IsReply() {
		bot.tipHandler(ctx, m)
		return
	}

	if ok, errstr := bot.SendCheckSyntax(m); !ok {
		bot.trySendMessage(m.Sender, helpSendUsage(errstr))
		NewMessage(m, WithDuration(0, bot.telegram))
		return
	}

	// get send amount, returns 0 if no amount is given
	amount, err := decodeAmountFromCommand(m.Text)
	// info: /send 10 <user> DEMANDS an amount, while /send <ln@address.com> also works without
	// todo: /send <user> should also invoke amount input dialog if no amount is given

	// CHECK whether first or second argument is a LIGHTNING ADDRESS
	arg := ""
	if len(strings.Split(m.Text, " ")) > 2 {
		arg, err = getArgumentFromCommand(m.Text, 2)
	} else if len(strings.Split(m.Text, " ")) == 2 {
		arg, err = getArgumentFromCommand(m.Text, 1)
	}
	if err == nil {
		if lightning.IsLightningAddress(arg) {
			// lightning address, send to that address
			err = bot.sendToLightningAddress(ctx, m, arg, amount)
			if err != nil {
				log.Errorln(err.Error())
				return
			}
			return
		}
	}

	// todo: this error might have been overwritten by the functions above
	// we should only check for a valid amount here, instead of error and amount

	// ASSUME INTERNAL SEND TO TELEGRAM USER
	if err != nil || amount < 1 {
		errmsg := fmt.Sprintf("[/send] Error: Send amount not valid.")
		log.Errorln(errmsg)
		// immediately delete if the amount is bullshit
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, helpSendUsage(sendValidAmountMessage))
		return
	}

	// SEND COMMAND IS VALID
	// check for memo in command
	sendMemo := GetMemoFromCommand(m.Text, 3)

	if len(m.Entities) < 2 {
		arg, err := getArgumentFromCommand(m.Text, 2)
		if err != nil {
			log.Errorln(err.Error())
			return
		}
		arg = MarkdownEscape(arg)
		NewMessage(m, WithDuration(0, bot.telegram))
		errmsg := fmt.Sprintf("Error: User %s could not be found", arg)
		bot.trySendMessage(m.Sender, helpSendUsage(fmt.Sprintf(sendUserNotFoundMessage, arg, bot.telegram.Me.Username)))
		log.Errorln(errmsg)

		return
	}
	if m.Entities[1].Type != "mention" {
		arg, err := getArgumentFromCommand(m.Text, 2)
		if err != nil {
			NewMessage(m, WithDuration(0, bot.telegram))
			log.Errorln(err.Error())
			return
		}
		arg = MarkdownEscape(arg)
		NewMessage(m, WithDuration(0, bot.telegram))
		errmsg := fmt.Sprintf("Error: %s is not a user", arg)
		bot.trySendMessage(m.Sender, fmt.Sprintf(sendIsNotAUsser, arg, bot.telegram.Me.Username))
		log.Errorln(errmsg)
		return
	}

	toUserStrMention := m.Text[m.Entities[1].Offset : m.Entities[1].Offset+m.Entities[1].Length]
	toUserStrWithoutAt := strings.TrimPrefix(toUserStrMention, "@")

	err = bot.parseCmdDonHandler(ctx, m)
	if err == nil {
		return
	}

	toUserDb := &lnbits.User{}
	tx := bot.database.Where("telegram_username = ?", strings.ToLower(toUserStrWithoutAt)).First(toUserDb)
	if tx.Error != nil || toUserDb.Wallet == nil || toUserDb.Initialized == false {
		NewMessage(m, WithDuration(0, bot.telegram))
		err = fmt.Errorf(sendUserHasNoWalletMessage, MarkdownEscape(toUserStrMention))
		bot.trySendMessage(m.Sender, err.Error())
		if tx.Error != nil {
			log.Printf("[/send] Error: %v %v", err, tx.Error)
			return
		}
		log.Printf("[/send] Error: %v", err)
		return
	}
	// string that holds all information about the send payment
	sendData := strconv.Itoa(toUserDb.Telegram.ID) + "|" + toUserStrWithoutAt + "|" +
		strconv.Itoa(amount)
	if len(sendMemo) > 0 {
		sendData = sendData + "|" + sendMemo
	}

	// save the send data to the database
	log.Debug(sendData)

	SetUserState(user, *bot, lnbits.UserStateConfirmSend, sendData)

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
func (bot *TipBot) cancelSendHandler(ctx context.Context, c *tb.Callback) {
	// reset state immediately
	user := LoadUser(ctx)
	ResetUserState(user, *bot)

	// delete the confirmation message
	err := bot.telegram.Delete(c.Message)
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
func (bot *TipBot) sendHandler(ctx context.Context, c *tb.Callback) {
	// remove buttons from confirmation message
	_, err := bot.telegram.Edit(c.Message, MarkdownEscape(c.Message.Text), &tb.ReplyMarkup{})
	if err != nil {
		log.Errorln("[sendHandler] " + err.Error())
	}
	// decode callback data
	// log.Debug("[sendHandler] Callback: %s", c.Data)
	from := LoadUser(ctx)
	if from.StateKey != lnbits.UserStateConfirmSend {
		log.Errorf("[sendHandler] User StateKey does not match! User: %d: StateKey: %d", c.Sender.ID, from.StateKey)
		return
	}

	// decode StateData in which we have information about the send payment
	splits := strings.Split(from.StateData, "|")
	if len(splits) < 3 {
		log.Error("[sendHandler] Not enough arguments in callback data")
		log.Errorf("user.StateData: %s", from.StateData)
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
	ResetUserState(from, *bot)

	// we can now get the wallets of both users
	to, err := GetUser(&tb.User{ID: toId, Username: toUserStrWithoutAt}, *bot)
	if err != nil {
		log.Errorln(err.Error())
		return
	}
	toUserStrMd := GetUserStrMd(to.Telegram)
	fromUserStrMd := GetUserStrMd(from.Telegram)
	toUserStr := GetUserStr(to.Telegram)
	fromUserStr := GetUserStr(from.Telegram)

	transactionMemo := fmt.Sprintf("Send from %s to %s (%d sat).", fromUserStr, toUserStr, amount)
	t := NewTransaction(bot, from, to, amount, TransactionType("send"))
	t.Memo = transactionMemo

	success, err := t.Send()
	if !success || err != nil {
		// NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(c.Sender, sendErrorMessage)
		errmsg := fmt.Sprintf("[/send] Error: Transaction failed. %s", err)
		log.Errorln(errmsg)
		return
	}

	bot.trySendMessage(from.Telegram, fmt.Sprintf(sendSentMessage, amount, toUserStrMd))
	bot.trySendMessage(to.Telegram, fmt.Sprintf(sendReceivedMessage, fromUserStrMd, amount))
	// send memo if it was present
	if len(sendMemo) > 0 {
		bot.trySendMessage(to.Telegram, fmt.Sprintf("✉️ %s", MarkdownEscape(sendMemo)))
	}

	return
}
