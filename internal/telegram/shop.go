package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	"github.com/eko/gocache/store"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

type ShopView struct {
	ID             string
	ShopID         string
	ShopOwner      *lnbits.User
	Page           int
	Message        *tb.Message
	StatusMessages []*tb.Message
}

type ShopItem struct {
	ID           string       `json:"ID"`          // ID of the tx object in bunt db
	ShopID       string       `json:"shopID"`      // ID of the shop
	Owner        *lnbits.User `json:"owner"`       // Owner of the item
	Type         string       `json:"Type"`        // Type of the tx object in bunt db
	FileIDs      []string     `json:"fileIDs"`     // Telegram fileID of the item files
	FileTypes    []string     `json:"fileTypes"`   // Telegram file type of the item files
	Title        string       `json:"title"`       // Title of the item
	Description  string       `json:"description"` // Description of the item
	Price        int          `json:"price"`       // price of the item
	NSold        int          `json:"nSold"`       // number of times item was sold
	TbPhoto      *tb.Photo    `json:"tbPhoto"`     // Telegram photo object
	LanguageCode string       `json:"languagecode"`
	MaxFiles     int          `json:"maxFiles"`
}

type Shop struct {
	*transaction.Base
	ID           string              `json:"ID"`          // holds the ID of the tx object in bunt db
	Owner        *lnbits.User        `json:"owner"`       // owner of the shop
	Type         string              `json:"Type"`        // type of the shop
	Title        string              `json:"title"`       // Title of the item
	Description  string              `json:"description"` // Description of the item
	ItemIds      []string            `json:"ItemsIDs"`    //
	Items        map[string]ShopItem `json:"Items"`       //
	LanguageCode string              `json:"languagecode"`
	ShopsID      string              `json:"shopsID"`
	MaxItems     int                 `json:"maxItems"`
}

type Shops struct {
	*transaction.Base
	ID       string       `json:"ID"`    // holds the ID of the tx object in bunt db
	Owner    *lnbits.User `json:"owner"` // owner of the shop
	Shops    []string     `json:"shop"`  //
	MaxShops int          `json:"maxShops"`
}

const (
	MAX_SHOPS             = 10
	MAX_ITEMS_PER_SHOP    = 20
	MAX_FILES_PER_ITEM    = 20
	SHOP_TITLE_MAX_LENGTH = 50
	ITEM_TITLE_MAX_LENGTH = 1500
)

func (shop *Shop) getItem(itemId string) (item ShopItem, ok bool) {
	item, ok = shop.Items[itemId]
	return
}

var (
	shopKeyboard         = &tb.ReplyMarkup{ResizeReplyKeyboard: false}
	browseShopButton     = shopKeyboard.Data("Browse shops", "shops_browse")
	shopNewShopButton    = shopKeyboard.Data("New Shop", "shops_newshop")
	shopDeleteShopButton = shopKeyboard.Data("Delete Shop", "shops_deleteshop")
	shopLinkShopButton   = shopKeyboard.Data("Shop links", "shops_linkshop")
	shopSettingsButton   = shopKeyboard.Data("Settings", "shops_settings")
	shopShopsButton      = shopKeyboard.Data("Back", "shops_shops")

	shopAddItemButton  = shopKeyboard.Data("New item", "shop_additem")
	shopNextitemButton = shopKeyboard.Data(">", "shop_nextitem")
	shopPrevitemButton = shopKeyboard.Data("<", "shop_previtem")
	shopBuyitemButton  = shopKeyboard.Data("Buy", "shop_buyitem")

	shopSelectButton           = shopKeyboard.Data("SHOP SELECTOR", "select_shop")        // shop slectino buttons
	shopDeleteSelectButton     = shopKeyboard.Data("DELETE SHOP SELECTOR", "delete_shop") // shop slectino buttons
	shopLinkSelectButton       = shopKeyboard.Data("LINK SHOP SELECTOR", "link_shop")     // shop slectino buttons
	shopItemPriceButton        = shopKeyboard.Data("Price", "shop_itemprice")
	shopItemDeleteButton       = shopKeyboard.Data("Delete", "shop_itemdelete")
	shopItemTitleButton        = shopKeyboard.Data("Set title", "shop_itemtitle")
	shopItemAddFileButton      = shopKeyboard.Data("Add file", "shop_itemaddfile")
	shopItemSettingsButton     = shopKeyboard.Data("Item settings", "shop_itemsettings")
	shopItemSettingsBackButton = shopKeyboard.Data("Back", "shop_itemsettingsback")
)

