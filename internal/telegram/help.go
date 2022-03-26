package telegram

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"

	tb "gopkg.in/lightningtipbot/telebot.v3"
)

func (bot TipBot) makeHelpMessage(ctx context.Context, m *tb.Message) string {
	fromUser := LoadUser(ctx)
	dynamicHelpMessage := ""
	// user has no username set
	if len(m.Sender.Username) == 0 {
		// return fmt.Sprintf(helpMessage, fmt.Sprintf("%s\n\n", helpNoUsernameMessage))
		dynamicHelpMessage = dynamicHelpMessage + "\n" + Translate(ctx, "helpNoUsernameMessage")
	}
	lnaddr, _ := bot.UserGetLightningAddress(fromUser)
	if len(lnaddr) > 0 {
		dynamicHelpMessage = dynamicHelpMessage + "\n" + fmt.Sprintf(Translate(ctx, "infoYourLightningAddress"), lnaddr)
	}
	if len(dynamicHelpMessage) > 0 {
		dynamicHelpMessage = Translate(ctx, "infoHelpMessage") + dynamicHelpMessage
	}
	helpMessage := Translate(ctx, "helpMessage")
	return fmt.Sprintf(helpMessage, dynamicHelpMessage)
}

func (bot TipBot) helpHandler(ctx intercept.Context) (intercept.Context, error) {
	// check and print all commands
	bot.anyTextHandler(ctx)
	if !ctx.Message().Private() {
		// delete message
		bot.tryDeleteMessage(ctx.Message())
	}
	bot.trySendMessage(ctx.Sender(), bot.makeHelpMessage(ctx, ctx.Message()), tb.NoPreview)
	return ctx, nil
}

func (bot TipBot) basicsHandler(ctx intercept.Context) (intercept.Context, error) {
	// check and print all commands
	bot.anyTextHandler(ctx)
	if !ctx.Message().Private() {
		// delete message
		bot.tryDeleteMessage(ctx.Message())
	}
	bot.trySendMessage(ctx.Sender(), Translate(ctx, "basicsMessage"), tb.NoPreview)
	return ctx, nil
}

func (bot TipBot) makeAdvancedHelpMessage(ctx context.Context, m *tb.Message) string {
	fromUser := LoadUser(ctx)
	dynamicHelpMessage := "ℹ️ *Info*\n"
	// user has no username set
	if len(m.Sender.Username) == 0 {
		// return fmt.Sprintf(helpMessage, fmt.Sprintf("%s\n\n", helpNoUsernameMessage))
		dynamicHelpMessage = dynamicHelpMessage + fmt.Sprintf("%s", Translate(ctx, "helpNoUsernameMessage")) + "\n"
	}
	// we print the anonymous ln address in the advanced help
	lnaddr, err := bot.UserGetAnonLightningAddress(fromUser)
	if err == nil {
		dynamicHelpMessage = dynamicHelpMessage + fmt.Sprintf("Anonymous lightning address: `%s`\n", lnaddr)
	}
	lnurl, err := UserGetLNURL(fromUser)
	if err == nil {
		dynamicHelpMessage = dynamicHelpMessage + fmt.Sprintf("LNURL: `%s`", lnurl)
	}

	// this is so stupid:
	return fmt.Sprintf(
		Translate(ctx, "advancedMessage"),
		dynamicHelpMessage,
		GetUserStr(bot.Telegram.Me),
	)
}

func (bot TipBot) advancedHelpHandler(ctx intercept.Context) (intercept.Context, error) {
	// check and print all commands
	bot.anyTextHandler(ctx)
	if !ctx.Message().Private() {
		// delete message
		bot.tryDeleteMessage(ctx.Message())
	}
	bot.trySendMessage(ctx.Sender(), bot.makeAdvancedHelpMessage(ctx, ctx.Message()), tb.NoPreview)
	return ctx, nil
}
