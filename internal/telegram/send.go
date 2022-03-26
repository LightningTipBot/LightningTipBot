package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"

	"github.com/LightningTipBot/LightningTipBot/internal/str"
	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

var (
	sendConfirmationMenu = &tb.ReplyMarkup{ResizeKeyboard: true}
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
	*storage.Base
	From           *lnbits.User `json:"from"`
	ToTelegramId   int64        `json:"to_telegram_id"`
	ToTelegramUser string       `json:"to_telegram_user"`
	Memo           string       `json:"memo"`
	Message        string       `json:"message"`
	Amount         int64        `json:"amount"`
	LanguageCode   string       `json:"languagecode"`
}

// sendHandler invoked on "/send 123 @user" command
func (bot *TipBot) sendHandler(ctx intercept.Context) (intercept.Context, error) {
	bot.anyTextHandler(ctx)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}

	// reset state immediately
	ResetUserState(user, bot)

	// check and print all commands

	// If the send is a reply, then trigger /tip handler
	if ctx.Message().IsReply() && ctx.Message().Chat.Type != tb.ChatPrivate {
		return bot.tipHandler(ctx)

	}

	// if ok, errstr := bot.SendCheckSyntax(ctx, m); !ok {
	// 	bot.trySendMessage(m.Sender, helpSendUsage(ctx, errstr))
	// 	NewMessage(m, WithDuration(0, bot))
	// 	return
	// }

	// get send amount, returns 0 if no amount is given
	amount, err := decodeAmountFromCommand(ctx.Text())
	// info: /send 10 <user> DEMANDS an amount, while /send <ln@address.com> also works without
	// todo: /send <user> should also invoke amount input dialog if no amount is given

	// CHECK whether first or second argument is a LIGHTNING ADDRESS
	arg := ""
	if len(strings.Split(ctx.Message().Text, " ")) > 2 {
		arg, err = getArgumentFromCommand(ctx.Message().Text, 2)
	} else if len(strings.Split(ctx.Message().Text, " ")) == 2 {
		arg, err = getArgumentFromCommand(ctx.Message().Text, 1)
	}
	if err == nil {
		if lightning.IsLightningAddress(arg) {
			// lightning address, send to that address
			ctx, err = bot.sendToLightningAddress(ctx, arg, amount)
			if err != nil {
				log.Errorln(err.Error())
				return ctx, err
			}
			return ctx, err
		}
	}

	// is a user given?
	arg, err = getArgumentFromCommand(ctx.Message().Text, 1)
	if err != nil && ctx.Message().Chat.Type == tb.ChatPrivate {
		_, err = bot.askForUser(ctx, "", "CreateSendState", ctx.Message().Text)
		return ctx, err
	}

	// is an amount given?
	amount, err = decodeAmountFromCommand(ctx.Message().Text)
	if (err != nil || amount < 1) && ctx.Message().Chat.Type == tb.ChatPrivate {
		_, err = bot.askForAmount(ctx, "", "CreateSendState", 0, 0, ctx.Message().Text)
		return ctx, err
	}

	// ASSUME INTERNAL SEND TO TELEGRAM USER
	if err != nil || amount < 1 {
		errmsg := fmt.Sprintf("[/send] Error: Send amount not valid.")
		log.Warnln(errmsg)
		// immediately delete if the amount is bullshit
		NewMessage(ctx.Message(), WithDuration(0, bot))
		bot.trySendMessage(ctx.Sender(), helpSendUsage(ctx, Translate(ctx, "sendValidAmountMessage")))
		return ctx, err
	}

	// SEND COMMAND IS VALID
	// check for memo in command
	sendMemo := GetMemoFromCommand(ctx.Message().Text, 3)

	toUserStrMention := ""
	toUserStrWithoutAt := ""

	// check for user in command, accepts user mention or plain username without @
	if len(ctx.Message().Entities) > 1 && ctx.Message().Entities[1].Type == "mention" {
		toUserStrMention = ctx.Message().Text[ctx.Message().Entities[1].Offset : ctx.Message().Entities[1].Offset+ctx.Message().Entities[1].Length]
		toUserStrWithoutAt = strings.TrimPrefix(toUserStrMention, "@")
	} else {
		toUserStrWithoutAt, err = getArgumentFromCommand(ctx.Message().Text, 2)
		if err != nil {
			log.Errorln(err.Error())
			return ctx, err
		}
		toUserStrWithoutAt = strings.TrimPrefix(toUserStrWithoutAt, "@")
		toUserStrMention = "@" + toUserStrWithoutAt
	}

	err = bot.parseCmdDonHandler(ctx)
	if err == nil {
		return ctx, errors.Create(errors.InvalidSyntaxError)
	}

	toUserDb, err := GetUserByTelegramUsername(toUserStrWithoutAt, *bot)
	if err != nil {
		NewMessage(ctx.Message(), WithDuration(0, bot))
		// cut username if it's too long
		if len(toUserStrMention) > 100 {
			toUserStrMention = toUserStrMention[:100]
		}
		bot.trySendMessage(ctx.Message().Sender, fmt.Sprintf(Translate(ctx, "sendUserHasNoWalletMessage"), str.MarkdownEscape(toUserStrMention)))
		return ctx, err
	}

	if user.ID == toUserDb.ID {
		bot.trySendMessage(ctx.Message().Sender, Translate(ctx, "sendYourselfMessage"))
		return ctx, errors.Create(errors.SelfPaymentError)
	}

	// entire text of the inline object
	confirmText := fmt.Sprintf(Translate(ctx, "confirmSendMessage"), str.MarkdownEscape(toUserStrMention), amount)
	if len(sendMemo) > 0 {
		confirmText = confirmText + fmt.Sprintf(Translate(ctx, "confirmSendAppendMemo"), str.MarkdownEscape(sendMemo))
	}
	// object that holds all information about the send payment
	id := fmt.Sprintf("send-%d-%d-%s", ctx.Message().Sender.ID, amount, RandStringRunes(5))
	sendData := &SendData{
		From:           user,
		Base:           storage.New(storage.ID(id)),
		Amount:         int64(amount),
		ToTelegramId:   toUserDb.Telegram.ID,
		ToTelegramUser: toUserStrWithoutAt,
		Memo:           sendMemo,
		Message:        confirmText,
		LanguageCode:   ctx.Value("publicLanguageCode").(string),
	}
	// save persistent struct
	runtime.IgnoreError(sendData.Set(sendData, bot.Bunt))

	sendDataJson, err := json.Marshal(sendData)
	if err != nil {
		NewMessage(ctx.Message(), WithDuration(0, bot))
		log.Printf("[/send] Error: %s\n", err.Error())
		bot.trySendMessage(ctx.Message().Sender, fmt.Sprint(Translate(ctx, "errorTryLaterMessage")))
		return ctx, err
	}
	// save the send data to the Database
	// log.Debug(sendData)
	SetUserState(user, bot, lnbits.UserStateConfirmSend, string(sendDataJson))
	sendButton := sendConfirmationMenu.Data(Translate(ctx, "sendButtonMessage"), "confirm_send")
	cancelButton := sendConfirmationMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_send")
	sendButton.Data = id
	cancelButton.Data = id

	sendConfirmationMenu.Inline(
		sendConfirmationMenu.Row(
			sendButton,
			cancelButton),
	)
	if ctx.Message().Private() {
		bot.trySendMessage(ctx.Chat(), confirmText, sendConfirmationMenu)
	} else {
		bot.tryReplyMessage(ctx.Message(), confirmText, sendConfirmationMenu)
	}
	return ctx, nil
}