// shopItemPriceHandler is invoked when the user presses the item settings button to set a price
func (bot *TipBot) shopItemPriceHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != c.Data {
		log.Error("[shopItemPriceHandler] item id mismatch")
		return
	}
	// We need to save the pay state in the user state so we can load the payment in the next handler
	SetUserState(user, bot, lnbits.UserStateShopItemSendPrice, item.ID)
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("💯 Enter item price."), tb.ForceReply)
}

// enterShopItemPriceHandler is invoked when the user enters a price amount
func (bot *TipBot) enterShopItemPriceHandler(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		return
	}
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != user.StateData {
		log.Error("[shopItemPriceHandler] item id mismatch")
		return
	}
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}

	amount, err := getAmount(m.Text)
	if err != nil {
		log.Warnf("[enterShopItemPriceHandler] %s", err.Error())
		bot.trySendMessage(m.Sender, Translate(ctx, "lnurlInvalidAmountMessage"))
		ResetUserState(user, bot)
		return //err, 0
	}

	item.Price = amount
	shop.Items[item.ID] = item
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	bot.tryDeleteMessage(m)
	bot.sendStatusMessage(ctx, m.Sender, fmt.Sprintf("✅ Price set."))
	ResetUserState(user, bot)
	time.Sleep(time.Duration(2) * time.Second)
	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	bot.displayShopItem(ctx, shopView.Message, shop)
}

// shopItemPriceHandler is invoked when the user presses the item settings button to set a item title
func (bot *TipBot) shopItemTitleHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != c.Data {
		log.Error("[shopItemTitleHandler] item id mismatch")
		return
	}
	// We need to save the pay state in the user state so we can load the payment in the next handler
	SetUserState(user, bot, lnbits.UserStateShopItemSendTitle, item.ID)
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("⌨️ Enter item title."), tb.ForceReply)
}

// enterShopItemTitleHandler is invoked when the user enters a title of the item
func (bot *TipBot) enterShopItemTitleHandler(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		return
	}
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != user.StateData {
		log.Error("[enterShopItemTitleHandler] item id mismatch")
		return
	}
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}

	// crop item title
	if len(m.Text) > ITEM_TITLE_MAX_LENGTH {
		m.Text = m.Text[:ITEM_TITLE_MAX_LENGTH]
	}
	item.Title = m.Text
	shop.Items[item.ID] = item
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	bot.tryDeleteMessage(m)
	bot.sendStatusMessage(ctx, m.Sender, fmt.Sprintf("✅ Title set."))
	ResetUserState(user, bot)
	time.Sleep(time.Duration(2) * time.Second)
	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	bot.displayShopItem(ctx, shopView.Message, shop)
}

// shopItemSettingsHandler is invoked when the user presses the item settings button
func (bot *TipBot) shopItemSettingsHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != c.Data {
		log.Error("[shopItemSettingsHandler] item id mismatch")
		return
	}
	bot.tryEditMessage(shopView.Message, item.TbPhoto, bot.shopItemSettingsMenu(ctx, shop, &item))
}

