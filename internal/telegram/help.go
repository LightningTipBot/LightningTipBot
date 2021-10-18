package telegram

import (
	"context"
	"fmt"

	tb "gopkg.in/tucnak/telebot.v2"
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

func (bot TipBot) helpHandler(ctx context.Context, m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	if !m.Private() {
		// delete message
		NewMessage(m, WithDuration(0, bot.Telegram))
	}
	bot.trySendMessage(m.Sender, bot.makeHelpMessage(ctx, m), tb.NoPreview)
	return
}

func (bot TipBot) basicsHandler(ctx context.Context, m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	if !m.Private() {
		// delete message
		NewMessage(m, WithDuration(0, bot.Telegram))
	}
	bot.trySendMessage(m.Sender, Translate(ctx, "basicsMessage"), tb.NoPreview)
	return
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
		dynamicHelpMessage = dynamicHelpMessage + fmt.Sprintf("Lightning Address: `%s`\n", lnaddr)
	}
	lnurl, err := UserGetLNURL(fromUser)
	if err == nil {
		dynamicHelpMessage = dynamicHelpMessage + fmt.Sprintf("LNURL: `%s`", lnurl)
	}

	// this is so stupid:
	return fmt.Sprintf(Translate(ctx, "advancedMessage"), dynamicHelpMessage, GetUserStr(bot.Telegram.Me), GetUserStr(bot.Telegram.Me), GetUserStr(bot.Telegram.Me))
}

func (bot TipBot) advancedHelpHandler(ctx context.Context, m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	if !m.Private() {
		// delete message
		NewMessage(m, WithDuration(0, bot.Telegram))
	}
	bot.trySendMessage(m.Sender, bot.makeAdvancedHelpMessage(ctx, m), tb.NoPreview)
	return
}
