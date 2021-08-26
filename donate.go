package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

// PLEASE DO NOT CHANGE THE CODE IN THIS FILE
// YOU MIGHT BREAK DONATIONS TO THE ORIGINAL PROJECT
// THE DEVELOPMENT OF LIGHTNINGTIPBOT RELIES ON DONATIONS
// IF YOU USE THIS PROJECT, LEAVE THIS CODE ALONE

var (
	donationEndpoint         = "https://relay.lnscan.com/donate/%d"
	donationSuccess          = "🙏 Thank you for your donation."
	donationErrorMessage     = "🚫 Oh no. Donation failed."
	donationFailedMessage    = "🚫 Donation failed: %s"
	donationInterceptMessage = "It looks like you want to donate to the bot. I'm routing this payment to the original project."
	donateEnterAmountMessage = "Did you enter an amount?"
	donateValidAmountMessage = "Did you enter a valid amount?"
	donateHelpText           = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/donate <amount>`\n" +
		"*Example:* `/donate 1000`"
)

func helpDonateUsage(errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(donateHelpText, fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(donateHelpText, "")
	}
}

func (bot TipBot) donationHandler(m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(m)

	if len(strings.Split(m.Text, " ")) < 2 {
		bot.telegram.Send(m.Sender, helpDonateUsage(donateEnterAmountMessage))
		return
	}
	amount, err := decodeAmountFromCommand(m.Text)
	if err != nil {
		return
	}
	if amount < 1 {
		bot.telegram.Send(m.Sender, helpDonateUsage(donateValidAmountMessage))
		return
	}

	// command is valid

	// get donation invoice
	resp, err := http.Get(fmt.Sprintf(donationEndpoint, amount))
	if err != nil {
		log.Errorln(err)
		bot.telegram.Send(m.Sender, donationErrorMessage)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorln(err)
		bot.telegram.Send(m.Sender, donationErrorMessage)
		return
	}

	// send donation invoice
	user, err := GetUser(m.Sender, bot)
	if err != nil {
		return
	}

	// bot.telegram.Send(user.Telegram, string(body))
	_, err = user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: string(body)}, *user.Wallet)
	if err != nil {
		userStr := GetUserStr(m.Sender)
		errmsg := fmt.Sprintf("[/donate] Donation failed for user %s: %s", userStr, err)
		log.Errorln(errmsg)
		bot.telegram.Send(m.Sender, fmt.Sprintf(donationFailedMessage, err))
		return
	}
	bot.telegram.Send(m.Sender, donationSuccess)

}

func (bot TipBot) parseCmdDonHandler(m *tb.Message) error {
	arg := ""
	if strings.HasPrefix(strings.ToLower(m.Text), "/send") {
		arg, _ = getArgumentFromCommand(m.Text, 2)
		if arg != "@"+bot.telegram.Me.Username {
			return fmt.Errorf("err")
		}
	}
	if strings.HasPrefix(strings.ToLower(m.Text), "/tip") {
		arg = GetUserStr(m.ReplyTo.Sender)
		if arg != "@"+bot.telegram.Me.Username {
			return fmt.Errorf("err")
		}
	}
	if arg == "@LightningTipBot" || len(arg) < 1 {
		return fmt.Errorf("err")
	}

	amount, err := decodeAmountFromCommand(m.Text)
	if err != nil {
		return err
	}
	bot.telegram.Send(m.Sender, donationInterceptMessage)
	m.Text = fmt.Sprintf("/donate %d", amount)
	bot.donationHandler(m)
	// returning nil here will abort the parent handler (/pay or /tip)
	return nil
}