// shopItemPriceHandler is invoked when the user presses the item settings button to set a item title
func (bot *TipBot) shopItemDeleteHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		return
	}
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return
	}

	delete(shop.Items, item.ID)
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))

	ResetUserState(user, bot)
	bot.sendStatusMessage(ctx, c.Message.Chat, fmt.Sprintf("✅ Item deleted."))
	time.Sleep(time.Duration(2) * time.Second)
	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	if shopView.Page > 0 {
		shopView.Page--
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	bot.displayShopItem(ctx, shopView.Message, shop)

	// shopView, err = bot.getUserShopview(ctx, user)
	// if err == nil {
	// 	shopView.StatusMessages = append(shopView.StatusMessages, statusMsg)
	// 	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	// 	time.Sleep(time.Duration(2) * time.Second)
	// 	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// 	if shopView.Page > 0 {
	// 		shopView.Page--
	// 	}
	// 	bot.displayShopItem(ctx, shopView.Message, shop)
	// }
	// bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
}

// displayShopItemHandler is invoked when the user presses the back button in the item settings
func (bot *TipBot) displayShopItemHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != c.Data {
		log.Error("[shopItemSettingsHandler] item id mismatch")
		return
	}
	bot.displayShopItem(ctx, c.Message, shop)
}

// shopNextItemHandler is invoked when the user presses the next item button
func (bot *TipBot) shopNextItemButtonHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	// shopView, err := bot.Cache.Get(fmt.Sprintf("shopview-%d", user.Telegram.ID))
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if shopView.Page < len(shop.Items)-1 {
		shopView.Page++
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
		shop, err = bot.getShop(ctx, shopView.ShopID)
		bot.displayShopItem(ctx, c.Message, shop)
	}
}

// shopPrevItemButtonHandler is invoked when the user presses the previous item button
func (bot *TipBot) shopPrevItemButtonHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	if shopView.Page == 0 {
		c.Message.Text = "/shops " + shopView.ShopOwner.Telegram.Username
		bot.shopsHandler(ctx, c.Message)
		return
	}
	if shopView.Page > 0 {
		shopView.Page--
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	shop, err := bot.getShop(ctx, shopView.ShopID)
	bot.displayShopItem(ctx, c.Message, shop)
}

