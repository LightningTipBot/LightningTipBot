package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"

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
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

type JoinTicket struct {
	Sender           *tb.User    `json:"sender"`
	Message          *tb.Message `json:"message"`
	CreatedTimestamp time.Time   `json:"created_timestamp"`
	Ticket           *Ticket     `json:"ticket"`
}

func (jt JoinTicket) Key() string {
	return fmt.Sprintf("join-ticket:%d_%d", jt.Message.Chat.ID, jt.Sender.ID)
}

func (jt JoinTicket) Type() EventType {
	return EventTypeTicketInvoice
}

func (bot *TipBot) loadGroup(groupName string) (*Group, error) {
	group := &Group{}
	tx := bot.DB.Groups.Where("id = ? COLLATE NOCASE", groupName).First(group)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return group, nil
}

type Ticket struct {
	Price        int64        `json:"price"`
	Memo         string       `json:"memo"`
	Creator      *lnbits.User `gorm:"embedded;embeddedPrefix:creator_"`
	Cut          int64        `json:"cut"` // Percent to cut from ticket price
	BaseFee      int64        `json:"base_fee"`
	CutCheap     int64        `json:"cut_cheap"` // Percent to cut from ticket price
	BaseFeeCheap int64        `json:"base_fee_cheap"`
}
type Group struct {
	Name  string   `json:"name"`
	Title string   `json:"title"`
	ID    int64    `json:"id" gorm:"primaryKey"`
	Owner *tb.User `gorm:"embedded;embeddedPrefix:owner_"`
	// Chat   *tb.Chat `gorm:"embedded;embeddedPrefix:chat_"`
	Ticket *Ticket `gorm:"embedded;embeddedPrefix:ticket_"`
}
type CreateChatInviteLink struct {
	ChatID             int64  `json:"chat_id"`
	Name               string `json:"name"`
	ExpiryDate         int    `json:"expiry_date"`
	MemberLimit        int    `json:"member_limit"`
	CreatesJoinRequest bool   `json:"creates_join_request"`
}
type Creator struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Firstname string `json:"first_name"`
	Username  string `json:"username"`
}
type Result struct {
	InviteLink         string  `json:"invite_link"`
	Name               string  `json:"name"`
	Creator            Creator `json:"creator"`
	CreatesJoinRequest bool    `json:"creates_join_request"`
	IsPrimary          bool    `json:"is_primary"`
	IsRevoked          bool    `json:"is_revoked"`
}
type ChatInviteLink struct {
	Ok     bool   `json:"ok"`
	Result Result `json:"result"`
}

type TicketEvent struct {
	*storage.Base
	*InvoiceEvent
	Group *Group `gorm:"embedded;embeddedPrefix:group_"`
}

func (ticketEvent TicketEvent) Type() EventType {
	return EventTypeTicketInvoice
}
func (ticketEvent TicketEvent) Key() string {
	return ticketEvent.Base.ID
}

var (
	ticketPayConfirmationMenu = &tb.ReplyMarkup{ResizeKeyboard: true}
	btnPayTicket              = paymentConfirmationMenu.Data("✅ Pay", "pay_ticket")
)

const (
	groupInvoiceMemo           = "🎟 Ticket for group %s"
	groupInvoiceCommissionMemo = "🎟 Commission for group %s"
)

// groupHandler is called if the /group <cmd> command is invoked. It then decides with other
// handler to call depending on the <cmd> passed.
func (bot TipBot) groupHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	splits := strings.Split(m.Text, " ")
	// user := LoadUser(ctx)
	if len(splits) == 1 {
		// command: /group
		if ctx.Message().Private() {
			// /group help message
			bot.trySendMessage(ctx.Message().Chat, fmt.Sprintf(Translate(ctx, "groupHelpMessage"), GetUserStr(bot.Telegram.Me), GetUserStr(bot.Telegram.Me)))
		}
		// else {
		// 	if bot.isOwner(ctx.Message().Chat, user.Telegram) {
		// 		bot.trySendMessage(ctx.Message().Chat, fmt.Sprintf(Translate(ctx, "commandPrivateMessage"), GetUserStr(bot.Telegram.Me)))
		// 	}
		// }
		return ctx, nil
	} else if len(splits) > 1 {
		if splits[1] == "join" {
			return bot.groupRequestJoinHandler(ctx)
		}
		if splits[1] == "add" {
			return bot.addGroupHandler(ctx)
		}
		if splits[1] == "ticket" {
			return bot.handleJoinTicketPayWall(ctx)
		}
		if splits[1] == "remove" {
			// todo -- implement this
			// return bot.addGroupHandler(ctx, m)
		}
	}
	return ctx, nil
}