// keyboardSendHandler will be called when the user presses the Send button on the keyboard
// it will pop up a new keyboard with the last interacted contacts to send funds to
// then, the flow is handled as if the user entered /send (then ask for contacts from keyboard or entry,
// then ask for an amount).
func (bot *TipBot) keyboardSendHandler(ctx intercept.Context) (intercept.Context, error) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}
	enterUserStateData := &EnterUserStateData{
		ID:              "id",
		Type:            "CreateSendState",
		OiringalCommand: "/send",
	}
	// set LNURLPayParams in the state of the user
	stateDataJson, err := json.Marshal(enterUserStateData)
	if err != nil {
		log.Errorln(err)
		return ctx, err
	}
	SetUserState(user, bot, lnbits.UserEnterUser, string(stateDataJson))
	sendToButtons = bot.makeContactsButtons(ctx)

	// if no contact is found (one entry will always be inside, it's the enter user button)
	// immediatelly go to the send handler
	if len(sendToButtons) == 1 {
		ctx.Message().Text = "/send"
		return bot.sendHandler(ctx)
	}

	// Attention! We need to ues the original Telegram.Send command here!
	// bot.trySendMessage will replace the keyboard with the default one and we want to send a different keyboard here
	// this is suboptimal because Telegram.Send is not rate limited etc. but it's the only way to send a custom keyboard for now
	_, err = bot.Telegram.Send(user.Telegram, Translate(ctx, "enterUserMessage"), sendToMenu)
	if err != nil {
		log.Errorln(err.Error())
	}
	return ctx, nil
}

