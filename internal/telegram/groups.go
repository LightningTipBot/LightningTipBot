package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

type Ticket struct {
	Price   int64        `json:"price"`
	Memo    string       `json:"memo"`
	Creator *lnbits.User `gorm:"embedded;embeddedPrefix:creator_"`
	Cut     int          `json:"cut"` // Percent to cut from ticket price
}
type Group struct {
	Name  string   `json:"name"`
	Title string   `json:"title"`
	ID    int64    `json:"id" gorm:"primaryKey"`
	Owner *tb.User `gorm:"embedded;embeddedPrefix:owner_"`
	// Chat   *tb.Chat `gorm:"embedded;embeddedPrefix:chat_"`
	Ticket *Ticket `gorm:"embedded;embeddedPrefix:ticket_"`
}

type TicketInvoiceEvent struct {
	*storage.Base
	*Invoice
	Group          *Group       `gorm:"embedded;embeddedPrefix:group_"`
	User           *lnbits.User `json:"user"`                      // the user that is being paid
	Message        *tb.Message  `json:"message,omitempty"`         // the message that the invoice replies to
	InvoiceMessage *tb.Message  `json:"invoice_message,omitempty"` // the message that displays the invoice
	LanguageCode   string       `json:"languagecode"`              // language code of the user
	Callback       int          `json:"func"`                      // which function to call if the invoice is paid
	CallbackData   string       `json:"callbackdata"`              // add some data for the callback
	Chat           *tb.Chat     `json:"chat,omitempty"`            // if invoice is supposed to be sent to a particular chat
	Payer          *lnbits.User `json:"payer,omitempty"`           // if a particular user is supposed to pay this
}

func (invoiceEvent TicketInvoiceEvent) Type() EventType {
	return EventTypeTicketInvoice
}
func (invoiceEvent TicketInvoiceEvent) Key() string {
	return fmt.Sprintf("invoice:%s", invoiceEvent.PaymentHash)
}

var (
	ticketPayConfirmationMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnPayTicket              = paymentConfirmationMenu.Data("✅ Pay", "pay_ticket")
)

var (
	groupAddGroupHelpMessage  = "📖 Oops, that didn't work. Please try again.\nUsage: `/group add <group_name> [<amount>]`\nExample: `/group add TheBestBitcoinGroup 1000`"
	grouJoinGroupHelpMessage  = "📖 Oops, that didn't work. Please try again.\nUsage: `/group join <group_name>`\nExample: `/group join TheBestBitcoinGroup`"
	groupClickToJoinMessage   = "[Click here](%s) 👈 to join."
	groupInvoiceMemo          = "Ticket for group %s"
	groupPayInvoiceMessage    = "🎟 To join the group %s, pay the invoice above."
	groupBotIsNotAdminMessage = "🚫 Oops, that didn't work. You must make me admin and grant me rights to invite users."
	groupNameExists           = "🚫 A group with this name already exists. Please choose a different name."
	groupAddedMessage         = "🎟 Tickets for group `%s` added.\nAlias: `%s` Price: %d sat\n\nTo request a ticket for this group, start a private chat with %s and write `/group join %s`."
	groupNotFoundMessage      = "🚫 Could not find a group with this name."
)

func (bot TipBot) groupHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	splits := strings.Split(m.Text, " ")
	if len(splits) == 1 {
		return ctx, nil
	} else if len(splits) > 1 {
		if splits[1] == "join" {
			return bot.groupRequestJoinHandler(ctx, m)
		}
		if splits[1] == "add" {
			return bot.addGroupHandler(ctx, m)
		}
		if splits[1] == "remove" {
			// todo -- implement this
			// return bot.addGroupHandler(ctx, m)
		}
	}
	return ctx, nil
}