// groupRequestJoinHandler sends a payment request to the user who wants to join a group
func (bot TipBot) groupRequestJoinHandler(ctx intercept.Context) (intercept.Context, error) {
	user := LoadUser(ctx)
	// // reply only in private message
	if ctx.Chat().Type != tb.ChatPrivate {
		return ctx, fmt.Errorf("not private chat")
	}
	splits := strings.Split(ctx.Message().Text, " ")
	// if the command was /group join
	splitIdx := 1
	// we also have the simpler command /join that can be used
	// also by users who don't have an account with the bot yet
	if splits[0] == "/join" {
		splitIdx = 0
	}
	if len(splits) != splitIdx+2 || len(ctx.Message().Text) > 100 {
		bot.trySendMessage(ctx.Message().Chat, Translate(ctx, "groupJoinGroupHelpMessage"))
		return ctx, nil
	}
	groupName := strings.ToLower(splits[splitIdx+1])

	group := &Group{}
	tx := bot.DB.Groups.Where("name = ? COLLATE NOCASE", groupName).First(group)
	if tx.Error != nil {
		bot.trySendMessage(ctx.Message().Chat, Translate(ctx, "groupNotFoundMessage"))
		return ctx, fmt.Errorf("group not found")
	}

	// create tickets
	id := fmt.Sprintf("ticket:%d", group.ID)
	invoiceEvent := &InvoiceEvent{
		Base:         storage.New(storage.ID(id)),
		User:         group.Ticket.Creator,
		LanguageCode: ctx.Value("publicLanguageCode").(string),
		Payer:        user,
		Chat:         &tb.Chat{ID: group.ID},
		CallbackData: id,
	}
	ticketEvent := &TicketEvent{
		Base:         storage.New(storage.ID(id)),
		InvoiceEvent: invoiceEvent,
		Group:        group,
	}
	// if no price is set, then we don't need to pay
	if group.Ticket.Price == 0 {
		// save ticketevent for later
		runtime.IgnoreError(ticketEvent.Set(ticketEvent, bot.Bunt))
		bot.groupGetInviteLinkHandler(invoiceEvent)
		return ctx, nil
	}

	// create an invoice
	memo := fmt.Sprintf(groupInvoiceMemo, groupName)
	var err error
	invoiceEvent, err = bot.createGroupTicketInvoice(ctx, user, group, memo, InvoiceCallbackGroupTicket, id)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Could not create an invoice: %s", err.Error())
		bot.trySendMessage(user.Telegram, Translate(ctx, "errorTryLaterMessage"))
		log.Errorln(errmsg)
		return ctx, err
	}

	ticketEvent.InvoiceEvent = invoiceEvent
	// save ticketevent for later
	defer ticketEvent.Set(ticketEvent, bot.Bunt)

	// // if the user has enough balance, we send him a payment button
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		errmsg := fmt.Sprintf("[/group] Error: Could not get user balance: %s", err.Error())
		log.Errorln(errmsg)
		bot.trySendMessage(ctx.Message().Sender, Translate(ctx, "errorTryLaterMessage"))
		return ctx, errors.New(errors.GetBalanceError, err)
	}
	if balance >= group.Ticket.Price {
		return bot.groupSendPayButtonHandler(ctx, *ticketEvent)
	}

	// otherwise we send a payment request

	// create qr code
	qr, err := qrcode.Encode(invoiceEvent.PaymentRequest, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Failed to create QR code for invoice: %s", err.Error())
		bot.trySendMessage(user.Telegram, Translate(ctx, "errorTryLaterMessage"))
		log.Errorln(errmsg)
		return ctx, err
	}
	ticketEvent.Message = bot.trySendMessage(ctx.Message().Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", invoiceEvent.PaymentRequest)})
	bot.trySendMessage(ctx.Message().Sender, fmt.Sprintf(Translate(ctx, "groupPayInvoiceMessage"), groupName))
	return ctx, nil
}

