package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

func (bot TipBot) anyTextHandler(ctx context.Context, m *tb.Message) {
	if m.Chat.Type != tb.ChatPrivate {
		return
	}

	// check if user is in Database, if not, initialize wallet
	user := LoadUser(ctx)
	if user.Wallet == nil || !user.Initialized {
		bot.startHandler(ctx, m)
		return
	}

	// could be an invoice
	anyText := strings.ToLower(m.Text)
	if lightning.IsInvoice(anyText) {
		m.Text = "/pay " + anyText
		bot.payHandler(ctx, m)
		return
	}
	if lightning.IsLnurl(anyText) {
		m.Text = "/lnurl " + anyText
		bot.lnurlHandler(ctx, m)
		return
	}

	// could be a LNURL
	// var lnurlregex = regexp.MustCompile(`.*?((lnurl)([0-9]{1,}[a-z0-9]+){1})`)

	// inputs asked for
	if user.StateKey == lnbits.UserStateLNURLEnterAmount || user.StateKey == lnbits.UserEnterAmount {
		bot.enterAmountHandler(ctx, m)
	}
	if user.StateKey == lnbits.UserEnterUser {
		bot.enterUserHandler(ctx, m)
	}

}

type EnterUserStateData struct {
	ID              string `json:"ID"`              // holds the ID of the tx object in bunt db
	Type            string `json:"Type"`            // holds type of the tx in bunt db (needed for type checking)
	Amount          int64  `json:"Amount"`          // holds the amount entered by the user mSat
	AmountMin       int64  `json:"AmountMin"`       // holds the minimum amount that needs to be entered mSat
	AmountMax       int64  `json:"AmountMax"`       // holds the maximum amount that needs to be entered mSat
	OiringalCommand string `json:"OiringalCommand"` // hold the originally entered command for evtl later use
}

func (bot *TipBot) askForUser(ctx context.Context, id string, eventType string, originalCommand string) (enterUserStateData *EnterUserStateData, err error) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}
	enterUserStateData = &EnterUserStateData{
		ID:              id,
		Type:            eventType,
		OiringalCommand: originalCommand,
	}
	// set LNURLPayParams in the state of the user
	stateDataJson, err := json.Marshal(enterUserStateData)
	if err != nil {
		log.Errorln(err)
		return
	}
	SetUserState(user, bot, lnbits.UserEnterUser, string(stateDataJson))
	// Let the user enter a user and return
	bot.trySendMessage(user.Telegram, Translate(ctx, "enterUserMessage"), tb.ForceReply)
	return
}

// enterAmountHandler is invoked in anyTextHandler when the user needs to enter an amount
// the amount is then stored as an entry in the user's stateKey in the user database
// any other handler that relies on this, needs to load the resulting amount from the database
func (bot *TipBot) enterUserHandler(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}

	if !(user.StateKey == lnbits.UserEnterUser) {
		ResetUserState(user, bot)
		return // errors.New("user state does not match"), 0
	}
	if len(m.Text) < 4 || strings.HasPrefix(m.Text, "/") {
		ResetUserState(user, bot)
		return
	}

	var EnterUserStateData EnterUserStateData
	err := json.Unmarshal([]byte(user.StateData), &EnterUserStateData)
	if err != nil {
		log.Errorf("[EnterUserHandler] %s", err.Error())
		ResetUserState(user, bot)
		return
	}

	userstr := m.Text

	// find out which type the object in bunt has waiting for an amount
	// we stored this in the EnterAmountStateData before
	switch EnterUserStateData.Type {
	case "CreateSendState":
		m.Text = fmt.Sprintf("/send %s", userstr)
		SetUserState(user, bot, lnbits.UserHasEnteredAmount, "")
		bot.sendHandler(ctx, m)
		return
	default:
		ResetUserState(user, bot)
		return
	}
}
