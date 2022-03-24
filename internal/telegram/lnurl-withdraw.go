package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"io/ioutil"
	"net/url"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"

	"github.com/LightningTipBot/LightningTipBot/internal/str"
	lnurl "github.com/fiatjaf/go-lnurl"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/telebot.v3"
)

var (
	withdrawConfirmationMenu = &tb.ReplyMarkup{ResizeKeyboard: true}
	btnCancelWithdraw        = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_withdraw")
	btnWithdraw              = paymentConfirmationMenu.Data("✅ Withdraw", "confirm_withdraw")
)

// LnurlWithdrawState saves the state of the user for an LNURL payment
type LnurlWithdrawState struct {
	*storage.Base
	From                  *lnbits.User                `json:"from"`
	LNURLWithdrawResponse lnurl.LNURLWithdrawResponse `json:"LNURLWithdrawResponse"`
	LNURResponse          lnurl.LNURLResponse         `json:"LNURLResponse"`
	Amount                int64                       `json:"amount"`
	Comment               string                      `json:"comment"`
	LanguageCode          string                      `json:"languagecode"`
	Success               bool                        `json:"success"`
	Invoice               lnbits.BitInvoice           `json:"invoice"`
	Message               string                      `json:"message"`
}

// editSingleButton edits a message to display a single button (for something like a progress indicator)
func (bot *TipBot) editSingleButton(ctx context.Context, m *tb.Message, message string, button string) {
	bot.tryEditMessage(
		m,
		message,
		&tb.ReplyMarkup{
			InlineKeyboard: [][]tb.InlineButton{
				{tb.InlineButton{Text: button}},
			},
		},
	)
}

// lnurlWithdrawHandler is invoked when the first lnurl response was a lnurl-withdraw response
// at this point, the user hans't necessarily entered an amount yet
func (bot *TipBot) lnurlWithdrawHandler(handler intercept.Handler, withdrawParams *LnurlWithdrawState) {
	m := handler.Message()
	user := LoadUser(handler.Ctx)
	if user.Wallet == nil {
		return
	}
	// object that holds all information about the send payment
	id := fmt.Sprintf("lnurlw-%d-%s", m.Sender.ID, RandStringRunes(5))

	withdrawParams.Base = storage.New(storage.ID(id))
	withdrawParams.From = user
	withdrawParams.LanguageCode = handler.Ctx.Value("publicLanguageCode").(string)

	// first we check whether an amount is present in the command
	amount, amount_err := decodeAmountFromCommand(m.Text)

	// amount is already present in the command, i.e., /lnurl <amount> <LNURL>
	// amount not in allowed range from LNURL
	if amount_err == nil &&
		(int64(amount) > (withdrawParams.LNURLWithdrawResponse.MaxWithdrawable/1000) || int64(amount) < (withdrawParams.LNURLWithdrawResponse.MinWithdrawable/1000)) &&
		(withdrawParams.LNURLWithdrawResponse.MaxWithdrawable != 0 && withdrawParams.LNURLWithdrawResponse.MinWithdrawable != 0) { // only if max and min are set
		err := fmt.Errorf("amount not in range")
		log.Warnf("[lnurlWithdrawHandler] Error: %s", err.Error())
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(handler.Ctx, "lnurlInvalidAmountRangeMessage"), withdrawParams.LNURLWithdrawResponse.MinWithdrawable/1000, withdrawParams.LNURLWithdrawResponse.MaxWithdrawable/1000))
		ResetUserState(user, bot)
		return
	}

	// if no amount is entered, and if only one amount is possible, we use it
	if amount_err != nil && withdrawParams.LNURLWithdrawResponse.MaxWithdrawable == withdrawParams.LNURLWithdrawResponse.MinWithdrawable {
		amount = int64(withdrawParams.LNURLWithdrawResponse.MaxWithdrawable / 1000)
		amount_err = nil
	}

	// set also amount in the state of the user
	withdrawParams.Amount = amount * 1000 // save as mSat

	// add result to persistent struct
	runtime.IgnoreError(withdrawParams.Set(withdrawParams, bot.Bunt))

	// now we actualy check whether the amount was in the command and if not, ask for it
	if amount_err != nil || amount < 1 {
		// // no amount was entered, set user state and ask for amount
		bot.askForAmount(handler.Ctx, id, "LnurlWithdrawState", withdrawParams.LNURLWithdrawResponse.MinWithdrawable, withdrawParams.LNURLWithdrawResponse.MaxWithdrawable, m.Text)
		return
	}

	// We need to save the pay state in the user state so we can load the payment in the next handler
	paramsJson, err := json.Marshal(withdrawParams)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandler] Error: %s", err.Error())
		// bot.trySendMessage(m.Sender, err.Error())
		return
	}
	SetUserState(user, bot, lnbits.UserHasEnteredAmount, string(paramsJson))
	// directly go to confirm
	bot.lnurlWithdrawHandlerWithdraw(handler)
	return
}