// groupSendPayButtonHandler is invoked if the user has enough balance so a message with a
// pay button is sent to the user.
func (bot *TipBot) groupSendPayButtonHandler(ctx intercept.Context, ticket TicketEvent) (intercept.Context, error) {
	confirmText, confirmationMenu := bot.getSendPayButton(ctx, ticket)
	bot.trySendMessageEditable(ctx.Message().Chat, confirmText, confirmationMenu)
	return ctx, nil
}
func (bot *TipBot) getSendPayButton(ctx intercept.Context, ticket TicketEvent) (string, *tb.ReplyMarkup) {
	// object that holds all information about the send payment
	// // // create inline buttons
	btnPayTicket := ticketPayConfirmationMenu.Data(Translate(ctx, "payButtonMessage"), "pay_ticket", ticket.Base.ID)
	ticketPayConfirmationMenu.Inline(
		ticketPayConfirmationMenu.Row(
			btnPayTicket),
	)
	confirmText := fmt.Sprintf(Translate(ctx, "confirmPayInvoiceMessage"), ticket.Group.Ticket.Price)
	// if len(ticket.Group.Ticket.Memo) > 0 {
	// 	confirmText = confirmText + fmt.Sprintf(Translate(ctx, "confirmPayAppendMemo"), str.MarkdownEscape(ticket.Group.Ticket.Memo))
	// }
	return confirmText, ticketPayConfirmationMenu
}

// groupConfirmPayButtonHandler is invoked if th user clicks the pay button.
func (bot *TipBot) groupConfirmPayButtonHandler(ctx intercept.Context) (intercept.Context, error) {
	c := ctx.Callback()
	tx := &TicketEvent{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(ctx, tx.ID)
	defer mutex.UnlockWithContext(ctx, tx.ID)
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[groupConfirmPayButtonHandler] %s", err.Error())
		return ctx, err
	}
	ticketEvent := sn.(*TicketEvent)

	// onnly the correct user can press
	if ticketEvent.Payer.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	if !ticketEvent.Active {
		log.Errorf("[confirmPayHandler] send not active anymore")
		bot.tryEditMessage(c, i18n.Translate(ticketEvent.LanguageCode, "errorTryLaterMessage"), &tb.ReplyMarkup{})
		bot.tryDeleteMessage(c)
		return ctx, errors.Create(errors.NotActiveError)
	}
	defer ticketEvent.Set(ticketEvent, bot.Bunt)

	user := LoadUser(ctx)
	if user.Wallet == nil {
		bot.tryDeleteMessage(c)
		return ctx, errors.Create(errors.UserNoWalletError)
	}

	log.Infof("[/pay] Attempting %s's invoice %s (%d sat)", GetUserStr(user.Telegram), ticketEvent.ID, ticketEvent.Group.Ticket.Price)
	// // pay invoice
	_, err = user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: ticketEvent.Invoice.PaymentRequest}, bot.Client)
	if err != nil {
		errmsg := fmt.Sprintf("[/pay] Could not pay invoice of %s: %s", GetUserStr(user.Telegram), err)
		err = fmt.Errorf(i18n.Translate(ticketEvent.LanguageCode, "invoiceUndefinedErrorMessage"))
		if ticketEvent.Callback != InvoiceCallbackPayJoinTicket {
			bot.tryEditMessage(c, fmt.Sprintf(i18n.Translate(ticketEvent.LanguageCode, "invoicePaymentFailedMessage"), err.Error()), &tb.ReplyMarkup{})
		}
		log.Errorln(errmsg)
		return ctx, err
	}
	// if this was a join-ticket, we want to delete the invoice message

	// update the message and remove the button
	bot.tryEditMessage(c, i18n.Translate(ticketEvent.LanguageCode, "invoicePaidMessage"), &tb.ReplyMarkup{})

	return ctx, nil
}

