package telegram

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/str"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

// PLEASE DO NOT CHANGE THE CODE IN THIS FILE
// YOU MIGHT BREAK DONATIONS TO THE ORIGINAL PROJECT
// THE DEVELOPMENT OF LIGHTNINGTIPBOT RELIES ON DONATIONS
// IF YOU USE THIS PROJECT, LEAVE THIS CODE ALONE

var (
	donationEndpoint string
)

func helpDonateUsage(ctx context.Context, errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(Translate(ctx, "donateHelpText"), fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(Translate(ctx, "donateHelpText"), "")
	}
}

func (bot TipBot) donationHandler(ctx context.Context, m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}
	// if no amount is in the command, ask for it
	amount, err := decodeAmountFromCommand(m.Text)
	if (err != nil || amount < 1) && m.Chat.Type == tb.ChatPrivate {
		// // no amount was entered, set user state and ask for amount
		bot.askForAmount(ctx, "", "CreateDonationState", 0, 0, m.Text)
		return
	}

	// command is valid
	msg := bot.trySendMessage(user.Telegram, Translate(ctx, "donationProgressMessage"))
	// get invoice
	resp, err := http.Get(fmt.Sprintf(donationEndpoint, amount, GetUserStr(user.Telegram), GetUserStr(bot.Telegram.Me)))
	if err != nil {
		log.Errorln(err)
		bot.tryEditMessage(msg, Translate(ctx, "donationErrorMessage"))
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorln(err)
		bot.tryEditMessage(msg, Translate(ctx, "donationErrorMessage"))
		return
	}

	// send donation invoice
	// user := LoadUser(ctx)
	// bot.trySendMessage(user.Telegram, string(body))
	_, err = user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: string(body)}, bot.Client)
	if err != nil {
		userStr := GetUserStr(user.Telegram)
		errmsg := fmt.Sprintf("[/donate] Donation failed for user %s: %s", userStr, err)
		log.Errorln(errmsg)
		bot.tryEditMessage(msg, Translate(ctx, "donationErrorMessage"))
		return
	}
	bot.tryEditMessage(msg, Translate(ctx, "donationSuccess"))

}

func init() {
	var sb strings.Builder
	_, err := io.Copy(&sb, rot13Reader{strings.NewReader("uggcf://ya.gvcf/qbangr/%q?sebz=%f&obg=%f")})
	if err != nil {
		panic(err)
	}
	donationEndpoint = sb.String()
}

type rot13Reader struct {
	r io.Reader
}

func (rot13 rot13Reader) Read(b []byte) (int, error) {
	n, err := rot13.r.Read(b)
	for i := 0; i < n; i++ {
		switch {
		case b[i] >= 65 && b[i] <= 90:
			if b[i] <= 77 {
				b[i] = b[i] + 13
			} else {
				b[i] = b[i] - 13
			}
		case b[i] >= 97 && b[i] <= 122:
			if b[i] <= 109 {
				b[i] = b[i] + 13
			} else {
				b[i] = b[i] - 13
			}
		}
	}
	return n, err
}

func (bot TipBot) parseCmdDonHandler(ctx context.Context, m *tb.Message) error {
	arg := ""
	if strings.HasPrefix(strings.ToLower(m.Text), "/send") {
		arg, _ = getArgumentFromCommand(m.Text, 2)
		if arg != "@"+bot.Telegram.Me.Username {
			return fmt.Errorf("err")
		}
	}
	if strings.HasPrefix(strings.ToLower(m.Text), "/tip") {
		arg = GetUserStr(m.ReplyTo.Sender)
		if arg != "@"+bot.Telegram.Me.Username {
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

	var sb strings.Builder
	_, err = io.Copy(&sb, rot13Reader{strings.NewReader("Gunax lbh! V'z ebhgvat guvf qbangvba gb YvtugavatGvcObg@ya.gvcf.")})
	if err != nil {
		panic(err)
	}
	donationInterceptMessage := sb.String()

	bot.trySendMessage(m.Sender, str.MarkdownEscape(donationInterceptMessage))
	m.Text = fmt.Sprintf("/donate %d", amount)
	bot.donationHandler(ctx, m)
	// returning nil here will abort the parent handler (/pay or /tip)
	return nil
}