// lnurlWithdrawHandlerWithdraw is invoked when the user has delivered an amount and is ready to pay
func (bot *TipBot) lnurlWithdrawHandlerWithdraw(handler intercept.Handler) (intercept.Handler, error) {
	m := handler.Message()
	user := LoadUser(handler.Ctx)
	if user.Wallet == nil {
		return handler, errors.Create(errors.UserNoWalletError)
	}
	statusMsg := bot.trySendMessageEditable(m.Sender, Translate(handler.Ctx, "lnurlPreparingWithdraw"))

	// assert that user has entered an amount
	if user.StateKey != lnbits.UserHasEnteredAmount {
		log.Errorln("[lnurlWithdrawHandlerWithdraw] state keys don't match")
		bot.tryEditMessage(statusMsg, Translate(handler.Ctx, "errorTryLaterMessage"))
		return handler, fmt.Errorf("wrong state key")
	}

	// read the enter amount state from user.StateData
	var enterAmountData EnterAmountStateData
	err := json.Unmarshal([]byte(user.StateData), &enterAmountData)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		bot.tryEditMessage(statusMsg, Translate(handler.Ctx, "errorTryLaterMessage"))
		return handler, err
	}

	// use the enter amount state of the user to load the LNURL payment state
	tx := &LnurlWithdrawState{Base: storage.New(storage.ID(enterAmountData.ID))}
	mutex.LockWithContext(handler.Ctx, tx.ID)
	defer mutex.UnlockWithContext(handler.Ctx, tx.ID)
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		bot.tryEditMessage(statusMsg, Translate(handler.Ctx, "errorTryLaterMessage"))
		return handler, err
	}
	var lnurlWithdrawState *LnurlWithdrawState
	switch fn.(type) {
	case *LnurlWithdrawState:
		lnurlWithdrawState = fn.(*LnurlWithdrawState)
	default:
		log.Errorf("[lnurlWithdrawHandlerWithdraw] invalid type")
		bot.tryEditMessage(statusMsg, Translate(handler.Ctx, "errorTryLaterMessage"))
		return handler, fmt.Errorf("invalid type")
	}

	confirmText := fmt.Sprintf(Translate(handler.Ctx, "confirmLnurlWithdrawMessage"), lnurlWithdrawState.Amount/1000)
	if len(lnurlWithdrawState.LNURLWithdrawResponse.DefaultDescription) > 0 {
		confirmText = confirmText + fmt.Sprintf(Translate(handler.Ctx, "confirmPayAppendMemo"), str.MarkdownEscape(lnurlWithdrawState.LNURLWithdrawResponse.DefaultDescription))
	}
	lnurlWithdrawState.Message = confirmText

	// create inline buttons
	withdrawButton := paymentConfirmationMenu.Data(Translate(handler.Ctx, "withdrawButtonMessage"), "confirm_withdraw", lnurlWithdrawState.ID)
	btnCancelWithdraw := paymentConfirmationMenu.Data(Translate(handler.Ctx, "cancelButtonMessage"), "cancel_withdraw", lnurlWithdrawState.ID)

	withdrawConfirmationMenu.Inline(
		withdrawConfirmationMenu.Row(
			withdrawButton,
			btnCancelWithdraw),
	)

	bot.tryEditMessage(statusMsg, confirmText, withdrawConfirmationMenu)

	// // add response to persistent struct
	// lnurlWithdrawState.LNURResponse = response2
	runtime.IgnoreError(lnurlWithdrawState.Set(lnurlWithdrawState, bot.Bunt))
	return handler, nil
}

