package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"net/url"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"github.com/imroc/req"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"

	lnurl "github.com/fiatjaf/go-lnurl"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/telebot.v3"
)

var (
	authConfirmationMenu = &tb.ReplyMarkup{ResizeKeyboard: true}
	btnCancelAuth        = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_login")
	btnAuth              = paymentConfirmationMenu.Data("✅ Login", "confirm_login")
)

type LnurlAuthState struct {
	*storage.Base
	From            *lnbits.User          `json:"from"`
	LNURLAuthParams lnurl.LNURLAuthParams `json:"LNURLAuthParams"`
	Comment         string                `json:"comment"`
	LanguageCode    string                `json:"languagecode"`
	Message         *tb.Message           `json:"message"`
}

// lnurlPayHandler1 is invoked when the first lnurl response was a lnurlpay response
// at this point, the user hans't necessarily entered an amount yet
func (bot *TipBot) lnurlAuthHandler(ctx context.Context, m *tb.Message, authParams *LnurlAuthState) (context.Context, error) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}
	// object that holds all information about the send payment
	id := fmt.Sprintf("lnurlauth-%d-%s", m.Sender.ID, RandStringRunes(5))
	authParams.Base = storage.New(storage.ID(id))
	authParams.From = user
	authParams.LanguageCode = ctx.Value("publicLanguageCode").(string)

	// // // create inline buttons
	btnAuth = paymentConfirmationMenu.Data(Translate(ctx, "loginButtonMessage"), "confirm_login", id)
	btnCancelAuth = paymentConfirmationMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_login", id)

	paymentConfirmationMenu.Inline(
		paymentConfirmationMenu.Row(
			btnAuth,
			btnCancelAuth),
	)
	authParams.Message = bot.trySendMessageEditable(m.Chat,
		fmt.Sprintf(Translate(ctx, "confirmLnurlAuthMessager"),
			authParams.LNURLAuthParams.CallbackURL.Host,
		),
		paymentConfirmationMenu,
	)

	// save to bunt
	runtime.IgnoreError(authParams.Set(authParams, bot.Bunt))
	return ctx, nil
}

func (bot *TipBot) confirmLnurlAuthHandler(ctx intercept.Context) (intercept.Context, error) {
	c := ctx.Callback()
	tx := &LnurlAuthState{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(ctx, tx.ID)
	defer mutex.UnlockWithContext(ctx, tx.ID)
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[confirmPayHandler] %s", err.Error())
		return ctx, err
	}
	lnurlAuthState := sn.(*LnurlAuthState)

	if !lnurlAuthState.Active {
		return ctx, fmt.Errorf("LnurlAuthData not active.")
	}

	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}

	// statusMsg := bot.trySendMessageEditable(c.Sender,
	// 	Translate(ctx, "lnurlResolvingUrlMessage"),
	// )
	bot.editSingleButton(ctx, c.Message, lnurlAuthState.Message.Text, Translate(ctx, "lnurlResolvingUrlMessage"))

	// from fiatjaf/go-lnurl
	p := lnurlAuthState.LNURLAuthParams
	key, sig, err := user.SignKeyAuth(p.Host, p.K1)
	if err != nil {
		return ctx, err
	}

	var sentsigres lnurl.LNURLResponse
	client, err := bot.GetHttpClient()
	if err != nil {
		return ctx, err
	}
	r := req.New()
	r.SetClient(client)
	res, err := r.Get(p.CallbackURL.String(), url.Values{"sig": {sig}, "key": {key}})
	if err != nil {
		return ctx, err
	}
	err = json.Unmarshal(res.Bytes(), &sentsigres)
	if err != nil {
		return ctx, err
	}
	if sentsigres.Status == "ERROR" {
		bot.tryEditMessage(c, fmt.Sprintf(Translate(ctx, "errorReasonMessage"), sentsigres.Reason))
		return ctx, err
	}
	bot.editSingleButton(ctx, c.Message, lnurlAuthState.Message.Text, Translate(ctx, "lnurlSuccessfulLogin"))
	return ctx, lnurlAuthState.Inactivate(lnurlAuthState, bot.Bunt)
}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot *TipBot) cancelLnurlAuthHandler(ctx intercept.Context) (intercept.Context, error) {
	c := ctx.Callback()
	tx := &LnurlAuthState{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(ctx, tx.ID)
	defer mutex.UnlockWithContext(ctx, tx.ID)
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[confirmPayHandler] %s", err.Error())
		return ctx, err
	}
	lnurlAuthState := sn.(*LnurlAuthState)

	// onnly the correct user can press
	if lnurlAuthState.From.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	// delete and send instead of edit for the keyboard to pop up after sending
	bot.tryEditMessage(c, i18n.Translate(lnurlAuthState.LanguageCode, "loginCancelledMessage"), &tb.ReplyMarkup{})
	// bot.tryEditMessage(c.Message, i18n.Translate(payData.LanguageCode, "paymentCancelledMessage"), &tb.ReplyMarkup{})
	return ctx, lnurlAuthState.Inactivate(lnurlAuthState, bot.Bunt)
}