// groupRequestJoinHandler sends a payment request to the user who wants to join a group
func (bot TipBot) groupRequestJoinHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	user := LoadUser(ctx)
	// // reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		return ctx, fmt.Errorf("not private chat")
	}
	splits := strings.Split(m.Text, " ")
	if len(splits) != 3 || len(m.Text) > 100 {
		bot.trySendMessage(m.Chat, grouJoinGroupHelpMessage)
		return ctx, nil
	}
	groupname := strings.ToLower(splits[2])

	group := &Group{}
	tx := bot.GroupsDb.Where("name = ? COLLATE NOCASE", groupname).First(group)
	if tx.Error != nil {
		bot.trySendMessage(m.Chat, groupNotFoundMessage)
		return ctx, fmt.Errorf("group not found")
	}

	// if no price is set, then we don't need to pay
	if group.Ticket.Price == 0 {
		invoiceEvent := &TicketInvoiceEvent{
			Payer:        user,
			Group:        group,
			LanguageCode: user.Telegram.LanguageCode,
			Chat:         m.Chat,
		}
		bot.groupGetInviteLinkHandler(invoiceEvent)
		return ctx, nil
	}

	// if a price is set ...

	// create an invoice
	memo := fmt.Sprintf(groupInvoiceMemo, groupname)
	invoice, err := bot.createInvoiceGroupTicket(ctx, user, group, memo, InvoiceCallbackGroupTicket, "")
	invoice.Base.ID = fmt.Sprintf("ticket:%d-%d-%s", m.Sender.ID, group.Ticket.Price, RandStringRunes(5))
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Could not create an invoice: %s", err.Error())
		bot.trySendMessage(user.Telegram, Translate(ctx, "errorTryLaterMessage"))
		log.Errorln(errmsg)
		return ctx, err
	}

	// add result to persistent struct
	runtime.IgnoreError(invoice.Set(invoice, bot.Bunt))

	// if the user has enough balance, we send him a payment button
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		errmsg := fmt.Sprintf("[/group] Error: Could not get user balance: %s", err.Error())
		log.Errorln(errmsg)
		bot.trySendMessage(m.Sender, Translate(ctx, "errorTryLaterMessage"))
		return ctx, errors.New(errors.GetBalanceError, err)
	}
	if balance >= group.Ticket.Price {
		return bot.groupSendPayButtonHandler(ctx, m, invoice)
	}

	// otherwise we send a payment request

	// create qr code
	qr, err := qrcode.Encode(invoice.PaymentRequest, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Failed to create QR code for invoice: %s", err.Error())
		bot.trySendMessage(user.Telegram, Translate(ctx, "errorTryLaterMessage"))
		log.Errorln(errmsg)
		return ctx, err
	}
	bot.trySendMessage(m.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", invoice.PaymentRequest)})
	bot.trySendMessage(m.Sender, fmt.Sprintf(groupPayInvoiceMessage, groupname))
	return ctx, nil
}

func (bot *TipBot) groupSendPayButtonHandler(ctx context.Context, m *tb.Message, invoice TicketInvoiceEvent) (context.Context, error) {
	// object that holds all information about the send payment
	// // // create inline buttons
	btnPayTicket := ticketPayConfirmationMenu.Data(Translate(ctx, "payButtonMessage"), "pay_ticket", invoice.Base.ID)
	ticketPayConfirmationMenu.Inline(
		ticketPayConfirmationMenu.Row(
			btnPayTicket),
	)
	return ctx, nil
}

