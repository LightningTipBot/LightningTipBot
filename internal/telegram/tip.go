package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/str"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

func helpTipUsage(ctx context.Context, errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(Translate(ctx, "tipHelpText"), fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(Translate(ctx, "tipHelpText"), "")
	}
}

func TipCheckSyntax(ctx context.Context, m *tb.Message) (bool, string) {
	arguments := strings.Split(m.Text, " ")
	if len(arguments) < 2 {
		return false, Translate(ctx, "tipEnterAmountMessage")
	}
	return true, ""
}

func (bot *TipBot) tipHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	// check and print all commands
	bot.anyTextHandler(ctx)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, fmt.Errorf("user has no wallet")
	}

	// only if message is a reply
	if !m.IsReply() {
		bot.tryDeleteMessage(m)
		bot.trySendMessage(m.Sender, helpTipUsage(ctx, Translate(ctx, "tipDidYouReplyMessage")))
		bot.trySendMessage(m.Sender, Translate(ctx, "tipInviteGroupMessage"))
		return ctx, errors.Create(errors.NoReplyMessageError)
	}

	if ok, err := TipCheckSyntax(ctx, m); !ok {
		bot.trySendMessage(m.Sender, helpTipUsage(ctx, err))
		NewMessage(m, WithDuration(0, bot))
		return ctx, errors.Create(errors.InvalidSyntaxError)
	}

	// get tip amount
	amount, err := decodeAmountFromCommand(m.Text)
	if err != nil || amount < 1 {
		// immediately delete if the amount is bullshit
		NewMessage(m, WithDuration(0, bot))
		bot.trySendMessage(m.Sender, helpTipUsage(ctx, Translate(ctx, "tipValidAmountMessage")))
		err = fmt.Errorf("%v: %v", errors.Create(errors.InvalidAmountError), err)
		log.WithFields(log.Fields{
			"module":    "telegram",
			"func":      "tipHandler",
			"path":      "/tip",
			"user":      GetUserStr(user.Telegram),
			"user_id":   user.ID,
			"amount":    amount,
			"wallet_id": user.Wallet.ID}).Warnln(err.Error())
		return ctx, err
	}

	err = bot.parseCmdDonHandler(ctx)
	if err == nil {
		err = fmt.Errorf("invalid parseCmdDonHandler")
		log.WithFields(log.Fields{
			"module":    "telegram",
			"func":      "tipHandler",
			"path":      "/tip",
			"user":      GetUserStr(user.Telegram),
			"user_id":   user.ID,
			"amount":    amount,
			"wallet_id": user.Wallet.ID}).Warnln(err.Error())
		return ctx, err
	}
	// TIP COMMAND IS VALID
	from := LoadUser(ctx)
	to := LoadReplyToUser(ctx)

	if from.Telegram.ID == to.Telegram.ID {
		NewMessage(m, WithDuration(0, bot))
		bot.trySendMessage(m.Sender, Translate(ctx, "tipYourselfMessage"))
		return ctx, fmt.Errorf("cannot tip yourself")
	}

	toUserStrMd := GetUserStrMd(to.Telegram)
	fromUserStrMd := GetUserStrMd(from.Telegram)
	toUserStr := GetUserStr(to.Telegram)
	fromUserStr := GetUserStr(from.Telegram)

	if _, exists := bot.UserExists(to.Telegram); !exists {
		log.WithFields(log.Fields{
			"module":      "telegram",
			"func":        "tipHandler",
			"path":        "/tip",
			"to_user":     toUserStr,
			"to_user_id":  to.ID,
			"user":        fromUserStr,
			"user_id":     from.ID,
			"wallet_id":   from.Wallet.ID,
			"telegram_id": from.Telegram.ID}).Infof("User has no wallet.")
		to, err = bot.CreateWalletForTelegramUser(to.Telegram)
		if err != nil {
			log.WithFields(log.Fields{
				"module":       "faucet",
				"func":         "acceptInlineFaucetHandler",
				"to_user":      GetUserStr(to.Telegram),
				"to_user_id":   to.ID,
				"to_wallet_id": to.Wallet.ID,
				"user":         GetUserStr(from.Telegram),
				"user_id":      from.ID,
				"wallet_id":    from.Wallet.ID,
				"error":        err.Error()},
			).Errorln("Could not create wallet for user")
			return ctx, err
		}
	}

	// check for memo in command
	tipMemo := ""
	if len(strings.Split(m.Text, " ")) > 2 {
		tipMemo = strings.SplitN(m.Text, " ", 3)[2]
		if len(tipMemo) > 200 {
			tipMemo = tipMemo[:200]
			tipMemo = tipMemo + "..."
		}
	}

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("🏅 Tip from %s to %s.", fromUserStr, toUserStr)
	t := NewTransaction(bot, from, to, amount, TransactionType("tip"), TransactionChat(m.Chat))
	t.Memo = transactionMemo
	success, err := t.Send()
	if !success {
		NewMessage(m, WithDuration(0, bot))
		bot.trySendMessage(m.Sender, fmt.Sprintf("%s: %s", Translate(ctx, "tipErrorMessage"), Translate(ctx, "tipUndefinedErrorMsg")))
		log.WithFields(log.Fields{
			"module":      "telegram",
			"func":        "tipHandler",
			"path":        "/tip",
			"to_user":     toUserStr,
			"to_user_id":  to.ID,
			"user":        fromUserStr,
			"user_id":     from.ID,
			"wallet_id":   from.Wallet.ID,
			"error":       err.Error(),
			"telegram_id": from.Telegram.ID}).Warnf("Transaction failed")
		return ctx, err
	}

	// update tooltip if necessary
	messageHasTip := tipTooltipHandler(m, bot, amount, to.Initialized)

	log.WithFields(log.Fields{
		"module":      "tip",
		"func":        "tipHandler",
		"to_user":     toUserStr,
		"user":        fromUserStr,
		"user_id":     user.ID,
		"wallet_id":   user.Wallet.ID,
		"amount":      amount,
		"telegram_id": user.Telegram.ID,
		"error":       err}).Info("created Tip")

	// notify users
	bot.trySendMessage(from.Telegram, fmt.Sprintf(i18n.Translate(from.Telegram.LanguageCode, "tipSentMessage"), amount, toUserStrMd))

	// forward tipped message to user once
	if !messageHasTip {
		bot.tryForwardMessage(to.Telegram, m.ReplyTo, tb.Silent)
	}
	bot.trySendMessage(to.Telegram, fmt.Sprintf(i18n.Translate(to.Telegram.LanguageCode, "tipReceivedMessage"), fromUserStrMd, amount))

	if len(tipMemo) > 0 {
		bot.trySendMessage(to.Telegram, fmt.Sprintf("✉️ %s", str.MarkdownEscape(tipMemo)))
	}
	// delete the tip message after a few seconds, this is default behaviour
	NewMessage(m, WithDuration(time.Second*time.Duration(internal.Configuration.Telegram.MessageDisposeDuration), bot))
	return ctx, nil
}
