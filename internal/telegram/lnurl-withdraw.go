package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	lnurl "github.com/fiatjaf/go-lnurl"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	withdrawConfirmationMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelWithdraw        = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_withdraw")
	btnWithdraw              = paymentConfirmationMenu.Data("✅ Withdraw", "confirm_withdraw")
)

// LnurlWithdrawState saves the state of the user for an LNURL payment
type LnurlWithdrawState struct {
	*transaction.Base
	From                  *lnbits.User                `json:"from"`
	LNURLWithdrawResponse lnurl.LNURLWithdrawResponse `json:"LNURLWithdrawResponse"`
	LNURResponse          lnurl.LNURLResponse         `json:"LNURLResponse"`
	Amount                int                         `json:"amount"`
	Comment               string                      `json:"comment"`
	LanguageCode          string                      `json:"languagecode"`
	Success               bool                        `json:"success"`
	Invoice               lnbits.BitInvoice           `json:"invoice"`
	Message               string                      `json:"message"`
}

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
func (bot *TipBot) lnurlWithdrawHandler(ctx context.Context, m *tb.Message, withdrawParams LnurlWithdrawState) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}
	// object that holds all information about the send payment
	id := fmt.Sprintf("lnurlw-%d-%s", m.Sender.ID, RandStringRunes(5))
	LnurlWithdrawState := LnurlWithdrawState{
		Base:                  transaction.New(transaction.ID(id)),
		From:                  user,
		LNURLWithdrawResponse: withdrawParams.LNURLWithdrawResponse,
		LanguageCode:          ctx.Value("publicLanguageCode").(string),
	}

	// first we check whether an amount is present in the command
	amount, amount_err := decodeAmountFromCommand(m.Text)

	// amount is already present in the command, i.e., /lnurl <amount> <LNURL>
	// amount not in allowed range from LNURL
	if amount_err == nil &&
		(int64(amount) > (LnurlWithdrawState.LNURLWithdrawResponse.MaxWithdrawable/1000) || int64(amount) < (LnurlWithdrawState.LNURLWithdrawResponse.MinWithdrawable/1000)) &&
		(LnurlWithdrawState.LNURLWithdrawResponse.MaxWithdrawable != 0 && LnurlWithdrawState.LNURLWithdrawResponse.MinWithdrawable != 0) { // only if max and min are set
		err := fmt.Errorf("amount not in range")
		log.Warnf("[lnurlWithdrawHandler] Error: %s", err.Error())
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "lnurlInvalidAmountRangeMessage"), LnurlWithdrawState.LNURLWithdrawResponse.MinWithdrawable/1000, LnurlWithdrawState.LNURLWithdrawResponse.MaxWithdrawable/1000))
		ResetUserState(user, bot)
		return
	}

	// if no amount is entered, and if only one amount is possible, we use it
	if amount_err != nil && LnurlWithdrawState.LNURLWithdrawResponse.MaxWithdrawable == LnurlWithdrawState.LNURLWithdrawResponse.MinWithdrawable {
		amount = int(LnurlWithdrawState.LNURLWithdrawResponse.MaxWithdrawable / 1000)
		amount_err = nil
	}

	// set also amount in the state of the user
	LnurlWithdrawState.Amount = amount * 1000 // save as mSat

	// add result to persistent struct
	runtime.IgnoreError(LnurlWithdrawState.Set(LnurlWithdrawState, bot.Bunt))

	// now we actualy check whether the amount was in the command and if not, ask for it
	if amount_err != nil || amount < 1 {
		// // no amount was entered, set user state and ask for amount
		bot.askForAmount(ctx, id, "LnurlWithdrawState", LnurlWithdrawState.LNURLWithdrawResponse.MinWithdrawable, LnurlWithdrawState.LNURLWithdrawResponse.MaxWithdrawable, m.Text)
		return
	}

	// We need to save the pay state in the user state so we can load the payment in the next handler
	paramsJson, err := json.Marshal(LnurlWithdrawState)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandler] Error: %s", err.Error())
		// bot.trySendMessage(m.Sender, err.Error())
		return
	}
	SetUserState(user, bot, lnbits.UserHasEnteredAmount, string(paramsJson))
	// directly go to confirm
	bot.lnurlWithdrawHandlerWithdraw(ctx, m)
	return
}