// groupGetInviteLinkHandler is called when the invoice is paid and sends a one-time group invite link to the payer
func (bot *TipBot) groupGetInviteLinkHandler(event Event) {
	invoiceEvent := event.(*InvoiceEvent)
	// take a cut
	// amount_bot := int64(ticketEvent.Group.Ticket.Price * int64(ticketEvent.Group.Ticket.Cut) / 100)

	log.Infof(invoiceEvent.CallbackData)
	ticketEvent := &TicketEvent{Base: storage.New(storage.ID(invoiceEvent.CallbackData))}
	err := bot.Bunt.Get(ticketEvent)
	if err != nil {
		log.Errorf("[groupGetInviteLinkHandler] %s", err.Error())
		return
	}

	log.Infof("[groupGetInviteLinkHandler] group: %d", ticketEvent.Chat.ID)
	params := map[string]interface {
	}{
		"chat_id":      ticketEvent.Group.ID,                                                                               // must be the chat ID of the group
		"name":         fmt.Sprintf("%s link for %s", GetUserStr(bot.Telegram.Me), GetUserStr(ticketEvent.Payer.Telegram)), // the name of the invite link
		"member_limit": 1,                                                                                                  // only one user can join with this link
		// "expire_date":  time.Now().AddDate(0, 0, 1),                                                                         // expiry date of the invite link, add one day
		// "creates_join_request": false,                       // True, if users joining the chat via the link need to be approved by chat administrators. If True, member_limit can't be specified
	}
	data, err := bot.Telegram.Raw("createChatInviteLink", params)
	if err != nil {
		return
	}

	var resp ChatInviteLink
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}

	if ticketEvent.Message != nil {
		bot.tryDeleteMessage(ticketEvent.Message)
		// do balance check for keyboard update
		_, err = bot.GetUserBalance(ticketEvent.Payer)
		if err != nil {
			errmsg := fmt.Sprintf("could not get balance of user %s", GetUserStr(ticketEvent.Payer.Telegram))
			log.Errorln(errmsg)
		}
		bot.trySendMessage(ticketEvent.Payer.Telegram, i18n.Translate(ticketEvent.LanguageCode, "invoicePaidText"))
	}

	// send confirmation text with the ticket to the user
	bot.trySendMessage(ticketEvent.Payer.Telegram, fmt.Sprintf(i18n.Translate(ticketEvent.LanguageCode, "groupClickToJoinMessage"), resp.Result.InviteLink, ticketEvent.Group.Title))

	// send a notification to the group that sold the ticket
	bot.trySendMessage(&tb.Chat{ID: ticketEvent.Group.ID}, fmt.Sprintf(i18n.Translate(ticketEvent.LanguageCode, "groupTicketIssuedGroupMessage"), GetUserStrMd(ticketEvent.Payer.Telegram)))

	// take a commission
	ticketSat := ticketEvent.Group.Ticket.Price
	if commissionSat := getTicketCommission(ticketEvent.Group.Ticket); commissionSat > 0 {
		me, err := GetUser(bot.Telegram.Me, *bot)
		if err != nil {
			log.Errorf("[groupGetInviteLinkHandler] Could not get bot user from DB: %s", err.Error())
			return
		}
		ticketSat = ticketEvent.Group.Ticket.Price - commissionSat
		invoice, err := me.Wallet.Invoice(
			lnbits.InvoiceParams{
				Out:     false,
				Amount:  commissionSat,
				Memo:    "🎟 Ticket commission for group " + ticketEvent.Group.Title,
				Webhook: internal.Configuration.Lnbits.WebhookServer},
			bot.Client)
		if err != nil {
			errmsg := fmt.Sprintf("[/invoice] Could not create an invoice: %s", err.Error())
			log.Errorln(errmsg)
			return
		}
		_, err = ticketEvent.User.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: invoice.PaymentRequest}, bot.Client)
		if err != nil {
			errmsg := fmt.Sprintf("[groupGetInviteLinkHandler] Could not pay commission of %s: %s", GetUserStr(ticketEvent.User.Telegram), err)
			log.Errorln(errmsg)
			return
		}
		// do balance check for keyboard update
		_, err = bot.GetUserBalance(ticketEvent.User)
		if err != nil {
			errmsg := fmt.Sprintf("could not get balance of user %s", GetUserStr(ticketEvent.Payer.Telegram))
			log.Errorln(errmsg)
		}
		bot.trySendMessage(ticketEvent.User.Telegram, fmt.Sprintf(i18n.Translate(ticketEvent.LanguageCode, "groupReceiveTicketInvoiceCommission"), ticketSat, commissionSat, ticketEvent.Group.Title, GetUserStrMd(ticketEvent.Payer.Telegram)))
	} else {
		bot.trySendMessage(ticketEvent.User.Telegram, fmt.Sprintf(i18n.Translate(ticketEvent.LanguageCode, "groupReceiveTicketInvoice"), ticketSat, ticketEvent.Group.Title, GetUserStrMd(ticketEvent.Payer.Telegram)))
	}
}