// displayShopItem renders the current item in the shopView
// requires that the shopview page is already set accordingly
// m is the message that will be edited
func (bot *TipBot) displayShopItem(ctx context.Context, m *tb.Message, shop *Shop) *tb.Message {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[displayShopItem] %s", err.Error())
		return nil
	}
	// failsafe: if the page is out of bounds, reset it
	if shopView.Page >= len(shop.Items) {
		shopView.Page = len(shop.Items) - 1
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]

	if len(item.Title) > 0 {
		item.TbPhoto.Caption = fmt.Sprintf("%s", item.Title)
	}
	if len(item.FileIDs) > 0 {
		if len(item.TbPhoto.Caption) > 0 {
			item.TbPhoto.Caption += " "
		}
		item.TbPhoto.Caption += fmt.Sprintf("(%d Files)", len(item.FileIDs))
	}
	if item.Price > 0 {
		item.TbPhoto.Caption += fmt.Sprintf("\n\n💸 Price: %d sat", item.Price)
	}
	var msg *tb.Message
	if shopView.Message != nil {
		if item.TbPhoto != nil {
			if shopView.Message.Photo != nil {
				// can only edit photo messages with another photo
				msg = bot.tryEditMessage(shopView.Message, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
			} else {
				// if editing failes
				bot.tryDeleteMessage(shopView.Message)
				msg = bot.trySendMessage(shopView.Message.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
			}
		} else if item.Title != "" {
			msg = bot.tryEditMessage(shopView.Message, item.Title, bot.shopMenu(ctx, shop, &item))
			if msg == nil {
				msg = bot.trySendMessage(shopView.Message.Chat, item.Title, bot.shopMenu(ctx, shop, &item))
			}
		}
	} else {
		if m != nil && m.Chat != nil {
			msg = bot.trySendMessage(m.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
		} else {
			msg = bot.trySendMessage(user.Telegram, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
		}
		shopView.Page = 0
	}
	shopView.Message = msg
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return msg
}

// shopHandler is invoked when the user enters /shop
func (bot *TipBot) shopHandler(ctx context.Context, m *tb.Message) {
	if !m.Private() {
		return
	}
	user := LoadUser(ctx)
	shopOwner := user

	// when no argument is given, i.e. command is only /shop, load /shops
	shop := &Shop{}
	if len(strings.Split(m.Text, " ")) < 2 {
		bot.shopsHandler(ctx, m)
		return
	} else {
		// else: get shop by shop ID
		shopID := strings.Split(m.Text, " ")[1]
		var err error
		shop, err = bot.getShop(ctx, shopID)
		if err != nil {
			log.Errorf("[shopHandler] %s", err)
			return
		}
	}
	shopOwner = shop.Owner
	shopView := ShopView{
		ID:        fmt.Sprintf("shopview-%d", user.Telegram.ID),
		ShopID:    shop.ID,
		Page:      0,
		ShopOwner: shopOwner,
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	shopMessage := &tb.Message{Chat: m.Chat}
	if len(shop.ItemIds) > 0 {
		// item := shop.Items[shop.ItemIds[shopView.Page]]
		// shopMessage = bot.trySendMessage(m.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
		shopMessage = bot.displayShopItem(ctx, m, shop)
	} else {
		shopMessage = bot.trySendMessage(m.Chat, "No items in shop.", bot.shopMenu(ctx, shop, &ShopItem{}))
	}
	shopView.Message = shopMessage
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return
}

// shopNewItemHandler is invoked when the user presses the new item button
func (bot *TipBot) shopNewItemHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shop, err := bot.getShop(ctx, c.Data)
	if err != nil {
		log.Errorf("[shopNewItemHandler] %s", err)
		return
	}
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return
	}
	if len(shop.Items) >= shop.MaxItems {
		bot.trySendMessage(c.Sender, fmt.Sprintf("🚫 You can only have %d items in this shop. Delete an item to add a new one.", shop.MaxItems))
		return
	}

	// We need to save the pay state in the user state so we can load the payment in the next handler
	paramsJson, err := json.Marshal(shop)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandler] Error: %s", err.Error())
		// bot.trySendMessage(m.Sender, err.Error())
		return
	}
	SetUserState(user, bot, lnbits.UserStateShopItemSendPhoto, string(paramsJson))
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("🌄 Send me an image."))
}

// addShopItem is a helper function for creating a shop item in the database
func (bot *TipBot) addShopItem(ctx context.Context, shopId string) (*Shop, ShopItem, error) {
	shop, err := bot.getShop(ctx, shopId)
	if err != nil {
		log.Errorf("[addShopItem] %s", err)
		return shop, ShopItem{}, err
	}
	user := LoadUser(ctx)
	// onnly the correct user can press
	if shop.Owner.Telegram.ID != user.Telegram.ID {
		return shop, ShopItem{}, fmt.Errorf("not owner")
	}
	err = shop.Lock(shop, bot.Bunt)
	defer shop.Release(shop, bot.Bunt)

	itemId := fmt.Sprintf("item-%s-%s", shop.ID, RandStringRunes(8))
	item := ShopItem{
		ID:           itemId,
		ShopID:       shop.ID,
		Owner:        user,
		Type:         "photo",
		LanguageCode: shop.LanguageCode,
		MaxFiles:     MAX_FILES_PER_ITEM,
	}
	shop.Items[itemId] = item
	shop.ItemIds = append(shop.ItemIds, itemId)
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	return shop, shop.Items[itemId], nil
}