// lnurlWithdrawHandlerWithdraw is invoked when the user has delivered an amount and is ready to pay
func (bot *TipBot) lnurlWithdrawHandlerWithdraw(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}
	statusMsg := bot.trySendMessage(m.Sender, Translate(ctx, "lnurlPreparingWithdraw"))

	// assert that user has entered an amount
	if user.StateKey != lnbits.UserHasEnteredAmount {
		log.Errorln("[lnurlWithdrawHandlerWithdraw] state keys don't match")
		bot.tryEditMessage(statusMsg, Translate(ctx, "errorTryLaterMessage"))
		return
	}

	// read the enter amount state from user.StateData
	var enterAmountData EnterAmountStateData
	err := json.Unmarshal([]byte(user.StateData), &enterAmountData)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		bot.tryEditMessage(statusMsg, Translate(ctx, "errorTryLaterMessage"))
		return
	}

	// use the enter amount state of the user to load the LNURL payment state
	tx := &LnurlWithdrawState{Base: transaction.New(transaction.ID(enterAmountData.ID))}
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		bot.tryEditMessage(statusMsg, Translate(ctx, "errorTryLaterMessage"))
		return
	}
	var lnurlWithdrawState *LnurlWithdrawState
	switch fn.(type) {
	case *LnurlWithdrawState:
		lnurlWithdrawState = fn.(*LnurlWithdrawState)
	default:
		log.Errorf("[lnurlWithdrawHandlerWithdraw] invalid type")
		bot.tryEditMessage(statusMsg, Translate(ctx, "errorTryLaterMessage"))
		return
	}

	confirmText := fmt.Sprintf(Translate(ctx, "confirmLnurlWithdrawMessage"), lnurlWithdrawState.Amount/1000)
	if len(lnurlWithdrawState.LNURLWithdrawResponse.DefaultDescription) > 0 {
		confirmText = confirmText + fmt.Sprintf(Translate(ctx, "confirmPayAppendMemo"), str.MarkdownEscape(lnurlWithdrawState.LNURLWithdrawResponse.DefaultDescription))
	}
	lnurlWithdrawState.Message = confirmText

	// create inline buttons
	withdrawButton := paymentConfirmationMenu.Data(Translate(ctx, "withdrawButtonMessage"), "confirm_withdraw")
	btnCancelWithdraw := paymentConfirmationMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_withdraw")
	withdrawButton.Data = lnurlWithdrawState.ID
	btnCancelWithdraw.Data = lnurlWithdrawState.ID

	withdrawConfirmationMenu.Inline(
		withdrawConfirmationMenu.Row(
			withdrawButton,
			btnCancelWithdraw),
	)

	bot.tryEditMessage(statusMsg, confirmText, withdrawConfirmationMenu)

	// // add response to persistent struct
	// lnurlWithdrawState.LNURResponse = response2
	runtime.IgnoreError(lnurlWithdrawState.Set(lnurlWithdrawState, bot.Bunt))
}

// confirmPayHandler when user clicked pay on payment confirmation
func (bot *TipBot) confirmWithdrawHandler(ctx context.Context, c *tb.Callback) {
	tx := &LnurlWithdrawState{Base: transaction.New(transaction.ID(c.Data))}
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[confirmWithdrawHandler] Error: %s", err.Error())
		return
	}
	var lnurlWithdrawState *LnurlWithdrawState
	switch fn.(type) {
	case *LnurlWithdrawState:
		lnurlWithdrawState = fn.(*LnurlWithdrawState)
	default:
		log.Errorf("[confirmWithdrawHandler] invalid type")
		return
	}
	// onnly the correct user can press
	if lnurlWithdrawState.From.Telegram.ID != c.Sender.ID {
		return
	}
	// immediatelly set intransaction to block duplicate calls
	err = lnurlWithdrawState.Lock(lnurlWithdrawState, bot.Bunt)
	if err != nil {
		log.Errorf("[confirmWithdrawHandler] %s", err)
		bot.tryDeleteMessage(c.Message)
		bot.tryEditMessage(c.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"), &tb.ReplyMarkup{})
		return
	}
	if !lnurlWithdrawState.Active {
		log.Errorf("[confirmPayHandler] send not active anymore")
		bot.tryEditMessage(c.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"), &tb.ReplyMarkup{})
		bot.tryDeleteMessage(c.Message)
		return
	}
	defer lnurlWithdrawState.Release(lnurlWithdrawState, bot.Bunt)

	user := LoadUser(ctx)
	if user.Wallet == nil {
		bot.tryDeleteMessage(c.Message)
		return
	}

	// reset state immediately
	ResetUserState(user, bot)

	// update button text
	bot.editSingleButton(ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "lnurlPreparingWithdraw"))

	callbackUrl, err := url.Parse(lnurlWithdrawState.LNURLWithdrawResponse.Callback)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		// bot.trySendMessage(c.Sender, Translate(ctx, "errorTryLaterMessage"))
		bot.editSingleButton(ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"))
		return
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
		errmsg := fmt.Sprintf("[lnurlWithdrawHandlerWithdraw] Could not create an invoice: %s", err)
		log.Errorln(errmsg)
		bot.editSingleButton(ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"))
		return
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
		// bot.trySendMessage(c.Sender, Translate(ctx, "errorTryLaterMessage"))
		bot.editSingleButton(ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"))
		return
	}
	res, err := client.Get(callbackUrl.String())
	if err != nil || res.StatusCode >= 300 {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Failed.")
		// bot.trySendMessage(c.Sender, Translate(ctx, "errorTryLaterMessage"))
		bot.editSingleButton(ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"))
		return
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		// bot.trySendMessage(c.Sender, Translate(ctx, "errorTryLaterMessage"))
		bot.editSingleButton(ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "errorTryLaterMessage"))
		return
	}

	// parse the response
	var response2 lnurl.LNURLResponse
	json.Unmarshal(body, &response2)
	if response2.Status == "OK" {
		// update button text
		bot.editSingleButton(ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "lnurlWithdrawSuccess"))

	} else {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] LNURLWithdraw failed.")
		// update button text
		bot.editSingleButton(ctx, c.Message, lnurlWithdrawState.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "lnurlWithdrawFailed"))
		return
	}

	// add response to persistent struct
	lnurlWithdrawState.LNURResponse = response2
	runtime.IgnoreError(lnurlWithdrawState.Set(lnurlWithdrawState, bot.Bunt))

}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot *TipBot) cancelWithdrawHandler(ctx context.Context, c *tb.Callback) {
	// reset state immediately
	user := LoadUser(ctx)
	ResetUserState(user, bot)
	tx := &LnurlWithdrawState{Base: transaction.New(transaction.ID(c.Data))}
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[cancelWithdrawHandler] Error: %s", err.Error())
		return
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
		return
	}
	bot.tryEditMessage(c.Message, i18n.Translate(lnurlWithdrawState.LanguageCode, "lnurlWithdrawCancelled"), &tb.ReplyMarkup{})
	lnurlWithdrawState.InTransaction = false
	lnurlWithdrawState.Inactivate(lnurlWithdrawState, bot.Bunt)
}