// sendHandler invoked when user clicked send on payment confirmation
func (bot *TipBot) confirmSendHandler(ctx intercept.Context) (intercept.Context, error) {
	tx := &SendData{Base: storage.New(storage.ID(ctx.Data()))}
	mutex.LockWithContext(ctx, tx.ID)
	defer mutex.UnlockWithContext(ctx, tx.ID)
	sn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err.Error())
		return ctx, err
	}
	sendData := sn.(*SendData)
	// onnly the correct user can press
	if sendData.From.Telegram.ID != ctx.Callback().Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	if !sendData.Active {
		log.Errorf("[acceptSendHandler] send not active anymore")
		// bot.tryDeleteMessage(c.Message)
		return ctx, errors.Create(errors.NotActiveError)
	}
	defer sendData.Set(sendData, bot.Bunt)

	// // remove buttons from confirmation message
	// bot.tryEditMessage(c.Message, MarkdownEscape(sendData.Message), &tb.ReplyMarkup{})

	// decode callback data
	// log.Debug("[send] Callback: %s", c.Data)
	from := LoadUser(ctx)
	ResetUserState(from, bot) // we don't need to check the statekey anymore like we did earlier

	// information about the send
	toId := sendData.ToTelegramId
	toUserStrWithoutAt := sendData.ToTelegramUser
	amount := sendData.Amount
	sendMemo := sendData.Memo

	// we can now get the wallets of both users
	to, err := GetLnbitsUser(&tb.User{ID: toId, Username: toUserStrWithoutAt}, *bot)
	if err != nil {
		log.Errorln(err.Error())
		bot.tryDeleteMessage(ctx.Callback().Message)
		return ctx, err
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
		// bot.trySendMessage(c.Sender, sendErrorMessage)
		errmsg := fmt.Sprintf("[/send] Error: Transaction failed. %s", err.Error())
		log.Errorln(errmsg)
		bot.tryEditMessage(ctx.Callback().Message, i18n.Translate(sendData.LanguageCode, "sendErrorMessage"), &tb.ReplyMarkup{})
		return ctx, errors.Create(errors.UnknownError)
	}
	sendData.Inactivate(sendData, bot.Bunt)

	log.Infof("[💸 send] Send from %s to %s (%d sat).", fromUserStr, toUserStr, amount)

	// notify to user
	bot.trySendMessage(to.Telegram, fmt.Sprintf(i18n.Translate(to.Telegram.LanguageCode, "sendReceivedMessage"), fromUserStrMd, amount))
	// bot.trySendMessage(from.Telegram, fmt.Sprintf(Translate(ctx, "sendSentMessage"), amount, toUserStrMd))
	if ctx.Callback().Message.Private() {
		// if the command was invoked in private chat
		// the edit below was cool, but we need to get rid of the replymarkup inline keyboard thingy for the main menu to pop up
		// bot.tryEditMessage(c.Message, fmt.Sprintf(i18n.Translate(sendData.LanguageCode, "sendSentMessage"), amount, toUserStrMd), &tb.ReplyMarkup{})
		bot.tryDeleteMessage(ctx.Callback().Message)
		bot.trySendMessage(ctx.Callback().Sender, fmt.Sprintf(i18n.Translate(sendData.LanguageCode, "sendSentMessage"), amount, toUserStrMd))
	} else {
		// if the command was invoked in group chat
		bot.trySendMessage(ctx.Callback().Sender, fmt.Sprintf(i18n.Translate(from.Telegram.LanguageCode, "sendSentMessage"), amount, toUserStrMd))
		bot.tryEditMessage(ctx.Callback().Message, fmt.Sprintf(i18n.Translate(sendData.LanguageCode, "sendPublicSentMessage"), amount, fromUserStrMd, toUserStrMd), &tb.ReplyMarkup{})
	}
	// send memo if it was present
	if len(sendMemo) > 0 {
		bot.trySendMessage(to.Telegram, fmt.Sprintf("✉️ %s", str.MarkdownEscape(sendMemo)))
	}

	return ctx, nil
}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot *TipBot) cancelSendHandler(ctx intercept.Context) (intercept.Context, error) {
	// reset state immediately
	c := ctx.Callback()
	user := LoadUser(ctx)
	ResetUserState(user, bot)
	tx := &SendData{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(ctx, tx.ID)
	defer mutex.UnlockWithContext(ctx, tx.ID)
	sn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err.Error())
		return ctx, err
	}

	sendData := sn.(*SendData)
	// onnly the correct user can press
	if sendData.From.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	// delete and send instead of edit for the keyboard to pop up after sending
	bot.tryDeleteMessage(c)
	bot.trySendMessage(c.Message.Chat, i18n.Translate(sendData.LanguageCode, "sendCancelledMessage"))
	sendData.Inactivate(sendData, bot.Bunt)
	return ctx, nil
}
