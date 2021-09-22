package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	sendValidAmountMessage     = "Did you enter a valid amount?"
	sendUserHasNoWalletMessage = "🚫 User %s hasn't created a wallet yet."
	sendSentMessage            = "💸 %d sat sent to %s."
	sendReceivedMessage        = "🏅 %s sent you %d sat."
	sendErrorMessage           = "🚫 Send failed."
	confirmSendMessage         = "Do you want to pay to %s?\n\n💸 Amount: %d sat"
	confirmSendAppendMemo      = "\n✉️ %s"
	sendCancelledMessage       = "🚫 Send cancelled."
	errorTryLaterMessage       = "🚫 Error. Please try again later."
	sendHelpText               = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/send <amount> <user> [<memo>]`\n" +
		"*Example:* `/send 1000 @LightningTipBot I just like the bot ❤️`\n" +
		"*Example:* `/send 1234 LightningTipBot@ln.tips`"
)

var (
	sendConfirmationMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelSend        = sendConfirmationMenu.Data("🚫 Cancel", "cancel_send")
	btnSend              = sendConfirmationMenu.Data("✅ Send", "confirm_send")
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
		return false, fmt.Sprintf("Did you enter an amount and a recipient? You can use the /send command to either send to Telegram users like %s or to a Lightning address like LightningTipBot@ln.tips.", GetUserStrMd(bot.telegram.Me))
	}
	return true, ""
}

type SendData struct {
	ID             string       `json:"id"`
	From           *lnbits.User `json:"from"`
	ToTelegramId   int          `json:"to_telegram_id"`
	ToTelegramUser string       `json:"to_telegram_user"`
	Memo           string       `json:"memo"`
	Message        string       `json:"message"`
	Amount         int64        `json:"amount"`
	InTransaction  bool         `json:"intransaction"`
	Active         bool         `json:"active"`
}

func NewSend() *SendData {
	sendData := &SendData{
		Active:        true,
		InTransaction: false,
	}
	return sendData
}

func (msg SendData) Key() string {
	return msg.ID
}

func (bot *TipBot) LockSend(tx *SendData) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = true
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) ReleaseSend(tx *SendData) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = false
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) InactivateSend(tx *SendData) error {
	tx.Active = false
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) getSend(c *tb.Callback) (*SendData, error) {
	sendData := NewSend()
	sendData.ID = c.Data

	err := bot.bunt.Get(sendData)

	// to avoid race conditions, we block the call if there is
	// already an active transaction by loop until InTransaction is false
	ticker := time.NewTicker(time.Second * 10)

	for sendData.InTransaction {
		select {
		case <-ticker.C:
			return nil, fmt.Errorf("send timeout")
		default:
			log.Infoln("[send] in transaction")
			time.Sleep(time.Duration(500) * time.Millisecond)
			err = bot.bunt.Get(sendData)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get sendData")
	}

	return sendData, nil

}

// sendHandler invoked on "/send 123 @user" command
func (bot *TipBot) sendHandler(ctx context.Context, m *tb.Message) {
	bot.anyTextHandler(ctx, m)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}

	// reset state immediately
	ResetUserState(user, *bot)

	// check and print all commands

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

	toUserStrMention := ""
	toUserStrWithoutAt := ""

	// check for user in command, accepts user mention or plan username without @
	if len(m.Entities) > 1 && m.Entities[1].Type != "mention" {
		toUserStrMention = m.Text[m.Entities[1].Offset : m.Entities[1].Offset+m.Entities[1].Length]
		toUserStrWithoutAt = strings.TrimPrefix(toUserStrMention, "@")
	} else {
		toUserStrWithoutAt, err = getArgumentFromCommand(m.Text, 2)
		if err != nil {
			log.Errorln(err.Error())
			return
		}
		toUserStrMention = "@" + toUserStrWithoutAt
		toUserStrWithoutAt = strings.TrimPrefix(toUserStrWithoutAt, "@")
	}

	err = bot.parseCmdDonHandler(ctx, m)
	if err == nil {
		return
	}

	toUserDb, err := GetUserByTelegramUsername(toUserStrWithoutAt, *bot)
	if err != nil {
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, fmt.Sprintf(sendUserHasNoWalletMessage, toUserStrMention))
		return
	}

	// entire text of the inline object
	confirmText := fmt.Sprintf(confirmSendMessage, MarkdownEscape(toUserStrMention), amount)
	if len(sendMemo) > 0 {
		confirmText = confirmText + fmt.Sprintf(confirmSendAppendMemo, MarkdownEscape(sendMemo))
	}
	// object that holds all information about the send payment
	id := fmt.Sprintf("send-%d-%d-%s", m.Sender.ID, amount, RandStringRunes(5))
	sendData := SendData{
		From:           user,
		Active:         true,
		InTransaction:  false,
		ID:             id,
		Amount:         int64(amount),
		ToTelegramId:   toUserDb.Telegram.ID,
		ToTelegramUser: toUserStrWithoutAt,
		Memo:           sendMemo,
		Message:        confirmText,
	}
	// add result to persistent struct
	runtime.IgnoreError(bot.bunt.Set(sendData))

	sendDataJson, err := json.Marshal(sendData)
	if err != nil {
		NewMessage(m, WithDuration(0, bot.telegram))
		log.Printf("[/send] Error: %s\n", err.Error())
		bot.trySendMessage(m.Sender, fmt.Sprint(errorTryLaterMessage))
		return
	}
	// save the send data to the database
	// log.Debug(sendData)
	SetUserState(user, *bot, lnbits.UserStateConfirmSend, string(sendDataJson))

	btnSend.Data = id
	btnCancelSend.Data = id

	sendConfirmationMenu.Inline(sendConfirmationMenu.Row(btnSend, btnCancelSend))
	_, err = bot.telegram.Send(m.Sender, confirmText, sendConfirmationMenu)
	if err != nil {
		log.Error("[send]" + err.Error())
		return
	}
}

// sendHandler invoked when user clicked send on payment confirmation
func (bot *TipBot) confirmSendHandler(ctx context.Context, c *tb.Callback) {
	sendData, err := bot.getSend(c)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err)
		return
	}
	// immediatelly set intransaction to block duplicate calls
	err = bot.LockSend(sendData)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err)
		return
	}
	if !sendData.Active {
		log.Errorf("[acceptSendHandler] send not active anymore")
		return
	}
	defer bot.ReleaseSend(sendData)

	// remove buttons from confirmation message
	bot.tryEditMessage(c.Message, MarkdownEscape(sendData.Message), &tb.ReplyMarkup{})

	// decode callback data
	// log.Debug("[send] Callback: %s", c.Data)
	from := LoadUser(ctx)
	ResetUserState(from, *bot) // we don't need to check the statekey anymore like we did earlier

	// information about the send
	toId := sendData.ToTelegramId
	toUserStrWithoutAt := sendData.ToTelegramUser
	amount := sendData.Amount
	sendMemo := sendData.Memo

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
	t := NewTransaction(bot, from, to, int(amount), TransactionType("send"))
	t.Memo = transactionMemo

	success, err := t.Send()
	if !success || err != nil {
		bot.trySendMessage(c.Sender, sendErrorMessage)
		errmsg := fmt.Sprintf("[/send] Error: Transaction failed. %s", err)
		log.Errorln(errmsg)
		return
	}

	sendData.InTransaction = false
	bot.trySendMessage(from.Telegram, fmt.Sprintf(sendSentMessage, amount, toUserStrMd))
	bot.trySendMessage(to.Telegram, fmt.Sprintf(sendReceivedMessage, fromUserStrMd, amount))
	// send memo if it was present
	if len(sendMemo) > 0 {
		bot.trySendMessage(to.Telegram, fmt.Sprintf("✉️ %s", MarkdownEscape(sendMemo)))
	}

	return
}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot *TipBot) cancelSendHandler(ctx context.Context, c *tb.Callback) {
	// reset state immediately
	user := LoadUser(ctx)
	ResetUserState(user, *bot)
	sendData, err := bot.getSend(c)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err)
		return
	}

	// delete the confirmation message
	bot.tryDeleteMessage(c.Message)
	// notify the user
	bot.trySendMessage(c.Sender, sendCancelledMessage)

	// set the inlineSend inactive
	sendData.Active = false
	sendData.InTransaction = false
	runtime.IgnoreError(bot.bunt.Set(sendData))

}
