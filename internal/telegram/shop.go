package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"
	"github.com/eko/gocache/store"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

type ShopView struct {
	ID            string
	ShopID        string
	Page          int
	Message       *tb.Message
	StatusMessage *tb.Message
}

type ShopItem struct {
	ID           string       `json:"ID"`          // ID of the tx object in bunt db
	ShopID       string       `json:"shopID"`      // ID of the shop
	Owner        *lnbits.User `json:"owner"`       // Owner of the item
	Type         string       `json:"Type"`        // Type of the tx object in bunt db
	FileID       string       `json:"fileID"`      // Telegram fileID of the item
	Title        string       `json:"title"`       // Title of the item
	Description  string       `json:"description"` // Description of the item
	Price        int          `json:"price"`       // price of the item
	NSold        int          `json:"nSold"`       // number of times item was sold
	TbPhoto      *tb.Photo    `json:"tbPhoto"`     // Telegram photo object
	LanguageCode string       `json:"languagecode"`
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
}

type Shops struct {
	*transaction.Base
	ID    string       `json:"ID"`    // holds the ID of the tx object in bunt db
	Owner *lnbits.User `json:"owner"` // owner of the shop
	Shops []string     `json:"shop"`  //
}

func (shop *Shop) getItem(itemId string) (item ShopItem, ok bool) {
	item, ok = shop.Items[itemId]
	return
}

var (
	shopKeyboard          = &tb.ReplyMarkup{ResizeReplyKeyboard: false}
	browseShopButton      = shopKeyboard.Data("Browse shops", "shops_browse")
	shopNewShopButton     = shopKeyboard.Data("New Shop", "shops_newshop")
	shopDeleteShopButton  = shopKeyboard.Data("Delete Shop", "shops_deleteshop")
	shopSettingsButton    = shopKeyboard.Data("Settings", "shops_settings")
	shopBrowseItemsButton = shopKeyboard.Data("Browse shops", "shop_browse")

	shopAddItemButton  = shopKeyboard.Data("Add item", "shop_additem")
	shopNextitemButton = shopKeyboard.Data(">", "shop_nextitem")
	shopPrevitemButton = shopKeyboard.Data("<", "shop_previtem")
	shopBuyitemButton  = shopKeyboard.Data("Buy", "shop_buyitem")

	shopSelectButton           = shopKeyboard.Data("SHOP SELECTOR", "select_shop")        // shop slectino buttons
	shopDeleteSelectButton     = shopKeyboard.Data("DELETE SHOP SELECTOR", "delete_shop") // shop slectino buttons
	shopItemPriceButton        = shopKeyboard.Data("Price", "shop_itemprice")
	shopItemDeleteButton       = shopKeyboard.Data("Delete", "shop_itemdelete")
	shopItemTitleButton        = shopKeyboard.Data("Set title", "shop_itemtitle")
	shopItemSettingsButton     = shopKeyboard.Data("Item settings", "shop_itemsettings")
	shopItemSettingsBackButton = shopKeyboard.Data("Back", "shop_itemsettingsback")
)

func (bot TipBot) shopsMainMenu(ctx context.Context, shops *Shops) *tb.ReplyMarkup {
	browseShopButton := shopKeyboard.Data("Browse shops", "shops_browse", shops.ID)
	shopNewShopButton := shopKeyboard.Data("New Shop", "shops_newshop", shops.ID)
	shopSettingsButton := shopKeyboard.Data("Settings", "shops_settings", shops.ID)
	user := LoadUser(ctx)

	buttons := []tb.Row{}
	if len(shops.Shops) > 0 {
		buttons = append(buttons, shopKeyboard.Row(browseShopButton))
	}
	if user.Telegram.ID == shops.Owner.Telegram.ID {
		buttons = append(buttons, shopKeyboard.Row(shopNewShopButton, shopSettingsButton))
	}
	shopKeyboard.Inline(
		buttons...,
	)
	return shopKeyboard
}

func (bot TipBot) shopsSettingsMenu(ctx context.Context, shops *Shops) *tb.ReplyMarkup {
	browseShopButton := shopKeyboard.Data("Browse shops", "shops_browse", shops.ID)
	shopDeleteShopButton := shopKeyboard.Data("Delete Shop", "shops_deleteshop", shops.ID)
	buttons := []tb.Row{shopKeyboard.Row(browseShopButton), shopKeyboard.Row(shopDeleteShopButton)}
	shopKeyboard.Inline(
		buttons...,
	)
	return shopKeyboard
}

