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
	shopMainMenu          = &tb.ReplyMarkup{ResizeReplyKeyboard: false}
	browseShopButton      = shopMainMenu.Data("Browse shops", "shops_browse")
	shopNewShopButton     = shopMainMenu.Data("New Shop", "shops_newshop")
	shopSettingsButton    = shopMainMenu.Data("Settings", "shops_settings")
	shopBrowseItemsButton = shopMainMenu.Data("Browse items", "shop_browse")
	shopAddItemButton     = shopMainMenu.Data("Add item", "shop_additem")
	shopNextitemButton    = getNextItemButton("")
	shopPrevitemButton    = shopMainMenu.Data("<", "shop_previtem")
	shopBuyitemButton     = shopMainMenu.Data("Buy", "shop_buyitem")
	shopSelect            = shopMainMenu.Data("", "select_shop")
)

func (bot TipBot) shopsMainMenu(ctx context.Context, shops *Shops) *tb.ReplyMarkup {
	browseShopButton := shopMainMenu.Data("Browse shops", "shops_browse", shops.ID)
	shopNewShopButton := shopMainMenu.Data("New Shop", "shops_newshop", shops.ID)
	shopSettingsButton := shopMainMenu.Data("Settings", "shops_settings", shops.ID)

	shopMainMenu.Inline(
		shopMainMenu.Row(
			browseShopButton),
		shopMainMenu.Row(
			shopNewShopButton,
			shopSettingsButton),
	)
	return shopMainMenu
}

func (bot TipBot) shopMenu(ctx context.Context, shop *Shop) *tb.ReplyMarkup {
	shopBrowseItemsButton = shopMainMenu.Data("Browse items", "shop_browse", shop.ID)
	shopAddItemButton = shopMainMenu.Data("Add item", "shop_additem", shop.ID)
	shopNextitemButton = shopMainMenu.Data(">", "shop_nextitem", shop.ID)
	shopPrevitemButton = shopMainMenu.Data("<", "shop_previtem", shop.ID)
	shopBuyitemButton = shopMainMenu.Data("Tip", "shop_buyitem", shop.ID)

	shopMainMenu.Inline(
		shopMainMenu.Row(
			shopBrowseItemsButton,
			shopAddItemButton),
		shopMainMenu.Row(
			shopPrevitemButton,
			shopBuyitemButton,
			shopNextitemButton),
	)
	return shopMainMenu
}

func (bot *TipBot) getUserShopview(ctx context.Context, user *lnbits.User) (shopView ShopView, err error) {
	sv, err := bot.Cache.Get(fmt.Sprintf("shopview-%d", user.Telegram.ID))
	if err != nil {

		return
	}
	shopView = sv.(ShopView)
	return
}

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

	var shopMessage *tb.Message
	if len(shop.ItemIds) > 0 {
		bot.tryDeleteMessage(c.Message)
		shopMessage = bot.trySendMessage(c.Message.Chat, shop.Items[shop.ItemIds[shopView.Page]].TbPhoto, bot.shopMenu(ctx, shop))
	} else {
		shopMessage = bot.tryEditMessage(c.Message, "No items in shop.", bot.shopMenu(ctx, shop))
	}
	shopView.Message = shopMessage
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
}

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

	shopMainMenu.Inline(buttonWrapper(selectShopButtons(s), shopMainMenu, 3)...)
	bot.tryEditMessage(c.Message, "Select your shop!", shopMainMenu)

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

func (bot *TipBot) shopsHandler(ctx context.Context, m *tb.Message) {
	if !m.Private() {
		return
	}
	user := LoadUser(ctx)

	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		bot.trySendMessage(m.Chat, "You have no shops.", bot.shopsMainMenu(ctx, shops))
		return
	}

	shopTitles := ""
	for _, shopId := range shops.Shops {
		shop, err := bot.getShop(ctx, shopId)
		if err != nil {
			log.Errorf("[shopHandler] %s", err)
			return
		}
		shopTitles += fmt.Sprintf("\n%s", shop.Title)

	}

	// if the user used the command /shops, we will send a new message
	// if the user clied a button and has a shopview set, we will edit an old message
	shopView, err := bot.getUserShopview(ctx, user)
	if err == nil {
		if shopView.Message.ID != m.ID {
			bot.tryEditMessage(shopView.Message, fmt.Sprintf(ShopsText, len(shops.Shops), shopTitles), bot.shopsMainMenu(ctx, shops))
		}
	}
	bot.trySendMessage(m.Chat, fmt.Sprintf(ShopsText, len(shops.Shops), shopTitles), bot.shopsMainMenu(ctx, shops))
	// runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	return
}

func (bot *TipBot) displayShopItem(ctx context.Context, m *tb.Message, shop *Shop) {
	user := LoadUser(ctx)
	// shopView, err := bot.Cache.Get(fmt.Sprintf("shopview-%d", user.Telegram.ID))
	sv, err := bot.Cache.Get(fmt.Sprintf("shopview-%d", user.Telegram.ID))
	if err != nil {
		return
	}
	shopView := sv.(ShopView)
	bot.tryEditMessage(m, shop.Items[shop.ItemIds[shopView.Page]].TbPhoto, bot.shopMenu(ctx, shop))

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

	shop, err := bot.getShop(ctx, shops.Shops[0])

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
		shopMessage = bot.trySendMessage(m.Chat, shop.Items[shop.ItemIds[shopView.Page]].TbPhoto, bot.shopMenu(ctx, shop))
	} else {
		shopMessage = bot.trySendMessage(m.Chat, "No items in shop.", bot.shopMenu(ctx, shop))
	}
	shopView.Message = shopMessage
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return
}