// confirmPayHandler when user clicked pay on payment confirmation
func (bot *TipBot) confirmWithdrawHandler(handler intercept.Handler) (intercept.Handler, error) {
	c := handler.Callback()
	tx := &LnurlWithdrawState{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(handler.Ctx, tx.ID)
	defer mutex.UnlockWithContext(handler.Ctx, tx.ID)
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[confirmWithdrawHandler] Error: %s", err.Error())
		return handler, err
	}

	var lnurlWithdrawState *LnurlWithdrawState
	switch fn.(type) {
	case *LnurlWithdrawState:
		lnurlWithdrawState = fn.(*LnurlWithdrawState)
	default:
		log.Errorf("[confirmWithdrawHandler] invalid type")
		return handler, errors.Create(errors.InvalidTypeError)
	}
	// onnly the correct user can press
	if lnurlWithdrawState.From.Telegram.ID != c.Sender.ID {
		return handler, errors.Create(errors.UnknownError)
	}
	if !lnurlWithdrawState.Active {
		log.Errorf("[confirmPayHandler] send not active anymore")
		bot.tryEditMessage(c.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"), &tb.ReplyMarkup{})
		bot.tryDeleteMessage(c.Message)
		return handler, errors.Create(errors.NotActiveError)
	}
	defer lnurlWithdrawState.Set(lnurlWithdrawState, bot.Bunt)

	user := LoadUser(handler.Ctx)
	if user.Wallet == nil {
		bot.tryDeleteMessage(c.Message)
		return handler, errors.Create(errors.UserNoWalletError)
	}

	// reset state immediately
	ResetUserState(user, bot)

	// update button text
	bot.editSingleButton(handler.Ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "lnurlPreparingWithdraw"))

	callbackUrl, err := url.Parse(lnurlWithdrawState.LNURLWithdrawResponse.Callback)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		// bot.trySendMessage(c.Sender, Translate(handler.Ctx, "errorTryLaterMessage"))
		bot.editSingleButton(handler.Ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"))
		return handler, err
	}

	// generate an invoice and add the pr to the request
	// generate invoice
	invoice, err := user.Wallet.Invoice(
		lnbits.InvoiceParams{
			Out:     false,
			Amount:  int64(lnurlWithdrawState.Amount) / 1000,
			Memo:    "Withdraw",
			Webhook: internal.Configuration.Lnbits.WebhookServer},
		bot.Client)
	if err != nil {
		errmsg := fmt.Sprintf("[lnurlWithdrawHandlerWithdraw] Could not create an invoice: %s", err.Error())
		log.Errorln(errmsg)
		bot.editSingleButton(handler.Ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"))
		return handler, err
	}
	lnurlWithdrawState.Invoice = invoice

	qs := callbackUrl.Query()
	// add amount to query string
	qs.Set("pr", invoice.PaymentRequest)
	qs.Set("k1", lnurlWithdrawState.LNURLWithdrawResponse.K1)
	callbackUrl.RawQuery = qs.Encode()

	// lnurlWithdrawState loaded
	client, err := bot.GetHttpClient()
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		// bot.trySendMessage(c.Sender, Translate(handler.Ctx, "errorTryLaterMessage"))
		bot.editSingleButton(handler.Ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"))
		return handler, err
	}
	res, err := client.Get(callbackUrl.String())
	if err != nil || res.StatusCode >= 300 {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Failed.")
		// bot.trySendMessage(c.Sender, Translate(handler.Ctx, "errorTryLaterMessage"))
		bot.editSingleButton(handler.Ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"))
		return handler, errors.New(errors.UnknownError, err)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		// bot.trySendMessage(c.Sender, Translate(handler.Ctx, "errorTryLaterMessage"))
		bot.editSingleButton(handler.Ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"))
		return handler, err
	}

	// parse the response
	var response2 lnurl.LNURLResponse
	json.Unmarshal(body, &response2)
	if response2.Status == "OK" {
		// update button text
		bot.editSingleButton(handler.Ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "lnurlWithdrawSuccess"))

	} else {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] LNURLWithdraw failed.")
		// update button text
		bot.editSingleButton(handler.Ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "lnurlWithdrawFailed"))
		return handler, errors.New(errors.UnknownError, fmt.Errorf("LNURLWithdraw failed"))
	}

	// add response to persistent struct
	lnurlWithdrawState.LNURResponse = response2
	return handler, lnurlWithdrawState.Set(lnurlWithdrawState, bot.Bunt)

}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot *TipBot) cancelWithdrawHandler(handler intercept.Handler) (intercept.Handler, error) {
	c := handler.Callback()
	// reset state immediately
	user := LoadUser(handler.Ctx)
	ResetUserState(user, bot)
	tx := &LnurlWithdrawState{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(handler.Ctx, tx.ID)
	defer mutex.UnlockWithContext(handler.Ctx, tx.ID)
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[cancelWithdrawHandler] Error: %s", err.Error())
		return handler, err
	}
	var lnurlWithdrawState *LnurlWithdrawState
	switch fn.(type) {
	case *LnurlWithdrawState:
		lnurlWithdrawState = fn.(*LnurlWithdrawState)
	default:
		log.Errorf("[cancelWithdrawHandler] invalid type")
	}
	// onnly the correct user can press
	if lnurlWithdrawState.From.Telegram.ID != c.Sender.ID {
		return handler, errors.Create(errors.UnknownError)
	}
	bot.tryEditMessage(c.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "lnurlWithdrawCancelled"), &tb.ReplyMarkup{})
	return handler, lnurlWithdrawState.Inactivate(lnurlWithdrawState, bot.Bunt)
}
