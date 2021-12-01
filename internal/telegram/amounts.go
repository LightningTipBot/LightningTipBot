package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/price"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

func getArgumentFromCommand(input string, which int) (output string, err error) {
	if len(strings.Split(input, " ")) < which+1 {
		return "", errors.New("message doesn't contain enough arguments")
	}
	output = strings.Split(input, " ")[which]
	return output, nil
}

func decodeAmountFromCommand(input string) (amount int, err error) {
	if len(strings.Split(input, " ")) < 2 {
		errmsg := "message doesn't contain any amount"
		// log.Errorln(errmsg)
		return 0, errors.New(errmsg)
	}
	amount, err = getAmount(strings.Split(input, " ")[1])
	return amount, err
}

func getAmount(input string) (amount int, err error) {
	// convert something like 1.2k into 1200
	if strings.HasSuffix(strings.ToLower(input), "k") {
		fmount, err := strconv.ParseFloat(strings.TrimSpace(input[:len(input)-1]), 64)
		if err != nil {
			return 0, err
		}
		amount = int(fmount * 1000)
		return amount, err
	}

	// convert fiat currencies to satoshis
	for currency, symbol := range price.P.Currencies {
		if strings.HasPrefix(input, symbol) || strings.HasSuffix(input, symbol) || // for 1$ and $1
			strings.HasPrefix(strings.ToLower(input), strings.ToLower(currency)) || // for USD1
			strings.HasSuffix(strings.ToLower(input), strings.ToLower(currency)) { // for 1USD
			numeric_string := ""
			numeric_string = strings.Replace(input, symbol, "", 1)                                              // for symbol like $
			numeric_string = strings.Replace(strings.ToLower(numeric_string), strings.ToLower(currency), "", 1) // for 1USD
			fmount, err := strconv.ParseFloat(numeric_string, 64)
			if err != nil {
				log.Errorln(err)
				return 0, err
			}
			if !(price.Price[currency] > 0) {
				return 0, errors.New("price is zero")
			}
			amount = int(fmount / price.Price[currency] * float64(100_000_000))
			return amount, nil
		}
	}

	// use plain integer as satoshis
	amount, err = strconv.Atoi(input)
	if err != nil {
		return 0, err
	}
	if amount <= 0 {
		return 0, errors.New("amount must be greater than 0")
	}
	return amount, err
}

type EnterAmountStateData struct {
	ID              string `json:"ID"`              // holds the ID of the tx object in bunt db
	Type            string `json:"Type"`            // holds type of the tx in bunt db (needed for type checking)
	Amount          int64  `json:"Amount"`          // holds the amount entered by the user mSat
	AmountMin       int64  `json:"AmountMin"`       // holds the minimum amount that needs to be entered mSat
	AmountMax       int64  `json:"AmountMax"`       // holds the maximum amount that needs to be entered mSat
	OiringalCommand string `json:"OiringalCommand"` // hold the originally entered command for evtl later use
}

func (bot *TipBot) askForAmount(ctx context.Context, id string, eventType string, amountMin int64, amountMax int64, originalCommand string) (enterAmountStateData *EnterAmountStateData, err error) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}
	enterAmountStateData = &EnterAmountStateData{
		ID:              id,
		Type:            eventType,
		AmountMin:       amountMin,
		AmountMax:       amountMax,
		OiringalCommand: originalCommand,
	}
	// set LNURLPayParams in the state of the user
	stateDataJson, err := json.Marshal(enterAmountStateData)
	if err != nil {
		log.Errorln(err)
		return
	}
	SetUserState(user, bot, lnbits.UserEnterAmount, string(stateDataJson))
	askAmountText := Translate(ctx, "enterAmountMessage")
	if amountMin > 0 && amountMax >= amountMin {
		askAmountText = fmt.Sprintf(Translate(ctx, "enterAmountRangeMessage"), enterAmountStateData.AmountMin/1000, enterAmountStateData.AmountMax/1000)
	}
	// Let the user enter an amount and return
	bot.trySendMessage(user.Telegram, askAmountText, tb.ForceReply)
	return
}

