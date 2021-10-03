package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/str"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"
	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
	"strings"
)

var (
	sendConfirmationMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelSend        = sendConfirmationMenu.Data("🚫 Cancel", "cancel_send")
	btnSend              = sendConfirmationMenu.Data("✅ Send", "confirm_send")
)

func helpSendUsage(ctx context.Context, errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(Translate(ctx, "sendHelpText"), fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(Translate(ctx, "sendHelpText"), "")
	}
}

func (bot *TipBot) SendCheckSyntax(ctx context.Context, m *tb.Message) (bool, string) {
	arguments := strings.Split(m.Text, " ")
	if len(arguments) < 2 {
		return false, fmt.Sprintf(Translate(ctx, "sendSyntaxErrorMessage"), GetUserStrMd(bot.Telegram.Me))
	}
	return true, ""
}

type SendData struct {
	*transaction.Base
	From           *lnbits.User `json:"from"`
	ToTelegramId   int          `json:"to_telegram_id"`
	ToTelegramUser string       `json:"to_telegram_user"`
	Memo           string       `json:"memo"`
	Message        string       `json:"message"`
	Amount         int64        `json:"amount"`
	LanguageCode   string       `json:"languagecode"`
}

func NewSend() *SendData {
	sendData := &SendData{
		Base: transaction.New(),
	}
	return sendData
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

	if ok, errstr := bot.SendCheckSyntax(ctx, m); !ok {
		bot.trySendMessage(m.Sender, helpSendUsage(ctx, errstr))
		NewMessage(m, WithDuration(0, bot.Telegram))
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
		NewMessage(m, WithDuration(0, bot.Telegram))
		bot.trySendMessage(m.Sender, helpSendUsage(ctx, Translate(ctx, "sendValidAmountMessage")))
		return
	}

	// SEND COMMAND IS VALID
	// check for memo in command
	sendMemo := GetMemoFromCommand(m.Text, 3)

	toUserStrMention := ""
	toUserStrWithoutAt := ""

	// check for user in command, accepts user mention or plan username without @
	if len(m.Entities) > 1 && m.Entities[1].Type == "mention" {
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
		NewMessage(m, WithDuration(0, bot.Telegram))
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "sendUserHasNoWalletMessage"), toUserStrMention))
		return
	}

	// entire text of the inline object
	confirmText := fmt.Sprintf(Translate(ctx, "confirmSendMessage"), str.MarkdownEscape(toUserStrMention), amount)
	if len(sendMemo) > 0 {
		confirmText = confirmText + fmt.Sprintf(Translate(ctx, "confirmSendAppendMemo"), str.MarkdownEscape(sendMemo))
	}
	// object that holds all information about the send payment
	id := fmt.Sprintf("send-%d-%d-%s", m.Sender.ID, amount, RandStringRunes(5))
	sendData := SendData{
		From:           user,
		Base:           transaction.New(transaction.ID(id)),
		Amount:         int64(amount),
		ToTelegramId:   toUserDb.Telegram.ID,
		ToTelegramUser: toUserStrWithoutAt,
		Memo:           sendMemo,
		Message:        confirmText,
		LanguageCode:   ctx.Value("publicLanguageCode").(string),
	}
	// save persistent struct
	runtime.IgnoreError(bot.Bunt.Set(sendData))

	sendDataJson, err := json.Marshal(sendData)
	if err != nil {
		NewMessage(m, WithDuration(0, bot.Telegram))
		log.Printf("[/send] Error: %s\n", err.Error())
		bot.trySendMessage(m.Sender, fmt.Sprint(Translate(ctx, "errorTryLaterMessage")))
		return
	}
	// save the send data to the Database
	// log.Debug(sendData)
	SetUserState(user, *bot, lnbits.UserStateConfirmSend, string(sendDataJson))
	sendButton := sendConfirmationMenu.Data(Translate(ctx, "sendButtonMessage"), "confirm_send")
	cancelButton := sendConfirmationMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_send")
	sendButton.Data = id
	cancelButton.Data = id

	sendConfirmationMenu.Inline(
		sendConfirmationMenu.Row(
			sendButton,
			cancelButton),
	)
	if m.Private() {
		bot.trySendMessage(m.Chat, confirmText, sendConfirmationMenu)
	} else {
		bot.tryReplyMessage(m, confirmText, sendConfirmationMenu)
	}
}