// shopItemSettingsHandler is invoked when the user presses the item settings button
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
		log.Error("[shopItemSettingsHandler] item id mismatch")
	}
	bot.tryEditMessage(shopView.Message, shopView.Message.Text, bot.shopsSettingsMenu(ctx, shops))
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

	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "delete_shop"), shopBrowseItemsButton), shopKeyboard, 1)...)
	bot.tryEditMessage(c.Message, "Which shop do you want to delete?", shopKeyboard)

}

// shopItemSettingsMenu builds the buttons of the item settings
func (bot TipBot) shopItemSettingsMenu(ctx context.Context, shop *Shop, item *ShopItem) *tb.ReplyMarkup {
	shopItemPriceButton = shopKeyboard.Data("Price", "shop_itemprice", item.ID)
	shopItemDeleteButton = shopKeyboard.Data("Delete", "shop_itemdelete", item.ID)
	shopItemTitleButton = shopKeyboard.Data("Set title", "shop_itemtitle", item.ID)
	shopItemSettingsBackButton = shopKeyboard.Data("Back", "shop_itemsettingsback", item.ID)
	user := LoadUser(ctx)
	buttons := []tb.Row{}
	if user.Telegram.ID == shop.Owner.Telegram.ID {
		buttons = append(buttons, shopKeyboard.Row(shopItemDeleteButton, shopItemPriceButton))
		buttons = append(buttons, shopKeyboard.Row(shopItemTitleButton, shopItemSettingsBackButton))
	}
	shopKeyboard.Inline(
		buttons...,
	)
	return shopKeyboard
}

// shopMenu builds the buttons in the item browser
func (bot TipBot) shopMenu(ctx context.Context, shop *Shop, item *ShopItem) *tb.ReplyMarkup {
	shopBrowseItemsButton = shopKeyboard.Data("Browse shops", "shop_browse", shop.ID)
	shopAddItemButton = shopKeyboard.Data("Add item", "shop_additem", shop.ID)
	shopItemSettingsButton = shopKeyboard.Data("Item settings", "shop_itemsettings", item.ID)
	shopNextitemButton = shopKeyboard.Data(">", "shop_nextitem", shop.ID)
	shopPrevitemButton = shopKeyboard.Data("<", "shop_previtem", shop.ID)
	buyButtonText := "Get"
	if item.Price > 0 {
		buyButtonText = fmt.Sprintf("Buy (%d sat)", item.Price)
	}
	shopBuyitemButton = shopKeyboard.Data(buyButtonText, "shop_buyitem", item.ID)
	user := LoadUser(ctx)
	buttons := []tb.Row{}
	if user.Telegram.ID == shop.Owner.Telegram.ID {
		if len(shop.Items) == 0 {
			buttons = append(buttons, shopKeyboard.Row(shopAddItemButton))
		} else {
			buttons = append(buttons, shopKeyboard.Row(shopAddItemButton, shopItemSettingsButton))
		}
	}
	// publicButtons := []tb.Row{}
	if len(shop.Items) > 0 {
		buttons = append(buttons, shopKeyboard.Row(shopPrevitemButton, shopBuyitemButton, shopNextitemButton))
		buttons = append(buttons, shopKeyboard.Row(shopBrowseItemsButton))
	}
	shopKeyboard.Inline(
		buttons...,
	)
	return shopKeyboard
}

// getUserShopview returns ShopView object from cache that holds information about the user's current browsing view
func (bot *TipBot) getUserShopview(ctx context.Context, user *lnbits.User) (shopView ShopView, err error) {
	sv, err := bot.Cache.Get(fmt.Sprintf("shopview-%d", user.Telegram.ID))
	if err != nil {
		return
	}
	shopView = sv.(ShopView)
	return
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
		shopMessage = bot.tryEditMessage(c.Message, "No items in shop.", bot.shopMenu(ctx, shop, &ShopItem{}))
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
			shops.Shops = append(shops.Shops[:i], shops.Shops[i+1:]...)
			break
		}
	}
	runtime.IgnoreError(shops.Set(shops, bot.Bunt))

	// then, delete shop
	runtime.IgnoreError(shop.Delete(shop, bot.Bunt))
}