// enterAmountHandler is invoked in anyTextHandler when the user needs to enter an amount
// the amount is then stored as an entry in the user's stateKey in the user database
// any other handler that relies on this, needs to load the resulting amount from the database
func (bot *TipBot) enterAmountHandler(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}

	if !(user.StateKey == lnbits.UserEnterAmount) {
		ResetUserState(user, bot)
		return // errors.New("user state does not match"), 0
	}

	var EnterAmountStateData EnterAmountStateData
	err := json.Unmarshal([]byte(user.StateData), &EnterAmountStateData)
	if err != nil {
		log.Errorf("[enterAmountHandler] %s", err.Error())
		ResetUserState(user, bot)
		return
	}

	amount, err := getAmount(m.Text)
	if err != nil {
		log.Warnf("[enterAmountHandler] %s", err.Error())
		bot.trySendMessage(m.Sender, Translate(ctx, "lnurlInvalidAmountMessage"))
		ResetUserState(user, bot)
		return //err, 0
	}
	// amount not in allowed range from LNURL
	if EnterAmountStateData.AmountMin > 0 && EnterAmountStateData.AmountMax >= EnterAmountStateData.AmountMin && // this line checks whether min_max is set at all
		(amount > int(EnterAmountStateData.AmountMax/1000) || amount < int(EnterAmountStateData.AmountMin/1000)) { // this line then checks whether the amount is in the range
		err = fmt.Errorf("amount not in range")
		log.Warnf("[enterAmountHandler] %s", err.Error())
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "lnurlInvalidAmountRangeMessage"), EnterAmountStateData.AmountMin/1000, EnterAmountStateData.AmountMax/1000))
		ResetUserState(user, bot)
		return
	}

	// find out which type the object in bunt has waiting for an amount
	// we stored this in the EnterAmountStateData before
	switch EnterAmountStateData.Type {
	case "LnurlPayState":
		tx := &LnurlPayState{Base: transaction.New(transaction.ID(EnterAmountStateData.ID))}
		sn, err := tx.Get(tx, bot.Bunt)
		if err != nil {
			return
		}
		LnurlPayState := sn.(*LnurlPayState)
		LnurlPayState.Amount = amount * 1000 // mSat
		// add result to persistent struct
		runtime.IgnoreError(LnurlPayState.Set(LnurlPayState, bot.Bunt))

		EnterAmountStateData.Amount = int64(amount) * 1000 // mSat
		StateDataJson, err := json.Marshal(EnterAmountStateData)
		if err != nil {
			log.Errorln(err)
			return
		}
		SetUserState(user, bot, lnbits.UserHasEnteredAmount, string(StateDataJson))
		bot.lnurlPayHandlerSend(ctx, m)
		return
	case "LnurlWithdrawState":
		tx := &LnurlWithdrawState{Base: transaction.New(transaction.ID(EnterAmountStateData.ID))}
		sn, err := tx.Get(tx, bot.Bunt)
		if err != nil {
			return
		}
		LnurlWithdrawState := sn.(*LnurlWithdrawState)
		LnurlWithdrawState.Amount = amount * 1000 // mSat
		// add result to persistent struct
		runtime.IgnoreError(LnurlWithdrawState.Set(LnurlWithdrawState, bot.Bunt))

		EnterAmountStateData.Amount = int64(amount) * 1000 // mSat
		StateDataJson, err := json.Marshal(EnterAmountStateData)
		if err != nil {
			log.Errorln(err)
			return
		}
		SetUserState(user, bot, lnbits.UserHasEnteredAmount, string(StateDataJson))
		bot.lnurlWithdrawHandlerWithdraw(ctx, m)
		return
	case "CreateInvoiceState":
		m.Text = fmt.Sprintf("/invoice %d", amount)
		SetUserState(user, bot, lnbits.UserHasEnteredAmount, "")
		bot.invoiceHandler(ctx, m)
		return
	case "CreateDonationState":
		m.Text = fmt.Sprintf("/donate %d", amount)
		SetUserState(user, bot, lnbits.UserHasEnteredAmount, "")
		bot.donationHandler(ctx, m)
		return
	case "CreateSendState":
		splits := strings.SplitAfterN(EnterAmountStateData.OiringalCommand, " ", 2)
		if len(splits) > 1 {
			m.Text = fmt.Sprintf("/send %d %s", amount, splits[1])
			SetUserState(user, bot, lnbits.UserHasEnteredAmount, "")
			bot.sendHandler(ctx, m)
			return
		}
		return
	default:
		ResetUserState(user, bot)
		return
	}
	// // reset database entry
	// ResetUserState(user, bot)
	// return
}
