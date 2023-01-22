package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"github.com/tidwall/buntdb"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

var defaultTicketDuration = time.Minute * 15

// handleTelegramNewMember is invoked when users join groups.
// handler will create a new invoice and send it to the group chat.
// a ticket callback timer function is stored in the blunt db.
func (bot *TipBot) handleTelegramNewMember(ctx intercept.Context) (intercept.Context, error) {
	id := strconv.FormatInt(ctx.Chat().ID, 10)
	group, err := bot.loadGroup(id)
	if err != nil {
		return ctx, err
	}
	if !bot.isAdmin(ctx.Chat(), bot.Telegram.Me) {
		log.Traceln("[TICKET] I am not an admin of this group")
		return ctx, fmt.Errorf("no admin rights")
	}
	user := LoadUser(ctx)
	// if the user does not have an account we put his Telegram user here
	// because we will need it after the invoice callback has been triggered in stopJoinTicketTimer
	if user == nil {
		user = &lnbits.User{
			ID:       "",
			Telegram: ctx.Message().Sender,
		}
	}

	ownerUser, err := GetUser(group.Owner, *bot)
	if err != nil {
		log.Errorln("[TICKET] Error: no owner found")
		return ctx, err
	}
	ticket := JoinTicket{
		Sender: ctx.Sender(),
		Ticket: group.Ticket,
	}
	// group owner creates invoice
	invoice, err := ownerUser.Wallet.Invoice(
		lnbits.InvoiceParams{
			Out:     false,
			Amount:  ticket.Ticket.Price,
			Memo:    ticket.Ticket.Memo,
			Webhook: internal.Configuration.Lnbits.WebhookServer},
		bot.Client)
	if err != nil {
		errmsg := fmt.Sprintf("[handleTelegramNewMember] Could not create an invoice: %s", err.Error())
		log.Errorln(errmsg)
		return ctx, err
	}

	// send the owner invoice to chat
	// TicketEvent is used for callback button (if user has balance)
	ticketEvent := TicketEvent{
		// invoice event is used for invoice callbacks (do not kick if invoice is paid)
		InvoiceEvent: &InvoiceEvent{
			Base: storage.New(storage.ID(fmt.Sprintf("invoice-event:%s", id))),
			Invoice: &Invoice{PaymentHash: invoice.PaymentHash,
				PaymentRequest: invoice.PaymentRequest,
				Amount:         ticket.Ticket.Price,
				Memo:           ticket.Ticket.Memo},
			Payer:        user,
			Chat:         ctx.Chat(),
			Callback:     InvoiceCallbackPayJoinTicket,
			CallbackData: "",
			LanguageCode: ctx.Value("publicLanguageCode").(string),
		},
		Group: group,
		Base:  storage.New(storage.ID(fmt.Sprintf("ticket-event:%s", id))),
	}
	captionText := fmt.Sprintf("⚠️ %s, this group requires you to pay *%d sat* to join. You have 15 minutes to pay or you will be kicked for one day.", GetUserStrMd(ctx.Message().Sender), ticket.Ticket.Price)

	var balance int64 = 0
	if user.ID != "" {
		// if the user has an account
		// // if the user has enough balance, we send him a payment button
		balance, err = bot.GetUserBalance(user)
		if err != nil {
			errmsg := fmt.Sprintf("[/group] Error: Could not get user balance: %s", err.Error())
			log.Errorln(errmsg)
			bot.trySendMessage(ctx.Message().Sender, Translate(ctx, "errorTryLaterMessage"))
			return ctx, errors.New(errors.GetBalanceError, err)
		}
	} else {
		balance = 0
	}

	var msg *tb.Message
	if balance >= group.Ticket.Price {
		_, menu := bot.getSendPayButton(ctx, ticketEvent)
		msg = bot.trySendMessageEditable(ctx.Chat(), captionText, menu)
	} else {
		// create qr code
		qr, err := qrcode.Encode(invoice.PaymentRequest, qrcode.Medium, 256)
		if err != nil {
			return ctx, err
		}
		entryMessage := &tb.Photo{
			File:    tb.File{FileReader: bytes.NewReader(qr)},
			Caption: captionText}
		entryMessage.Caption = fmt.Sprintf("%s\n\n`%s`", entryMessage.Caption, invoice.PaymentRequest)
		msg = bot.trySendMessageEditable(ctx.Chat(), entryMessage)
	}
	ticketEvent.Message = msg
	ticket.Message = msg
	ticket.CreatedTimestamp = time.Now()
	err = bot.Bunt.Set(&ticket)
	if err != nil {
		return ctx, err
	}
	// save invoice struct for later use
	err = ticketEvent.Set(&ticketEvent, bot.Bunt)
	if err != nil {
		return ctx, err
	}
	err = ticketEvent.InvoiceEvent.Set(ticketEvent.InvoiceEvent, bot.Bunt)
	if err != nil {
		return ctx, err
	}
	bot.startTicketCallbackFunctionTimer(ticket)
	fmt.Println(ctx.Message())
	return ctx, nil
}

