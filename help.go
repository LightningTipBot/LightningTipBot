package main

import (
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	helpMessage = "ℹ️ *Info*\n_This bot is a Bitcoin Lightning ⚡️ wallet and can sends tips on Telegram. Simply add it to your group. The basic unit of tips are Satoshis (sat). 100,000,000 sat = 1 Bitcoin. There will only ever be 21 Million Bitcoin. Type /info for more._\n\n" +
		"❤️ *Donate*\n" +
		"_This bot charges no fees but generates costs. If you like to support this bot, please consider a donation. To donate, just tip @LightningTipBot or try_ `/send 1000 @LightningTipBot`\n\n" +
		"⚙️ *Commands*\n" +
		"*/tip* 🏅 Reply to a message to tip it: `/tip <amount> [<memo>]`\n" +
		"*/balance* 👑 Check your balance: `/balance`\n" +
		"*/send* 💸 Send funds to a user: `/send <amount> <@username> [<memo>]`\n" +
		"*/invoice* ⚡️ Receive over Lightning: `/invoice <amount> [<memo>]`\n" +
		"*/pay* ⚡️ Pay over Lightning: `/pay <invoice>`\n" +
		"*/help* 📖 Read this help.\n"

	infoMessage = "🧡 *Bitcoin*\n" +
		"_Bitcoin is the currency of the internet. It is permissionless and decentralized and has no masters and no controling authority. Bitcoin is sound money that is faster, more secure, and more inclusive than the legacy financial system._\n\n" +
		"🧮 *Economnics*\n" +
		"_The smallest unit of Bitcoin are Satoshis (sat) and 100,000,000 sat = 1 Bitcoin. There will only ever be 21 Million Bitcoin. The fiat currency value of Bitcoin can change daily. However, if you live on a Bitcoin standard 1 sat will always equal 1 sat._\n\n" +
		"⚡️ *The Lightning Network*\n" +
		"_The Lightning Network is a payment protocol that enables fast and cheap Bitcoin payments that require almost no energy. It is what scales Bitcoin to the billions of people around the world._\n\n" +
		"📲 *Lightning Wallets*\n" +
		"_Your funds on this bot can be sent to any other Lightning wallet and vice versa. Recommended Lightning wallets for your phone are_ [Phoenix](https://phoenix.acinq.co/)_,_ [Breez](https://breez.technology/)_,_ [Muun](https://muun.com/)_ (non-custodial), or_ [Wallet of Satoshi](https://www.walletofsatoshi.com/) _(easy)_.\n\n" +
		"📄 *Open Source*\n" +
		"_This bot is free and_ [open source](https://github.com/LightningTipBot/LightningTipBot) _software. You can run it on your own computer and use it in your own community. You don't have to trust anyone to keep your funds safe._\n\n" +
		"✈️ *Telegram*\n" +
		"_Add this bot to your Telegram group chat to /tip posts. If you make the bot admin of the group it will also clean up commands to keep the chat tidy._\n\n" +
		// "🏛 *Terms*\n" +
		// "_We are not custodian of your funds. Any amount you load onto your wallet will be legally considered a donation that belongs to us. We will act in your best interest but we're also aware that the situation without KYC is tricky until we figure something out. Do not give us all your money.  Be aware that this bot is in beta development. Use at your own risk._\n\n" +
		"❤️ *Donate*\n" +
		"_This bot charges no fees but generates costs. If you like to support this bot, please consider a donation. To donate, just tip @LightningTipBot or try_ `/send 1000 @LightningTipBot`"
)

func (bot TipBot) helpHandler(m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	if !m.Private() {
		// delete message
		NewMessage(m).Dispose(0, bot.telegram)
	}
	bot.telegram.Send(m.Sender, helpMessage, tb.NoPreview)
	return
}

func (bot TipBot) infoHandler(m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	if !m.Private() {
		// delete message
		NewMessage(m).Dispose(0, bot.telegram)
	}
	bot.telegram.Send(m.Sender, infoMessage, tb.NoPreview)
	return
}
