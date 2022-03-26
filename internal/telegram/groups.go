package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
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
	tb "gopkg.in/telebot.v3"
)

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

var (
	groupAddGroupHelpMessage            = "📖 Oops, that didn't work. Please try again.\nUsage: `/group add <group_name> [<amount>]`\nExample: `/group add TheBestBitcoinGroup 1000`"
	grouJoinGroupHelpMessage            = "📖 Oops, that didn't work. Please try again.\nUsage: `/join <group_name>`\nExample: `/join TheBestBitcoinGroup`"
	groupClickToJoinMessage             = "🎟 [Click here](%s) 👈 to join `%s`."
	groupInvoiceMemo                    = "Ticket for group %s"
	groupPayInvoiceMessage              = "🎟 To join the group %s, pay the invoice above."
	groupBotIsNotAdminMessage           = "🚫 Oops, that didn't work. You must make me admin and grant me rights to invite users."
	groupNameExists                     = "🚫 A group with this name already exists. Please choose a different name."
	groupAddedMessage                   = "🎟 Tickets for group `%s` added.\nAlias: `%s` Price: %d sat\n\nTo request a ticket for this group, start a private chat with %s and write `/join %s`."
	groupNotFoundMessage                = "🚫 Could not find a group with this name."
	groupReceiveTicketInvoiceCommission = "🎟 You received *%d sat* (excl. %d sat commission) for a ticket for group `%s` paid by user %s."
	groupReceiveTicketInvoice           = "🎟 You received *%d sat* for a ticket for group `%s` paid by user %s."
)

func (bot TipBot) groupHandler(handler intercept.Handler) (intercept.Handler, error) {
	m := handler.Message()
	splits := strings.Split(m.Text, " ")
	if len(splits) == 1 {
		return handler, nil
	} else if len(splits) > 1 {
		if splits[1] == "join" {
			return bot.groupRequestJoinHandler(handler)
		}
		if splits[1] == "add" {
			return bot.addGroupHandler(handler)
		}
		if splits[1] == "remove" {
			// todo -- implement this
			// return bot.addGroupHandler(ctx, m)
		}
	}
	return handler, nil
}

// groupRequestJoinHandler sends a payment request to the user who wants to join a group
func (bot TipBot) groupRequestJoinHandler(handler intercept.Handler) (intercept.Handler, error) {
	user := LoadUser(handler.Ctx)
	// // reply only in private message
	if handler.Chat().Type != tb.ChatPrivate {
		return handler, fmt.Errorf("not private chat")
	}
	splits := strings.Split(handler.Text(), " ")
	// if the command was /group join
	splitIdx := 1
	// we also have the simpler command /join that can be used
	// also by users who don't have an account with the bot yet
	if splits[0] == "/join" {
		splitIdx = 0
	}
	if len(splits) != splitIdx+2 || len(handler.Message().Text) > 100 {
		bot.trySendMessage(handler.Message().Chat, grouJoinGroupHelpMessage)
		return handler, nil
	}
	groupName := strings.ToLower(splits[splitIdx+1])

	group := &Group{}
	tx := bot.GroupsDb.Where("name = ? COLLATE NOCASE", groupName).First(group)
	if tx.Error != nil {
		bot.trySendMessage(handler.Message().Chat, groupNotFoundMessage)
		return handler, fmt.Errorf("group not found")
	}

	// create tickets
	id := fmt.Sprintf("ticket:%d", group.ID)
	invoiceEvent := &InvoiceEvent{
		Base:         storage.New(storage.ID(id)),
		User:         group.Ticket.Creator,
		LanguageCode: handler.Ctx.Value("publicLanguageCode").(string),
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
		return handler, nil
	}

	// create an invoice
	memo := fmt.Sprintf(groupInvoiceMemo, groupName)
	var err error
	invoiceEvent, err = bot.createGroupTicketInvoice(handler.Ctx, user, group, memo, InvoiceCallbackGroupTicket, id)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Could not create an invoice: %s", err.Error())
		bot.trySendMessage(user.Telegram, Translate(handler.Ctx, "errorTryLaterMessage"))
		log.Errorln(errmsg)
		return handler, err
	}

	ticketEvent.InvoiceEvent = invoiceEvent
	// save ticketevent for later
	defer ticketEvent.Set(ticketEvent, bot.Bunt)

	// // if the user has enough balance, we send him a payment button
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		errmsg := fmt.Sprintf("[/group] Error: Could not get user balance: %s", err.Error())
		log.Errorln(errmsg)
		bot.trySendMessage(handler.Message().Sender, Translate(handler.Ctx, "errorTryLaterMessage"))
		return handler, errors.New(errors.GetBalanceError, err)
	}
	if balance >= group.Ticket.Price {
		return bot.groupSendPayButtonHandler(handler, *ticketEvent)
	}

	// otherwise we send a payment request

	// create qr code
	qr, err := qrcode.Encode(invoiceEvent.PaymentRequest, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Failed to create QR code for invoice: %s", err.Error())
		bot.trySendMessage(user.Telegram, Translate(handler.Ctx, "errorTryLaterMessage"))
		log.Errorln(errmsg)
		return handler, err
	}
	ticketEvent.Message = bot.trySendMessage(handler.Message().Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", invoiceEvent.PaymentRequest)})
	bot.trySendMessage(handler.Message().Sender, fmt.Sprintf(groupPayInvoiceMessage, groupName))
	return handler, nil
}