func (bot TipBot) handleJoinTicketPayWall(ctx intercept.Context) (intercept.Context, error) {
	var err error
	var cmd string
	if cmd, err = getArgumentFromCommand(ctx.Message().Text, 2); err == nil {
		switch strings.TrimSpace(strings.ToLower(cmd)) {
		case "del":
			fallthrough
		case "delete":
			fallthrough
		case "remove":
			return bot.removeJoinTicketPayWallHandler(ctx)
		default:
			return bot.addJoinTicketPayWallHandler(ctx)
		}
	}
	return ctx, err
}
func (bot TipBot) removeJoinTicketPayWallHandler(ctx intercept.Context) (intercept.Context, error) {
	groupName := strconv.FormatInt(ctx.Chat().ID, 10)
	tx := bot.DB.Groups.Where("id = ? COLLATE NOCASE", groupName).Delete(&Group{})
	if tx.Error != nil {
		return ctx, tx.Error
	}
	bot.trySendMessage(ctx.Message().Chat, "🎟 Ticket removed.")
	return ctx, nil
}

// addJoinTicketPayWallHandler is invoked if the user calls the "/group ticket" command
func (bot TipBot) addJoinTicketPayWallHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	if m.Chat.Type == tb.ChatPrivate {
		bot.trySendMessage(m.Chat, Translate(ctx, "groupAddGroupHelpMessage"))
		return ctx, fmt.Errorf("not in group")
	}
	// parse command "/group ticket <grou_name> [<amount>]"
	splits := strings.Split(m.Text, " ")
	if len(splits) < 3 || len(m.Text) > 100 {
		bot.trySendMessage(m.Chat, Translate(ctx, "groupAddGroupHelpMessage"))
		return ctx, nil
	}
	groupName := strconv.FormatInt(ctx.Chat().ID, 10)

	user := LoadUser(ctx)
	// check if the user is the owner of the group
	if !bot.isOwner(m.Chat, user.Telegram) {
		return ctx, fmt.Errorf("not owner")
	}

	if !bot.isAdminAndCanInviteUsers(m.Chat, bot.Telegram.Me) {
		bot.trySendMessage(m.Chat, Translate(ctx, "groupBotIsNotAdminMessage"))
		return ctx, fmt.Errorf("bot is not admin")
	}

	// check if the group with this name is already in db
	// only if a group with this name is owned by this user, it can be overwritten
	group := &Group{}
	tx := bot.DB.Groups.Where("id = ? COLLATE NOCASE", groupName).First(group)
	if tx.Error == nil {
		// if it is already added, check if this user is the admin
		if user.Telegram.ID != group.Owner.ID || group.ID != m.Chat.ID {
			bot.trySendMessage(m.Chat, Translate(ctx, "groupNameExists"))
			return ctx, fmt.Errorf("not owner")
		}
	}

	amount := int64(0) // default amount is zero
	if amount_str, err := getArgumentFromCommand(m.Text, 2); err == nil {
		amount, err = GetAmount(amount_str)
		if err != nil {
			bot.trySendMessage(m.Sender, Translate(ctx, "lnurlInvalidAmountMessage"))
			return ctx, err
		}
	}

	ticket := &Ticket{
		Price:        amount,
		Memo:         "Ticket for group " + groupName,
		Creator:      user,
		Cut:          2,
		BaseFee:      100,
		CutCheap:     10,
		BaseFeeCheap: 10,
	}

	group = &Group{
		Name:   groupName,
		Title:  m.Chat.Title,
		ID:     m.Chat.ID,
		Owner:  user.Telegram,
		Ticket: ticket,
	}

	bot.DB.Groups.Save(group)
	log.Infof("[group] Ticket of %d sat added to group %s.", group.Ticket.Price, group.Name)
	bot.trySendMessage(m.Chat, Translate(ctx, "groupAddedMessagePublic"))

	return ctx, nil
}

