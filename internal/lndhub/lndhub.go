package lndhub

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/api"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type LndHub struct {
	database *gorm.DB
}

func New(bot *telegram.TipBot) LndHub {
	return LndHub{database: bot.DB.Users}
}
func (w LndHub) Handle(writer http.ResponseWriter, request *http.Request) {
	auth := request.Header.Get("Authorization")

	// check if the user is banned
	if auth != "" {
		_, password, ok := parseBearerAuth(auth)
		if !ok {
			return
		}
		// first we make sure that the password is not already "banned_"
		if strings.Contains(password, "_") || strings.HasPrefix(password, "banned_") {
			log.WithFields(log.Fields{
				"module": "lndhub",
				"func":   "Handle",
				"user":   password}).Warn("Banned user . Not forwarding request")
			return
		}
		// then we check whether the "normal" password provided is in the database (it should be not if the user is banned)
		user := &lnbits.User{}
		tx := w.database.Where("wallet_adminkey = ? COLLATE NOCASE", password).First(user)
		if tx.Error != nil {
			log.WithFields(log.Fields{
				"module": "lndhub",
				"func":   "Handle",
				"user":   password}).Warnf("Could not get wallet admin key: %v", tx.Error)
			return
		}
		log.WithFields(log.Fields{
			"module": "lndhub",
			"func":   "Handle",
			"user":   telegram.GetUserStr(user.Telegram)}).Debugf("User")
	}
	// if not, proxy the request
	api.Proxy(writer, request, internal.Configuration.Lnbits.Url)
}

// parseBearerAuth parses an HTTP Basic Authentication string.
// "Bearer QWxhZGRpbjpvcGVuIHNlc2FtZQ==" returns ("Aladdin", "open sesame", true).
func parseBearerAuth(auth string) (username, password string, ok bool) {
	const prefix = "Bearer "
	// Case insensitive prefix match. See Issue 22736.
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}