// addShopItemPhoto is invoked when the users sends a photo as a new item
func (bot *TipBot) addShopItemPhoto(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}

	// read item from user.StateData
	var state_shop Shop
	err := json.Unmarshal([]byte(user.StateData), &state_shop)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		bot.trySendMessage(m.Sender, Translate(ctx, "errorTryLaterMessage"), Translate(ctx, "errorTryLaterMessage"))
		return
	}
	if state_shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}
	// todo: can go away

	// shop, err := bot.getShop(ctx, state_shop.ID)
	// if err != nil {
	// 	log.Errorf("[addShopItemPhoto] %s", err)
	// }
	// if shop.Owner.Telegram.ID != m.Sender.ID {
	// 	return
	// }
	// immediatelly set lock to block duplicate calls

	shop, item, err := bot.addShopItem(ctx, state_shop.ID)
	// err = shop.Lock(shop, bot.Bunt)
	// defer shop.Release(shop, bot.Bunt)
	item.TbPhoto = m.Photo
	item.Title = m.Caption
	shop.Items[item.ID] = item
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	bot.tryDeleteMessage(m)

	bot.sendStatusMessage(ctx, m.Sender, fmt.Sprintf("✅ Image added."))
	ResetUserState(user, bot)
	time.Sleep(time.Duration(2) * time.Second)
	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	shopView, err := bot.getUserShopview(ctx, user)
	shopView.Page = len(shop.Items) - 1
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	bot.displayShopItem(ctx, shopView.Message, shop)
}

// ------------------- item files ----------
// shopItemAddItemHandler is invoked when the user presses the new item button
func (bot *TipBot) shopItemAddItemHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[addItemFileHandler] %s", err.Error())
		return
	}

	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		log.Errorf("[shopNewItemHandler] %s", err)
		return
	}

	itemID := user.StateData

	item := shop.Items[itemID]

	if len(item.FileIDs) >= item.MaxFiles {
		bot.trySendMessage(c.Sender, fmt.Sprintf("🚫 You can only have %d files in this item.", item.MaxFiles))
		return
	}
	SetUserState(user, bot, lnbits.UserStateShopItemSendItemFile, c.Data)
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("💾 Send me a file."))
}

// addItemFileHandler is invoked when the users sends a new file for the item
func (bot *TipBot) addItemFileHandler(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[addItemFileHandler] %s", err.Error())
		return
	}

	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		log.Errorf("[shopNewItemHandler] %s", err)
		return
	}

	itemID := user.StateData

	item := shop.Items[itemID]
	if m.Photo != nil {
		item.FileIDs = append(item.FileIDs, m.Photo.FileID)
		item.FileTypes = append(item.FileTypes, "photo")
	} else if m.Document != nil {
		item.FileIDs = append(item.FileIDs, m.Document.FileID)
		item.FileTypes = append(item.FileTypes, "document")
	} else if m.Audio != nil {
		item.FileIDs = append(item.FileIDs, m.Audio.FileID)
		item.FileTypes = append(item.FileTypes, "audio")
	} else if m.Video != nil {
		item.FileIDs = append(item.FileIDs, m.Video.FileID)
		item.FileTypes = append(item.FileTypes, "video")
	} else if m.Voice != nil {
		item.FileIDs = append(item.FileIDs, m.Voice.FileID)
		item.FileTypes = append(item.FileTypes, "voice")
	} else if m.VideoNote != nil {
		item.FileIDs = append(item.FileIDs, m.VideoNote.FileID)
		item.FileTypes = append(item.FileTypes, "videonote")
	} else if m.Sticker != nil {
		item.FileIDs = append(item.FileIDs, m.Sticker.FileID)
		item.FileTypes = append(item.FileTypes, "sticker")
	} else {
		log.Errorf("[addItemFileHandler] no file found")
		return
	}
	shop.Items[item.ID] = item

	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	bot.tryDeleteMessage(m)
	bot.sendStatusMessage(ctx, m.Sender, fmt.Sprintf("✅ File added."))
	time.Sleep(time.Duration(2) * time.Second)
	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	bot.displayShopItem(ctx, shopView.Message, shop)

}

func (bot *TipBot) shopGetItemFilesHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[addItemFileHandler] %s", err.Error())
		return
	}

	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		log.Errorf("[shopNewItemHandler] %s", err)
		return
	}

	itemID := c.Data

	item := shop.Items[itemID]
	for i, fileID := range item.FileIDs {
		bot.sendFileByID(ctx, c.Sender, fileID, item.FileTypes[i])
	}
	// fileId := item.FileIDs[len(item.FileIDs)-1]
	// fileType := item.FileTypes[len(item.FileTypes)-1]
	// bot.sendFileByID(ctx, c.Sender, fileId, fileType)
}