func (bot *TipBot) groupConfirmPayButtonHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	tx := &PayData{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(ctx, tx.ID)
	defer mutex.UnlockWithContext(ctx, tx.ID)
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[confirmPayHandler] %s", err.Error())
		return ctx, err
	}
	invoice := sn.(TicketInvoiceEvent)

	// onnly the correct user can press
	if invoice.Payer.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	if !invoice.Active {
		log.Errorf("[confirmPayHandler] send not active anymore")
		bot.tryEditMessage(c.Message, i18n.Translate(invoice.LanguageCode, "errorTryLaterMessage"), &tb.ReplyMarkup{})
		bot.tryDeleteMessage(c.Message)
		return ctx, errors.Create(errors.NotActiveError)
	}
	defer invoice.Set(invoice, bot.Bunt)

	user := LoadUser(ctx)
	if user.Wallet == nil {
		bot.tryDeleteMessage(c.Message)
		return ctx, errors.Create(errors.UserNoWalletError)
	}

	log.Infof("[/pay] Attempting %s's invoice %s (%d sat)", GetUserStr(user.Telegram), invoice.ID, invoice.Invoice.Amount)
	// pay invoice
	_, err = user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: invoice.Invoice.PaymentRequest}, bot.Client)
	if err != nil {
		errmsg := fmt.Sprintf("[/pay] Could not pay invoice of %s: %s", GetUserStr(user.Telegram), err)
		err = fmt.Errorf(i18n.Translate(invoice.LanguageCode, "invoiceUndefinedErrorMessage"))
		bot.tryEditMessage(c.Message, fmt.Sprintf(i18n.Translate(invoice.LanguageCode, "invoicePaymentFailedMessage"), err.Error()), &tb.ReplyMarkup{})
		log.Errorln(errmsg)
		return ctx, err
	}

	// update the message and remove the button
	bot.tryEditMessage(c.Message, i18n.Translate(invoice.LanguageCode, "invoicePaidMessage"), &tb.ReplyMarkup{})
	return ctx, nil
}

// groupGetInviteLinkHandler is called when the invoice is paid and sends a one-time group invite link to the payer
func (bot *TipBot) groupGetInviteLinkHandler(event Event) {
	invoiceEvent := event.(*TicketInvoiceEvent)
	// take a cut
	// amount_bot := int64(invoiceEvent.Group.Ticket.Price * int64(invoiceEvent.Group.Ticket.Cut) / 100)

	type CreateChatInviteLink struct {
		ChatID             int64  `json:"chat_id"`
		Name               string `json:"name"`
		ExpiryDate         int    `json:"expiry_date"`
		MemberLimit        int    `json:"member_limit"`
		CreatesJoinRequest bool   `json:"creates_join_request"`
	}

	log.Infof("[groupGetInviteLinkHandler] group: %d", invoiceEvent.Chat.ID)
	params := map[string]interface {
	}{
		"chat_id":      invoiceEvent.Chat.ID,                                                                                // must be the chat ID of the group
		"name":         fmt.Sprintf("%s link for %s", GetUserStr(bot.Telegram.Me), GetUserStr(invoiceEvent.Payer.Telegram)), // the name of the invite link
		"member_limit": 1,                                                                                                   // only one user can join with this link
		// "expire_date":  time.Now().AddDate(0, 0, 1),                                                                         // expiry date of the invite link, add one day
		// "creates_join_request": false,                       // True, if users joining the chat via the link need to be approved by chat administrators. If True, member_limit can't be specified
	}
	data, err := bot.Telegram.Raw("createChatInviteLink", params)
	if err != nil {
		return
	}
	type Creator struct {
		ID        int64  `json:"id"`
		Isbot     bool   `json:"is_bot"`
		Firstname string `json:"first_name"`
		Username  string `json:"username"`
	}
	type Result struct {
		Invitelink         string  `json:"invite_link"`
		Name               string  `json:"name"`
		Creator            Creator `json:"creator"`
		Createsjoinrequest bool    `json:"creates_join_request"`
		Isprimary          bool    `json:"is_primary"`
		Isrevoked          bool    `json:"is_revoked"`
	}
	type ChatInviteLink struct {
		Ok     bool   `json:"ok"`
		Result Result `json:"result"`
	}

	var resp ChatInviteLink
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}

	bot.trySendMessage(invoiceEvent.Payer.Telegram, fmt.Sprintf(groupClickToJoinMessage, resp.Result.Invitelink))
	return
}

