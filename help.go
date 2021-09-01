package main

import (
	"fmt"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	helpMessage = "⚡️ *Wallet*\n_This bot is a Bitcoin Lightning wallet that can sends tips on Telegram. To tip, add the bot to a group chat. The basic unit of tips are Satoshis (sat). 100,000,000 sat = 1 Bitcoin. There will only ever be 21 Million Bitcoin. Type 📚 /info for more._\n\n" +
		"❤️ *Donate*\n" +
		"_This bot charges no fees but costs satoshis to operate. If you like the bot, please consider supporting this project with a donation. To donate, use_ `/donate 1000`\n\n" +
		"%s" +
		"⚙️ *Commands*\n" +
		"*/tip* 🏅 Reply to a message to tip: `/tip <amount> [<memo>]`\n" +
		"*/balance* 👑 Check balance: `/balance`\n" +
		"*/send* 💸 Send funds to a user: `/send <amount> @user or user@ln.tips [<memo>]`\n" +
		"*/invoice* ⚡️ Receive over Lightning: `/invoice <amount> [<memo>]`\n" +
		"*/pay* ⚡️ Pay over Lightning: `/pay <invoice>`\n" +
		"*/donate* ❤️ Donate to the project: `/donate 1000`\n" +
		"*/advanced* 🤖 Read the advanced help.\n" +
		"*/help* 📖 Read this help."

	infoMessage = "🧡 *Bitcoin*\n" +
		"_Bitcoin is the currency of the internet. It is permissionless and decentralized and has no masters and no controling authority. Bitcoin is sound money that is faster, more secure, and more inclusive than the legacy financial system._\n\n" +
		"🧮 *Economnics*\n" +
		"_The smallest unit of Bitcoin are Satoshis (sat) and 100,000,000 sat = 1 Bitcoin. There will only ever be 21 Million Bitcoin. The fiat currency value of Bitcoin can change daily. However, if you live on a Bitcoin standard 1 sat will always equal 1 sat._\n\n" +
		"⚡️ *The Lightning Network*\n" +
		"_The Lightning Network is a payment protocol that enables fast and cheap Bitcoin payments that require almost no energy. It is what scales Bitcoin to the billions of people around the world._\n\n" +
		"📲 *Lightning Wallets*\n" +
		"_Your funds on this bot can be sent to any other Lightning wallet and vice versa. Recommended Lightning wallets for your phone are_ [Phoenix](https://phoenix.acinq.co/)_,_ [Breez](https://breez.technology/)_,_ [Muun](https://muun.com/)_ (non-custodial), or_ [Wallet of Satoshi](https://www.walletofsatoshi.com/) _(easy)_.\n\n" +
		"📄 *Open Source*\n" +
		"_This bot is free and_ [open source](https://github.com/LightningTipBot/LightningTipBot) _software. You can run it on your own computer and use it in your own community._\n\n" +
		"✈️ *Telegram*\n" +
		"_Add this bot to your Telegram group chat to /tip posts. If you make the bot admin of the group it will also clean up commands to keep the chat tidy._\n\n" +
		// "🏛 *Terms*\n" +
		// "_We are not custodian of your funds. Any amount you load onto your wallet will be legally considered a donation that belongs to us. We will act in your best interest but we're also aware that the situation without KYC is tricky until we figure something out. Do not give us all your money.  Be aware that this bot is in beta development. Use at your own risk._\n\n" +
		"❤️ *Donate*\n" +
		"_This bot charges no fees but costs satoshis to operate. If you like the bot, please consider supporting this project with a donation. To donate, use_ `/donate 1000`"

	helpNoUsernameMessage = "ℹ️ You don't have a Telegram username yet."

	advancedMessage = "🤖 *Advanced commands*\n\n" +
		"*/link* 🔗 Link your wallet to [BlueWallet](https://bluewallet.io/) or [Zeus](https://zeusln.app/)\n" +
		"*/lnurl* ⚡️ Lnurl receive or pay: `/lnurl` or `/lnurl <lnurl>`\n"
)

func (bot TipBot) makeHelpMessage(m *tb.Message) string {
	host, _, err := net.SplitHostPort(strings.Split(Configuration.LNURLServer, "//")[1])
	if err != nil {
		log.Errorf("[makeHelpMessage] Error: %s", err)
		return fmt.Sprintf(helpMessage, "")
	}
	if err != nil || len(m.Sender.Username) == 0 {
		return fmt.Sprintf(helpMessage, fmt.Sprintf("%s\n\n", helpNoUsernameMessage))
	}
	return fmt.Sprintf(helpMessage, fmt.Sprintf("ℹ️ *Info*\nYour Lightning Address is `%s@%s`\n\n", strings.ToLower(m.Sender.Username), host))
}

func (bot TipBot) helpHandler(m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(m)
	if !m.Private() {
		// delete message
		NewMessage(m, WithDuration(0, bot.telegram))
	}
	bot.trySendMessage(m.Sender, bot.makeHelpMessage(m), tb.NoPreview)
	return
}

func (bot TipBot) infoHandler(m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(m)
	if !m.Private() {
		// delete message
		NewMessage(m, WithDuration(0, bot.telegram))
	}
	bot.trySendMessage(m.Sender, infoMessage, tb.NoPreview)
	return
}

func (bot TipBot) advancedHelpHandler(m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(m)
	if !m.Private() {
		// delete message
		NewMessage(m, WithDuration(0, bot.telegram))
	}
	bot.trySendMessage(m.Sender, advancedMessage, tb.NoPreview)
	return
}