func (bot *TipBot) shopNewShopHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	_, err := bot.getUserShops(ctx, user)
	if err != nil {
		_, err = bot.initUserShops(ctx, user)
		if err != nil {
			log.Errorf("[shopNewShopHandler] %s", err)
			return
		}
	}
	shop, err := bot.addUserShop(ctx, user)

	// We need to save the pay state in the user state so we can load the payment in the next handler
	paramsJson, err := json.Marshal(shop)
	if err != nil {
		log.Errorf("[shopNewShopHandler] Error: %s", err.Error())
		return
	}
	SetUserState(user, bot, lnbits.UserEnterShopTitle, string(paramsJson))
	bot.trySendMessage(c.Sender, fmt.Sprintf("Enter the name of your shop"), tb.ForceReply)
}

func (bot *TipBot) enterShopTitleHandler(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	// read item from user.StateData
	var shop Shop
	err := json.Unmarshal([]byte(user.StateData), &shop)
	if err != nil {
		log.Errorf("[enterShopTitleHandler] Error: %s", err.Error())
		return
	}
	shopb, err := bot.getShop(ctx, shop.ID)
	if shopb.Owner.Telegram.ID != m.Sender.ID {
		return
	}
	shopb.Title = m.Text
	runtime.IgnoreError(shopb.Set(shopb, bot.Bunt))
}

func (bot *TipBot) shopNewItemHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	item, err := bot.addShopItem(ctx, c.Data)
	if err != nil {
		log.Errorf("[shopNewItemHandler] %s", err)
		return
	}
	// We need to save the pay state in the user state so we can load the payment in the next handler
	paramsJson, err := json.Marshal(item)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandler] Error: %s", err.Error())
		// bot.trySendMessage(m.Sender, err.Error())
		return
	}
	SetUserState(user, bot, lnbits.UserStateShopItemSendPhoto, string(paramsJson))
	bot.trySendMessage(c.Sender, fmt.Sprintf("🌄 Send me an image ."))
}

func (bot *TipBot) addShopItem(ctx context.Context, shopId string) (ShopItem, error) {
	shop, err := bot.getShop(ctx, shopId)
	if err != nil {
		log.Errorf("[addShopItem] %s", err)
		return ShopItem{}, err
	}
	user := LoadUser(ctx)
	// onnly the correct user can press
	if shop.Owner.Telegram.ID != user.Telegram.ID {
		return ShopItem{}, fmt.Errorf("not owner")
	}
	// immediatelly set lock to block duplicate calls
	err = shop.Lock(shop, bot.Bunt)
	defer shop.Release(shop, bot.Bunt)

	itemId := fmt.Sprintf("item-%s-%s", shop.ID, RandStringRunes(8))
	item := ShopItem{
		ID:           itemId,
		ShopID:       shop.ID,
		Owner:        user,
		Type:         "photo",
		LanguageCode: shop.LanguageCode,
	}
	shop.Items[itemId] = item
	shop.ItemIds = append(shop.ItemIds, itemId)
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	return item, nil
}

func (bot *TipBot) addShopItemPhoto(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}

	// read item from user.StateData
	var item ShopItem
	err := json.Unmarshal([]byte(user.StateData), &item)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		bot.trySendMessage(m.Sender, Translate(ctx, "errorTryLaterMessage"), Translate(ctx, "errorTryLaterMessage"))
		return
	}
	shop, err := bot.getShop(ctx, item.ShopID)
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}
	item = shop.Items[item.ID]
	item.FileID = m.Photo.FileID
	item.TbPhoto = m.Photo
	shop.Items[item.ID] = item
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	bot.tryDeleteMessage(m)
	bot.trySendMessage(m.Sender, fmt.Sprintf("✅ Image added."))
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	bot.shopNextItemButtonHandler(ctx, &tb.Callback{Message: shopView.Message})
	// // tx := &Shop{Base: transaction.New(transaction.ID(item.ShopID))}
	// // sn, err := tx.Get(tx, bot.Bunt)
	// // // immediatelly set intransaction to block duplicate calls
	// // if err != nil {
	// // 	log.Errorf("[shopNewItemHandler] %s", err)
	// // 	return
	// // }
	// // shop := sn.(*Shop)
	// // switch from sqlite to bunt db
	// item = shop.Items[item.ID]
	// // user := LoadUser(ctx)
	// // onnly the correct user can press

	// // immediatelly set lock to block duplicate calls
	// err = shop.Lock(shop, bot.Bunt)
	// defer shop.Release(shop, bot.Bunt)

	// item.FileID = m.Photo.FileID
	// item.TbPhoto = m.Photo
	// fmt.Println(m.Photo.OnDisk())  // true
	// fmt.Println(m.Photo.InCloud()) // false
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
	return shop, nil
}