func (bot TipBot) addGroupHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	if m.Chat.Type == tb.ChatPrivate {
		return ctx, fmt.Errorf("not in group")
	}
	// parse command "/group add <grou_name> [<amount>]"
	splits := strings.Split(m.Text, " ")
	if len(splits) < 3 || len(m.Text) > 100 {
		bot.trySendMessage(m.Chat, groupAddGroupHelpMessage)
		return ctx, nil
	}
	groupname := strings.ToLower(splits[2])

	user := LoadUser(ctx)
	// check if the user is the owner of the group
	if !bot.isOwner(m.Chat, user.Telegram) {
		return ctx, fmt.Errorf("not owner")
	}

	if !bot.isAdminAndCanInviteUsers(m.Chat, bot.Telegram.Me) {
		bot.trySendMessage(m.Chat, groupBotIsNotAdminMessage)
		return ctx, fmt.Errorf("bot is not admin")
	}

	// check if the group with this name is already in db
	// only if a group with this name is owned by this user, it can be overwritten
	group := &Group{}
	tx := bot.GroupsDb.Where("name = ? COLLATE NOCASE", groupname).First(group)
	if tx.Error == nil {
		// if it is already added, check if this user is the admin
		if user.Telegram.ID != group.Owner.ID || group.ID != m.Chat.ID {
			bot.trySendMessage(m.Chat, groupNameExists)
			return ctx, fmt.Errorf("not owner")
		}
	}

	amount := int64(0) // default amount is zero
	if amount_str, err := getArgumentFromCommand(m.Text, 3); err == nil {
		amount, err = getAmount(amount_str)
		if err != nil {
			bot.trySendMessage(m.Sender, Translate(ctx, "lnurlInvalidAmountMessage"))
			return ctx, err
		}
	}

	ticket := &Ticket{
		Price:   amount,
		Memo:    "Ticket",
		Creator: user,
		Cut:     10,
	}

	group = &Group{
		Name:   groupname,
		Title:  m.Chat.Title,
		ID:     m.Chat.ID,
		Owner:  user.Telegram,
		Ticket: ticket,
	}

	bot.GroupsDb.Save(group)
	log.Infof("[group] Ticket of %d sat added to group %s.", group.Ticket.Price, group.Name)
	bot.trySendMessage(m.Chat, fmt.Sprintf(groupAddedMessage, str.MarkdownEscape(m.Chat.Title), group.Name, group.Ticket.Price, GetUserStrMd(bot.Telegram.Me), group.Name))

	return ctx, nil
}

func (bot *TipBot) createInvoiceGroupTicket(ctx context.Context, payer *lnbits.User, group *Group, memo string, callback int, callbackData string) (TicketInvoiceEvent, error) {
	invoice, err := group.Ticket.Creator.Wallet.Invoice(
		lnbits.InvoiceParams{
			Out:     false,
			Amount:  group.Ticket.Price,
			Memo:    memo,
			Webhook: internal.Configuration.Lnbits.WebhookServer},
		bot.Client)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Could not create an invoice: %s", err.Error())
		log.Errorln(errmsg)
		return TicketInvoiceEvent{}, err
	}
	invoiceEvent := TicketInvoiceEvent{
		Invoice: &Invoice{PaymentHash: invoice.PaymentHash,
			PaymentRequest: invoice.PaymentRequest,
			Amount:         group.Ticket.Price,
			Memo:           memo},
		User:         group.Ticket.Creator,
		Callback:     callback,
		CallbackData: callbackData,
		LanguageCode: ctx.Value("publicLanguageCode").(string),
		Payer:        payer,
		Chat:         &tb.Chat{ID: group.ID},
		Group:        group,
	}
	// save invoice struct for later use
	runtime.IgnoreError(bot.Bunt.Set(invoiceEvent))
	return invoiceEvent, nil
}