func (bot *TipBot) sendFileByID(ctx context.Context, to tb.Recipient, fileId string, fileType string) {
	switch fileType {
	case "photo":
		sendable := &tb.Photo{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "document":
		sendable := &tb.Document{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "audio":
		sendable := &tb.Audio{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "video":
		sendable := &tb.Video{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "voice":
		sendable := &tb.Voice{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "videonote":
		sendable := &tb.VideoNote{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "sticker":
		sendable := &tb.Sticker{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	}
	return
}

// -------------- shops handler --------------
var ShopsText = "*Welcome to %s shop.*\nThere are %d shops here.\n%s\n\nPress buttons to add a new shop."

// shopsHandlerCallback is a warpper for shopsHandler for callbacks
func (bot *TipBot) shopsHandlerCallback(ctx context.Context, c *tb.Callback) {
	bot.shopsHandler(ctx, c.Message)
}

// shopsHandler is invoked when the user enters /shops
func (bot *TipBot) shopsHandler(ctx context.Context, m *tb.Message) {
	if !m.Private() {
		return
	}
	user := LoadUser(ctx)
	shopOwner := user

	// if the user in the command, i.e. /shops @user
	if len(strings.Split(m.Text, " ")) > 1 && strings.Split(m.Text, " ")[0] == "/shops" {
		toUserStrMention := ""
		toUserStrWithoutAt := ""

		// check for user in command, accepts user mention or plain username without @
		if len(m.Entities) > 1 && m.Entities[1].Type == "mention" {
			toUserStrMention = m.Text[m.Entities[1].Offset : m.Entities[1].Offset+m.Entities[1].Length]
			toUserStrWithoutAt = strings.TrimPrefix(toUserStrMention, "@")
		} else {
			var err error
			toUserStrWithoutAt, err = getArgumentFromCommand(m.Text, 1)
			if err != nil {
				log.Errorln(err.Error())
				return
			}
			toUserStrWithoutAt = strings.TrimPrefix(toUserStrWithoutAt, "@")
			toUserStrMention = "@" + toUserStrWithoutAt
		}

		toUserDb, err := GetUserByTelegramUsername(toUserStrWithoutAt, *bot)
		if err != nil {
			NewMessage(m, WithDuration(0, bot))
			// cut username if it's too long
			if len(toUserStrMention) > 100 {
				toUserStrMention = toUserStrMention[:100]
			}
			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "sendUserHasNoWalletMessage"), str.MarkdownEscape(toUserStrMention)))
			return
		}
		// overwrite user with the one from db
		shopOwner = toUserDb
	} else if strings.Split(m.Text, " ")[0] != "/shops" {
		// otherwise, the user is returning to a shops view from a back button callback
		shopView, err := bot.getUserShopview(ctx, user)
		if err == nil {
			shopOwner = shopView.ShopOwner
		}
	}

	if shopOwner == nil {
		log.Error("[shopsHandler] shopOwner is nil")
		return
	}
	shops, err := bot.getUserShops(ctx, shopOwner)
	if err != nil && user == shopOwner {
		shops, err = bot.initUserShops(ctx, user)
		if err != nil {
			log.Errorf("[shopsHandler] %s", err)
			return
		}
	}

	// build shop list
	shopTitles := ""
	for _, shopId := range shops.Shops {
		shop, err := bot.getShop(ctx, shopId)
		if err != nil {
			log.Errorf("[shopsHandler] %s", err)
			return
		}
		shopTitles += fmt.Sprintf("\n%s (%d items)", shop.Title, len(shop.Items))

	}

	// shows "your shop" or "@other's shop"
	shopOwnerText := "your"
	if shopOwner != user {
		shopOwnerText = fmt.Sprintf("@%s's", shopOwner.Telegram.Username)
	}
	// if the user used the command /shops, we will send a new message
	// if the user clicked a button and has a shopview set, we will edit an old message
	shopView, err := bot.getUserShopview(ctx, user)
	var shopsMsg *tb.Message
	if err == nil && strings.Split(m.Text, " ")[0] != "/shops" {
		// the user is returning to a shops view from a back button callback
		if shopView.Message.Photo == nil {
			shopsMsg = bot.tryEditMessage(shopView.Message, fmt.Sprintf(ShopsText, shopOwnerText, len(shops.Shops), shopTitles), bot.shopsMainMenu(ctx, shops))
		}
		if shopsMsg == nil {
			// if editing has failed, we will send a new message
			bot.tryDeleteMessage(shopView.Message)
			shopsMsg = bot.trySendMessage(m.Chat, fmt.Sprintf(ShopsText, shopOwnerText, len(shops.Shops), shopTitles), bot.shopsMainMenu(ctx, shops))

		}
	} else {
		// the user has entered /shops or
		// the user has no shopview set, so we will send a new message
		if shopView.Message != nil {
			// delete any old shop message
			bot.tryDeleteMessage(shopView.Message)
		}
		shopsMsg = bot.trySendMessage(m.Chat, fmt.Sprintf(ShopsText, shopOwnerText, len(shops.Shops), shopTitles), bot.shopsMainMenu(ctx, shops))
	}
	shopView = ShopView{
		ID:        fmt.Sprintf("shopview-%d", user.Telegram.ID),
		Message:   shopsMsg,
		ShopOwner: shopOwner,
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return
}

// shopsDeleteShopBrowser is invoked when the user clicks on "delete shops" and makes a list of all shops
func (bot *TipBot) shopsDeleteShopBrowser(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return
	}
	var s []*Shop
	for _, shopId := range shops.Shops {
		shop, _ := bot.getShop(ctx, shopId)
		if shop.Owner.Telegram.ID != c.Sender.ID {
			return
		}
		s = append(s, shop)
	}
	shopShopsButton := shopKeyboard.Data("⬅️ Back", "shops_shops", shops.ID)
	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "delete_shop"), shopShopsButton), shopKeyboard, 1)...)
	bot.tryEditMessage(c.Message, "Which shop do you want to delete?", shopKeyboard)
}

