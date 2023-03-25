package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	log "github.com/sirupsen/logrus"
)

var (
	nosterRegisterMessage       = "📖 Add your nostr pubkey for zap receipts"
	nostrInfoMessage            = "💜 *Your nostr information*\n\nYour pubkey: `%s`"
	nostrInfoLNAddrMessage      = "Your Lightning address: `%s`"
	nostrHelpMessage            = "⚙️ *Nostr commands:*\n`/nostr add <pubkey>` ✅ Add your nostr pubkey.\n`/nostr help` 📖 Show help."
	nostrAddedMessage           = "✅ *Nostr pubkey added.*"
	nostrPrivateKeyErrorMessage = "🚫 This is not your public key but your private key! Very dangerous! Try again with your npub..."
	nostrPublicKeyErrorMessage  = "🚫 There was an error decoding your public key."
)

func uniqueSlice(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func cleanUrls(slice []string) []string {
	list := []string{}
	for _, entry := range slice {
		if strings.HasSuffix(entry, "/") {
			entry = entry[:len(entry)-1]
		}
		list = append(list, entry)
	}
	return list
}

func (bot *TipBot) publishNostrEvent(ev nostr.Event, relays []string) {
	pk := internal.Configuration.Nostr.PrivateKey

	// // BEGIN: testing
	// pub, _ := nostr.GetPublicKey(pk)
	// ev = nostr.Event{
	// 	PubKey:    pub,
	// 	CreatedAt: time.Now(),
	// 	Kind:      1,
	// 	Tags:      nil,
	// 	Content:   "Hello World!",
	// }
	// // END: testing

	// calling Sign sets the event ID field and the event Sig field
	ev.Sign(pk)
	log.Debugf("[NOSTR] 🟣 publishing nostr event %s", ev.ID)

	// more relays
	relays = append(relays, "wss://relay.nostr.ch", "wss://eden.nostr.land", "wss://nostr.btcmp.com", "wss://nostr.relayer.se", "wss://relay.current.fyi", "wss://nos.lol", "wss://nostr.mom", "wss://relay.nostr.info", "wss://nostr.zebedee.cloud", "wss://nostr-pub.wellorder.net", "wss://relay.snort.social/", "wss://relay.damus.io/", "wss://nostr.oxtr.dev/", "wss://nostr.fmt.wiz.biz/", "wss://brb.io")

	// remove trailing /
	relays = cleanUrls(relays)

	// unique relays
	relays = uniqueSlice(relays)

	// crop relays
	var max_relays int = 50
	if len(relays) > max_relays {
		relays = relays[:max_relays]
	}

	// publish the event to relays
	for _, url := range relays {
		go func(url string) {
			// remove trailing /
			relay, e := nostr.RelayConnect(context.Background(), url)
			if e != nil {
				log.Errorf(e.Error())
				return
			}
			time.Sleep(3 * time.Second)

			status := relay.Publish(context.Background(), ev)
			log.Debugf("[NOSTR] published to %s: %s", url, status)

			time.Sleep(3 * time.Second)
			relay.Close()
		}(url)

	}
}

func (bot *TipBot) nostrHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	splits := strings.Split(m.Text, " ")
	if len(splits) == 1 {
		return bot.getNostrHandler(ctx)
	} else if len(splits) > 1 {
		switch strings.ToLower(splits[1]) {
		case "add":
			return bot.addNostrPubkeyHandler(ctx)
		case "help":
			return bot.nostrHelpHandler(ctx)
		}
	}
	return ctx, nil
}

func (bot *TipBot) addNostrPubkeyHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	splits := strings.Split(m.Text, " ")
	splitlen := len(splits)
	if splitlen < 3 {
		return ctx, fmt.Errorf("not enough arguments")
	}
	nostrKeyInput := splits[2]

	// parse input
	if strings.HasPrefix(nostrKeyInput, "nsec") {
		bot.trySendMessage(ctx.Message().Sender, nostrPrivateKeyErrorMessage)
		return ctx, fmt.Errorf("user entered nostr private key")
	}
	// conver to hex
	if strings.HasPrefix(nostrKeyInput, "npub") {
		prefix, pubkey, err := nip19.Decode(nostrKeyInput)
		if err != nil {
			bot.trySendMessage(ctx.Message().Sender, nostrPublicKeyErrorMessage)
			return ctx, fmt.Errorf("shouldn't error: %s", err)
		}
		if prefix != "npub" {
			bot.trySendMessage(ctx.Message().Sender, nostrPublicKeyErrorMessage)
			return ctx, fmt.Errorf("returned invalid prefix")
		}
		nostrKeyInput = pubkey.(string)
	}

	user, err := GetLnbitsUserWithSettings(m.Sender, *bot)
	if err != nil {
		return ctx, err
	}
	// save node in db
	user.Settings.Nostr.PubKey = nostrKeyInput
	err = UpdateUserRecord(user, *bot)
	if err != nil {
		log.Errorf("[registerNodeHandler] could not update record of user %s: %v", GetUserStr(user.Telegram), err)
		return ctx, err
	}
	bot.trySendMessage(ctx.Message().Sender, nostrAddedMessage)
	return ctx, nil
}

func (bot *TipBot) nostrHelpHandler(ctx intercept.Context) (intercept.Context, error) {
	bot.trySendMessage(ctx.Message().Sender, nosterRegisterMessage+"\n\n"+nostrHelpMessage)
	return ctx, nil
}

func (bot *TipBot) getNostrHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	user, err := GetLnbitsUserWithSettings(m.Sender, *bot)
	if err != nil {
		log.Infof("Could not get user settings for user %s", GetUserStr(user.Telegram))
		return ctx, err
	}
	var dynamicHelpMessage string
	dynamicHelpMessage += nostrHelpMessage
	if user.Settings.Nostr.PubKey == "" {
		bot.trySendMessage(m.Sender, dynamicHelpMessage)
		return ctx, fmt.Errorf("no nostr pubkey registered")
	} else if len(user.Settings.Nostr.PubKey) > 0 {

		pubkeyBech32, err := nip19.EncodePublicKey(user.Settings.Nostr.PubKey)
		if err != nil {
			log.Infof("Could not decode user nostr pubkey %s", GetUserStr(user.Telegram))
			return ctx, err
		}
		dynamicHelpMessage += "\n\n" + fmt.Sprintf(nostrInfoMessage, pubkeyBech32)
		if lnaddr, _ := bot.UserGetLightningAddress(user); len(lnaddr) > 0 {
			dynamicHelpMessage += "\n\n" + fmt.Sprintf(nostrInfoLNAddrMessage, lnaddr)
		}
		bot.trySendMessage(m.Sender, dynamicHelpMessage)
	}

	return ctx, nil
}