// makseShopSelectionButtons produces a list of all buttons with a uniqueString ID
func (bot *TipBot) makseShopSelectionButtons(shops []*Shop, uniqueString string) []tb.Btn {
	var buttons []tb.Btn
	for _, shop := range shops {
		buttons = append(buttons, shopKeyboard.Data(shop.Title, uniqueString, shop.ID))
	}
	return buttons
}

// shopsBrowser makes a button list of all shops the user can browse
func (bot *TipBot) shopsBrowser(ctx context.Context, c *tb.Callback) {
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
	browseShopButton := shopKeyboard.Data("Browse shops", "shops_browse", shops.ID)
	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "select_shop"), browseShopButton), shopKeyboard, 3)...)
	bot.tryEditMessage(c.Message, "Select your shop!", shopKeyboard)

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
	}
	bot.tryEditMessage(shopView.Message, item.TbPhoto, bot.shopItemSettingsMenu(ctx, shop, &item))
}

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
	}
	bot.displayShopItem(ctx, c.Message, shop)
}

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

func (bot *TipBot) shopPrevItemButtonHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	if shopView.Page == 0 {
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

var ShopsText = "*Welcome to your shop.*\nYour have %d shops.\n%s\n🔞 Look at me `(8 items for 100 sat each)`\n📚 Audiobooks `(12 items for 1000 sat each)`\n\nPress buttons to add a new shop."

func (bot *TipBot) shopsHandlerCallback(ctx context.Context, c *tb.Callback) {
	bot.shopsHandler(ctx, c.Message)
}
func (bot *TipBot) shopsHandler(ctx context.Context, m *tb.Message) {
	if !m.Private() {
		return
	}
	user := LoadUser(ctx)

	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		shops, err = bot.initUserShops(ctx, user)
		if err != nil {
			log.Errorf("[shopsHandler] %s", err)
			return
		}
		bot.trySendMessage(m.Chat, "You have no shops.", bot.shopsMainMenu(ctx, shops))
		return
	}

	shopTitles := ""
	for _, shopId := range shops.Shops {
		shop, err := bot.getShop(ctx, shopId)
		if err != nil {
			log.Errorf("[shopsHandler] %s", err)
			return
		}
		shopTitles += fmt.Sprintf("\n%s", shop.Title)

	}

	// if the user used the command /shops, we will send a new message
	// if the user clied a button and has a shopview set, we will edit an old message
	shopView, err := bot.getUserShopview(ctx, user)
	if err == nil {
		if shopView.Message.ID == m.ID {
			bot.tryDeleteMessage(shopView.Message)
		}
	}
	shopsMsg := bot.trySendMessage(m.Chat, fmt.Sprintf(ShopsText, len(shops.Shops), shopTitles), bot.shopsMainMenu(ctx, shops))
	shopView = ShopView{
		ID: fmt.Sprintf("shopview-%d", m.Sender.ID),
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})

	shopView.Message = shopsMsg
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	// runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	return
}

func (bot *TipBot) displayShopItem(ctx context.Context, m *tb.Message, shop *Shop) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// item.TbPhoto.Caption = fmt.Sprintf("%s", item.Title)
	bot.tryEditMessage(m, item.TbPhoto, bot.shopMenu(ctx, shop, &item))

}

func (bot *TipBot) shopHandler(ctx context.Context, m *tb.Message) {
	if !m.Private() {
		return
	}
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		log.Errorf("[shopHandler] %s", err)
		return
	}

	shop, err := bot.getShop(ctx, shops.Shops[len(shops.Shops)-1])

	if shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}

	shopView := ShopView{
		ID:     fmt.Sprintf("shopview-%d", user.Telegram.ID),
		ShopID: shop.ID,
		Page:   0,
	}

	var shopMessage *tb.Message
	if len(shop.ItemIds) > 0 {
		item := shop.Items[shop.ItemIds[shopView.Page]]
		shopMessage = bot.trySendMessage(m.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
	} else {
		shopMessage = bot.trySendMessage(m.Chat, "No items in shop.", bot.shopMenu(ctx, shop, &ShopItem{}))
	}
	shopView.Message = shopMessage
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return
}