func (bot *TipBot) groupSendPayButtonHandler(handler intercept.Handler, ticket TicketEvent) (intercept.Handler, error) {
	// object that holds all information about the send payment
	// // // create inline buttons
	btnPayTicket := ticketPayConfirmationMenu.Data(Translate(handler.Ctx, "payButtonMessage"), "pay_ticket", ticket.Base.ID)
	ticketPayConfirmationMenu.Inline(
		ticketPayConfirmationMenu.Row(
			btnPayTicket),
	)
	confirmText := fmt.Sprintf(Translate(handler.Ctx, "confirmPayInvoiceMessage"), ticket.Group.Ticket.Price)
	if len(ticket.Group.Ticket.Memo) > 0 {
		confirmText = confirmText + fmt.Sprintf(Translate(handler.Ctx, "confirmPayAppendMemo"), str.MarkdownEscape(ticket.Group.Ticket.Memo))
	}
	bot.trySendMessageEditable(handler.Message().Chat, confirmText, ticketPayConfirmationMenu)
	return handler, nil
}

func (bot *TipBot) groupConfirmPayButtonHandler(handler intercept.Handler) (intercept.Handler, error) {
	tx := &TicketEvent{Base: storage.New(storage.ID(handler.Callback().Data))}
	mutex.LockWithContext(handler.Ctx, tx.ID)
	defer mutex.UnlockWithContext(handler.Ctx, tx.ID)
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[groupConfirmPayButtonHandler] %s", err.Error())
		return handler, err
	}
	ticketEvent := sn.(*TicketEvent)
	c := handler.Callback()
	// onnly the correct user can press
	if ticketEvent.Payer.Telegram.ID != c.Sender.ID {
		return handler, errors.Create(errors.UnknownError)
	}
	if !ticketEvent.Active {
		log.Errorf("[confirmPayHandler] send not active anymore")
		bot.tryEditMessage(c, i18n.Translate(ticketEvent.LanguageCode, "errorTryLaterMessage"), &tb.ReplyMarkup{})
		bot.tryDeleteMessage(c)
		return handler, errors.Create(errors.NotActiveError)
	}
	defer ticketEvent.Set(ticketEvent, bot.Bunt)

	user := LoadUser(handler.Ctx)
	if user.Wallet == nil {
		bot.tryDeleteMessage(c)
		return handler, errors.Create(errors.UserNoWalletError)
	}

	log.Infof("[/pay] Attempting %s's invoice %s (%d sat)", GetUserStr(user.Telegram), ticketEvent.ID, ticketEvent.Group.Ticket.Price)
	// // pay invoice
	_, err = user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: ticketEvent.Invoice.PaymentRequest}, bot.Client)
	if err != nil {
		errmsg := fmt.Sprintf("[/pay] Could not pay invoice of %s: %s", GetUserStr(user.Telegram), err)
		err = fmt.Errorf(i18n.Translate(ticketEvent.LanguageCode, "invoiceUndefinedErrorMessage"))
		bot.tryEditMessage(c, fmt.Sprintf(i18n.Translate(ticketEvent.LanguageCode, "invoicePaymentFailedMessage"), err.Error()), &tb.ReplyMarkup{})
		log.Errorln(errmsg)
		return handler, err
	}

	// update the message and remove the button
	bot.tryEditMessage(c, i18n.Translate(ticketEvent.LanguageCode, "invoicePaidMessage"), &tb.ReplyMarkup{})
	return handler, nil
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

	bot.trySendMessage(ticketEvent.Payer.Telegram, fmt.Sprintf(groupClickToJoinMessage, resp.Result.InviteLink, ticketEvent.Group.Title))

	// take a commission
	ticketSat := ticketEvent.Group.Ticket.Price
	if ticketEvent.Group.Ticket.Price > 20 {
		me, err := GetUser(bot.Telegram.Me, *bot)
		if err != nil {
			log.Errorf("[groupGetInviteLinkHandler] Could not get bot user from DB: %s", err.Error())
			return
		}

		// 2% cut + 100 sat base fee
		commissionSat := ticketEvent.Group.Ticket.Price*ticketEvent.Group.Ticket.Cut/100 + ticketEvent.Group.Ticket.BaseFee
		if ticketEvent.Group.Ticket.Price <= 1000 {
			// if < 1000, then 10% cut + 10 sat base fee
			commissionSat = ticketEvent.Group.Ticket.Price*ticketEvent.Group.Ticket.CutCheap/100 + ticketEvent.Group.Ticket.BaseFeeCheap
		}

		ticketSat = ticketEvent.Group.Ticket.Price - commissionSat
		invoice, err := me.Wallet.Invoice(
			lnbits.InvoiceParams{
				Out:     false,
				Amount:  commissionSat,
				Memo:    "Ticket commission for group " + ticketEvent.Group.Title,
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
			err = fmt.Errorf(i18n.Translate(ticketEvent.LanguageCode, "invoiceUndefinedErrorMessage"))
			log.Errorln(errmsg)
			return
		}
		// do balance check for keyboard update
		_, err = bot.GetUserBalance(ticketEvent.User)
		if err != nil {
			errmsg := fmt.Sprintf("could not get balance of user %s", GetUserStr(ticketEvent.Payer.Telegram))
			log.Errorln(errmsg)
		}
		bot.trySendMessage(ticketEvent.User.Telegram, fmt.Sprintf(groupReceiveTicketInvoiceCommission, ticketSat, commissionSat, ticketEvent.Group.Title, GetUserStr(ticketEvent.Payer.Telegram)))
	} else {
		bot.trySendMessage(ticketEvent.User.Telegram, fmt.Sprintf(groupReceiveTicketInvoice, ticketSat, ticketEvent.Group.Title, GetUserStr(ticketEvent.Payer.Telegram)))
	}
	return
}