// stopTicketTimer will load the timer function based on the event.
// should stop the ticker timer function and remove ticket from bluntDB.
func (bot *TipBot) stopJoinTicketTimer(event Event) {
	ev := event.(*InvoiceEvent)
	ticket := JoinTicket{Sender: ev.Payer.Telegram, Message: ev.Message}
	err := bot.Bunt.Get(&ticket)
	if err != nil {
		log.Errorf("[stopJoinTicketTimer] %v", err)
		return
	}
	if commission := getTicketCommission(ticket.Ticket); commission > 0 {
		me, err := GetUser(bot.Telegram.Me, *bot)
		if err != nil {
			log.Errorf("[stopJoinTicketTimer] %v", err)
			return
		}
		invoice, err := me.Wallet.Invoice(
			lnbits.InvoiceParams{
				Out:     false,
				Amount:  commission,
				Memo:    "🎟 Ticket commission",
				Webhook: internal.Configuration.Lnbits.WebhookServer},
			bot.Client)
		if err != nil {
			log.Errorf("[stopJoinTicketTimer] %v", err)
			return
		}
		_, err = ticket.Ticket.Creator.Wallet.Pay(lnbits.PaymentParams{Bolt11: invoice.PaymentRequest, Out: true}, bot.Client)
		if err != nil {
			log.Errorf("[stopJoinTicketTimer] %v", err)
			return
		}
	}

	d := time.Until(time.Now().Add(defaultTicketDuration))
	bot.tryDeleteMessage(ev.Message)
	t := runtime.GetFunction(ticket.Key(), runtime.WithDuration(d))
	t.StopChan <- struct{}{}
	err = bot.Bunt.Delete(ticket.Key(), &ticket)
	if err != nil {
		log.Errorf("[stopJoinTicketTimer] %v", err)
	}
}

// startTicketCallbackFunctionTimer will start a ticket which will ban users if the timer runs out.
func (bot *TipBot) startTicketCallbackFunctionTimer(ticket JoinTicket) {
	// check if ticket is already expired
	if ticket.CreatedTimestamp.Add(defaultTicketDuration).Before(time.Now()) {
		return
	}
	// ticket is valid. create duration until kick
	ticketDuration := time.Until(ticket.CreatedTimestamp.Add(defaultTicketDuration))
	// create function timer
	t := runtime.NewResettableFunction(ticket.Key(),
		runtime.WithTimer(time.NewTimer(ticketDuration)))
	// run the ticket callback function
	t.Do(func() {
		// ticket expired
		member, err := bot.Telegram.ChatMemberOf(ticket.Message.Chat, ticket.Sender)
		if err != nil {
			log.Errorln("🧨 could not fetch / ban chat member")
			return
		}
		// ban user
		err = bot.Telegram.Ban(ticket.Message.Chat, member)
		if err != nil {
			log.Errorln(err)
		}
		err = bot.Bunt.Delete(ticket.Key(), &ticket)
		if err != nil {
			log.Errorln(err)
		}
		bot.tryDeleteMessage(ticket.Message)
	})
}

// restartPersistedTickets kicks of all ticket timers
func (bot *TipBot) restartPersistedTickets() {
	bot.Bunt.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend("join-ticket", func(key, value string) bool {
			ticket := JoinTicket{}
			err := json.Unmarshal([]byte(value), &ticket)
			if err != nil {
				return true
			}
			bot.startTicketCallbackFunctionTimer(ticket)
			return true // continue iteration
		})
		return err
	})
}
func getTicketCommission(ticket *Ticket) int64 {
	if ticket.Price < 20 {
		return 0
	}
	// 2% cut + 100 sat base fee
	commissionSat := ticket.Price*ticket.Cut/100 + ticket.BaseFee
	if ticket.Price <= 1000 {
		// if < 1000, then 10% cut + 10 sat base fee
		commissionSat = ticket.Price*ticket.CutCheap/100 + ticket.BaseFeeCheap
	}
	return commissionSat
}