// shopsLinkShopBrowser is invoked when the user clicks on "shop links" and makes a list of all shops
func (bot *TipBot) shopsLinkShopBrowser(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return
	}
	var s []*Shop
	for _, shopId := range shops.Shops {
		shop, _ := bot.getShop(ctx, shopId)
		if shop.Owner.Telegram.ID != c.Sender.ID {
			return
		}
		s = append(s, shop)
	}
	shopShopsButton := shopKeyboard.Data("⬅️ Back", "shops_shops", shops.ID)
	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "link_shop"), shopShopsButton), shopKeyboard, 1)...)
	bot.tryEditMessage(c.Message, "Select the shop you want to get the link of.", shopKeyboard)
}

// shopSelectLink is invoked when the user has chosen a shop to get the link of
func (bot *TipBot) shopSelectLink(ctx context.Context, c *tb.Callback) {
	shop, _ := bot.getShop(ctx, c.Data)
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return
	}
	bot.trySendMessage(c.Sender, fmt.Sprintf("*%s*: `/shop %s`", shop.Title, shop.ID))
}

// shopSelect is invoked when the user has selected a shop to browse
func (bot *TipBot) shopSelect(ctx context.Context, c *tb.Callback) {
	shop, _ := bot.getShop(ctx, c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		shopView = ShopView{
			ID:     fmt.Sprintf("shopview-%d", c.Sender.ID),
			ShopID: shop.ID,
			Page:   0,
		}
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	}
	shopView.Page = 0
	shopView.ShopID = shop.ID

	var shopMessage *tb.Message
	if len(shop.ItemIds) > 0 {
		bot.tryDeleteMessage(c.Message)
		item := shop.Items[shop.ItemIds[shopView.Page]]
		shopMessage = bot.trySendMessage(c.Message.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
	} else {
		shopMessage = bot.tryEditMessage(c.Message, "There are no items in this shop yet.", bot.shopMenu(ctx, shop, &ShopItem{}))
	}
	shopView.Message = shopMessage
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
}

// shopSelectDelete is invoked when the user has chosen a shop to delete
func (bot *TipBot) shopSelectDelete(ctx context.Context, c *tb.Callback) {
	shop, _ := bot.getShop(ctx, c.Data)
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return
	}
	// first, delete from Shops
	for i, shopId := range shops.Shops {
		if shopId == shop.ID {
			if i == len(shops.Shops)-1 {
				shops.Shops = shops.Shops[:i]
			} else {
				shops.Shops = append(shops.Shops[:i], shops.Shops[i+1:]...)
			}
			break
		}
	}
	runtime.IgnoreError(shops.Set(shops, bot.Bunt))

	// then, delete shop
	runtime.IgnoreError(shop.Delete(shop, bot.Bunt))

	// then update buttons
	bot.shopsDeleteShopBrowser(ctx, c)
}