// addGroupHandler is invoked if the user calls the "/group add" command
func (bot TipBot) addGroupHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	if m.Chat.Type == tb.ChatPrivate {
		bot.trySendMessage(m.Chat, Translate(ctx, "groupAddGroupHelpMessage"))
		return ctx, fmt.Errorf("not in group")
	}
	// parse command "/group add <grou_name> [<amount>]"
	splits := strings.Split(m.Text, " ")
	if len(splits) < 3 || len(m.Text) > 100 {
		bot.trySendMessage(m.Chat, Translate(ctx, "groupAddGroupHelpMessage"))
		return ctx, nil
	}
	groupName := strings.ToLower(splits[2])

	user := LoadUser(ctx)
	// check if the user is the owner of the group
	if !bot.isOwner(m.Chat, user.Telegram) {
		return ctx, fmt.Errorf("not owner")
	}

	if !bot.isAdminAndCanInviteUsers(m.Chat, bot.Telegram.Me) {
		bot.trySendMessage(m.Chat, Translate(ctx, "groupBotIsNotAdminMessage"))
		return ctx, fmt.Errorf("bot is not admin")
	}

	// check if the group with this name is already in db
	// only if a group with this name is owned by this user, it can be overwritten
	group := &Group{}
	tx := bot.DB.Groups.Where("name = ? COLLATE NOCASE", groupName).First(group)
	if tx.Error == nil {
		// if it is already added, check if this user is the admin
		if user.Telegram.ID != group.Owner.ID || group.ID != m.Chat.ID {
			bot.trySendMessage(m.Chat, Translate(ctx, "groupNameExists"))
			return ctx, fmt.Errorf("not owner")
		}
	}

	amount := int64(0) // default amount is zero
	if amount_str, err := getArgumentFromCommand(m.Text, 3); err == nil {
		amount, err = GetAmount(amount_str)
		if err != nil {
			bot.trySendMessage(m.Sender, Translate(ctx, "lnurlInvalidAmountMessage"))
			return ctx, err
		}
	}

	ticket := &Ticket{
		Price:        amount,
		Memo:         "Ticket for group " + groupName,
		Creator:      user,
		Cut:          2,
		BaseFee:      100,
		CutCheap:     10,
		BaseFeeCheap: 10,
	}

	group = &Group{
		Name:   groupName,
		Title:  m.Chat.Title,
		ID:     m.Chat.ID,
		Owner:  user.Telegram,
		Ticket: ticket,
	}

	bot.DB.Groups.Save(group)
	log.Infof("[group] Ticket of %d sat added to group %s.", group.Ticket.Price, group.Name)
	bot.trySendMessage(m.Chat, fmt.Sprintf(Translate(ctx, "groupAddedMessagePrivate"), str.MarkdownEscape(m.Chat.Title), group.Name, group.Ticket.Price, GetUserStrMd(bot.Telegram.Me), group.Name))

	return ctx, nil
}

// createGroupTicketInvoice produces an invoice for the group ticket with a
// callback that then calls groupGetInviteLinkHandler upton payment
func (bot *TipBot) createGroupTicketInvoice(ctx context.Context, payer *lnbits.User, group *Group, memo string, callback int, callbackData string) (*InvoiceEvent, error) {
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
		return &InvoiceEvent{}, err
	}

	// save the invoice event
	id := fmt.Sprintf("invoice:%s", invoice.PaymentHash)
	invoiceEvent := &InvoiceEvent{
		Base: storage.New(storage.ID(id)),
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
	}
	// add result to persistent struct
	runtime.IgnoreError(invoiceEvent.Set(invoiceEvent, bot.Bunt))
	return invoiceEvent, nil
}