func (bot TipBot) addGroupHandler(handler intercept.Handler) (intercept.Handler, error) {
	m := handler.Message()
	if m.Chat.Type == tb.ChatPrivate {
		return handler, fmt.Errorf("not in group")
	}
	// parse command "/group add <grou_name> [<amount>]"
	splits := strings.Split(m.Text, " ")
	if len(splits) < 3 || len(m.Text) > 100 {
		bot.trySendMessage(m.Chat, groupAddGroupHelpMessage)
		return handler, nil
	}
	groupName := strings.ToLower(splits[2])

	user := LoadUser(handler.Ctx)
	// check if the user is the owner of the group
	if !bot.isOwner(m.Chat, user.Telegram) {
		return handler, fmt.Errorf("not owner")
	}

	if !bot.isAdminAndCanInviteUsers(m.Chat, bot.Telegram.Me) {
		bot.trySendMessage(m.Chat, groupBotIsNotAdminMessage)
		return handler, fmt.Errorf("bot is not admin")
	}

	// check if the group with this name is already in db
	// only if a group with this name is owned by this user, it can be overwritten
	group := &Group{}
	tx := bot.GroupsDb.Where("name = ? COLLATE NOCASE", groupName).First(group)
	if tx.Error == nil {
		// if it is already added, check if this user is the admin
		if user.Telegram.ID != group.Owner.ID || group.ID != m.Chat.ID {
			bot.trySendMessage(m.Chat, groupNameExists)
			return handler, fmt.Errorf("not owner")
		}
	}

	amount := int64(0) // default amount is zero
	if amount_str, err := getArgumentFromCommand(m.Text, 3); err == nil {
		amount, err = getAmount(amount_str)
		if err != nil {
			bot.trySendMessage(m.Sender, Translate(handler.Ctx, "lnurlInvalidAmountMessage"))
			return handler, err
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

	bot.GroupsDb.Save(group)
	log.Infof("[group] Ticket of %d sat added to group %s.", group.Ticket.Price, group.Name)
	bot.trySendMessage(m.Chat, fmt.Sprintf(groupAddedMessage, str.MarkdownEscape(m.Chat.Title), group.Name, group.Ticket.Price, GetUserStrMd(bot.Telegram.Me), group.Name))

	return handler, nil
}

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