// sendHandler invoked when user clicked send on payment confirmation
func (bot *TipBot) confirmSendHandler(ctx context.Context, c *tb.Callback) {
	tx := NewSend()
	tx.ID = c.Data
	sn, err := transaction.Get(tx, tx.Base, bot.bunt)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err)
		return
	}
	sendData := sn.(*SendData)
	// onnly the correct user can press
	if sendData.From.Telegram.ID != c.Sender.ID {
		return
	}
	// immediatelly set intransaction to block duplicate calls
	err = transaction.Lock(sendData, sendData.Base, bot.bunt)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err)
		bot.tryDeleteMessage(c.Message)
		return
	}
	if !sendData.Active {
		log.Errorf("[acceptSendHandler] send not active anymore")
		bot.tryDeleteMessage(c.Message)
		return
	}
	defer transaction.Release(sendData, sendData.Base, bot.bunt)

	// // remove buttons from confirmation message
	// bot.tryEditMessage(c.Message, MarkdownEscape(sendData.Message), &tb.ReplyMarkup{})

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
	to, err := GetLnbitsUser(&tb.User{ID: toId, Username: toUserStrWithoutAt}, *bot)
	if err != nil {
		log.Errorln(err.Error())
		bot.tryDeleteMessage(c.Message)
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
		// bot.trySendMessage(c.Sender, sendErrorMessage)
		errmsg := fmt.Sprintf("[/send] Error: Transaction failed. %s", err)
		log.Errorln(errmsg)
		bot.tryEditMessage(c.Message, fmt.Sprintf("%s %s", i18n.Translate(sendData.LanguageCode, "sendErrorMessage"), err), &tb.ReplyMarkup{})
		return
	}

	log.Infof("[send] Transaction sent from %s to %s (%d sat).", fromUserStr, toUserStr, amount)

	sendData.InTransaction = false

	// notify to user
	bot.trySendMessage(to.Telegram, fmt.Sprintf(i18n.Translate(to.Telegram.LanguageCode, "sendReceivedMessage"), fromUserStrMd, amount))
	// bot.trySendMessage(from.Telegram, fmt.Sprintf(Translate(ctx, "sendSentMessage"), amount, toUserStrMd))
	if c.Message.Private() {
		// if the command was invoked in private chat
		bot.tryEditMessage(c.Message, fmt.Sprintf(i18n.Translate(sendData.LanguageCode, "sendSentMessage"), amount, toUserStrMd), &tb.ReplyMarkup{})
	} else {
		// if the command was invoked in group chat
		bot.trySendMessage(c.Sender, fmt.Sprintf(i18n.Translate(from.Telegram.LanguageCode, "sendSentMessage"), amount, toUserStrMd))
		bot.tryEditMessage(c.Message, fmt.Sprintf(i18n.Translate(sendData.LanguageCode, "sendPublicSentMessage"), amount, fromUserStrMd, toUserStrMd), &tb.ReplyMarkup{})
	}
	// send memo if it was present
	if len(sendMemo) > 0 {
		bot.trySendMessage(to.Telegram, fmt.Sprintf("✉️ %s", str.MarkdownEscape(sendMemo)))
	}

	return
}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot *TipBot) cancelSendHandler(ctx context.Context, c *tb.Callback) {
	// reset state immediately
	user := LoadUser(ctx)
	ResetUserState(user, *bot)
	tx := NewSend()
	tx.ID = c.Data
	sn, err := transaction.Get(tx, tx.Base, bot.bunt)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err)
		return
	}
	sendData := sn.(*SendData)
	// onnly the correct user can press
	if sendData.From.Telegram.ID != c.Sender.ID {
		return
	}
	// remove buttons from confirmation message
	bot.tryEditMessage(c.Message, i18n.Translate(sendData.LanguageCode, "sendCancelledMessage"), &tb.ReplyMarkup{})
	sendData.InTransaction = false
	transaction.Inactivate(sendData, sendData.Base, bot.bunt)
}