func (bot *TipBot) shopNewShopHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	_, err := bot.getUserShops(ctx, user)
	if err != nil {
		// _, err = bot.initUserShops(ctx, user)
		// if err != nil {
		// 	log.Errorf("[shopNewShopHandler] %s", err)
		// 	return
		// }
		log.Errorf("[shopNewShopHandler] %s", err)
		return
	}
	shop, err := bot.addUserShop(ctx, user)
	// We need to save the pay state in the user state so we can load the payment in the next handler
	SetUserState(user, bot, lnbits.UserEnterShopTitle, shop.ID)
	bot.trySendMessage(c.Sender, fmt.Sprintf("Enter the name of your shop"), tb.ForceReply)
}

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
	shop.Title = m.Text
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
}

func (bot *TipBot) shopNewItemHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shop, err := bot.getShop(ctx, c.Data)
	if err != nil {
		log.Errorf("[shopNewItemHandler] %s", err)
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
	bot.trySendMessage(c.Sender, fmt.Sprintf("🌄 Send me an image ."))
}

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
		Title:        "Item tiiitle",
		ShopID:       shop.ID,
		Owner:        user,
		Type:         "photo",
		LanguageCode: shop.LanguageCode,
		Price:        10,
	}
	shop.Items[itemId] = item
	shop.ItemIds = append(shop.ItemIds, itemId)
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	return shop, shop.Items[itemId], nil
}

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
	item.FileID = m.Photo.FileID
	item.TbPhoto = m.Photo
	shop.Items[item.ID] = item
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))

	bot.tryDeleteMessage(m)
	statusMsg := bot.trySendMessage(m.Sender, fmt.Sprintf("✅ Image added."))
	NewMessage(statusMsg, WithDuration(5, bot))
	// shopView, err := bot.getUserShopview(ctx, user)
	// if err != nil {
	// 	return
	// }

	// time.Sleep(time.Duration(1) * time.Second)
	// // bot.shopNextItemButtonHandler(ctx, &tb.Callback{Message: shopView.Message})
	// if shopView.Page < len(shop.Items)-1 {
	// 	shopView.Page = len(shop.Items) - 1
	// 	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	// 	bot.displayShopItem(ctx, shopView.Message, shop)
	// }
}

func (bot *TipBot) initUserShops(ctx context.Context, user *lnbits.User) (*Shops, error) {
	id := fmt.Sprintf("shops-%d", user.Telegram.ID)
	shops := &Shops{
		Base:  transaction.New(transaction.ID(id)),
		ID:    id,
		Owner: user,
		Shops: []string{},
	}
	runtime.IgnoreError(shops.Set(shops, bot.Bunt))
	return shops, nil
}
func (bot *TipBot) getUserShops(ctx context.Context, user *lnbits.User) (*Shops, error) {
	tx := &Shops{Base: transaction.New(transaction.ID(fmt.Sprintf("shops-%d", user.Telegram.ID)))}
	sn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[getUserShops] User: %s (%d): %s", GetUserStr(user.Telegram), user.Telegram.ID, err)
		return &Shops{}, err
	}
	shops := sn.(*Shops)
	return shops, nil
}
func (bot *TipBot) addUserShop(ctx context.Context, user *lnbits.User) (*Shop, error) {
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return &Shop{}, err
	}
	shopId := fmt.Sprintf("shop-%d-%s", user.Telegram.ID, RandStringRunes(5))
	shop := &Shop{
		Base:         transaction.New(transaction.ID(shopId)),
		ID:           shopId,
		Title:        fmt.Sprintf("Shop %d (%s)", len(shops.Shops)+1, shopId),
		Owner:        user,
		Type:         "photo",
		Items:        make(map[string]ShopItem),
		LanguageCode: ctx.Value("publicLanguageCode").(string),
	}
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	shops.Shops = append(shops.Shops, shopId)
	runtime.IgnoreError(shops.Set(shops, bot.Bunt))
	return shop, nil
}
func (bot *TipBot) getShop(ctx context.Context, shopId string) (*Shop, error) {
	tx := &Shop{Base: transaction.New(transaction.ID(shopId))}
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[getShop] %s", err)
		return &Shop{}, err
	}
	shop := sn.(*Shop)
	if shop.Owner == nil {
		return &Shop{}, fmt.Errorf("shop has no owner")
	}
	return shop, nil
}