// shopsBrowser makes a button list of all shops the user can browse
func (bot *TipBot) shopsBrowser(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shops, err := bot.getUserShops(ctx, shopView.ShopOwner)
	if err != nil {
		return
	}
	var s []*Shop
	for _, shopId := range shops.Shops {
		shop, _ := bot.getShop(ctx, shopId)
		s = append(s, shop)
	}
	shopShopsButton := shopKeyboard.Data("⬅️ Back", "shops_shops", shops.ID)
	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "select_shop"), shopShopsButton), shopKeyboard, 1)...)
	shopMessage := bot.tryEditMessage(c.Message, "Select a shop you want to browse.", shopKeyboard)
	shopView, err = bot.getUserShopview(ctx, user)
	if err != nil {
		shopView.Message = shopMessage
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	}

}

// shopItemSettingsHandler is invoked when the user presses the shop settings button
func (bot *TipBot) shopSettingsHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return
	}
	if shops.ID != c.Data || shops.Owner.Telegram.ID != user.Telegram.ID {
		log.Error("[shopSettingsHandler] item id mismatch")
		return
	}
	bot.tryEditMessage(shopView.Message, shopView.Message.Text, bot.shopsSettingsMenu(ctx, shops))
}

// shopNewShopHandler is invoked when the user presses the new shop button
func (bot *TipBot) shopNewShopHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		log.Errorf("[shopNewShopHandler] %s", err)
		return
	}
	if len(shops.Shops) >= shops.MaxShops {
		bot.trySendMessage(c.Sender, fmt.Sprintf("🚫 You can only have %d shops. Delete a shop to create a new one.", shops.MaxShops))
		return
	}
	shop, err := bot.addUserShop(ctx, user)
	// We need to save the pay state in the user state so we can load the payment in the next handler
	SetUserState(user, bot, lnbits.UserEnterShopTitle, shop.ID)
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("⌨️ Enter the name of your shop."), tb.ForceReply)
}

// enterShopTitleHandler is invoked when the user enters the shop title
func (bot *TipBot) enterShopTitleHandler(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	// read item from user.StateData
	shop, err := bot.getShop(ctx, user.StateData)
	if err != nil {
		return
	}
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}
	// crop shop title
	if len(m.Text) > SHOP_TITLE_MAX_LENGTH {
		m.Text = m.Text[:SHOP_TITLE_MAX_LENGTH]
	}
	shop.Title = m.Text
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	bot.sendStatusMessage(ctx, m.Sender, fmt.Sprintf("✅ Shop added."))
	ResetUserState(user, bot)
	time.Sleep(time.Duration(2) * time.Second)
	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	bot.shopsHandler(ctx, m)
	bot.tryDeleteMessage(m)
}
