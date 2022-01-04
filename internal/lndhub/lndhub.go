package lndhub

import (
	"encoding/base64"
	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/api"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net/http"
	"strings"
)

type LndHub struct {
	database *gorm.DB
}

func New(bot *telegram.TipBot) LndHub {
	return LndHub{database: bot.Database}
}
func (w LndHub) Handle(writer http.ResponseWriter, request *http.Request) {
	auth := request.Header.Get("Authorization")
	if auth == "" {
		return
	}
	username, password, ok := parseBearerAuth(auth)
	if !ok {
		return
	}
	log.Debugf("[LNDHUB] %s, %s", username, password)
	if strings.Contains(password, "_") || strings.HasPrefix(password, "banned_") {
		log.Warnf("[LNDHUB] Banned user. Not forwarding request")
		return
	}
	user := &lnbits.User{}
	tx := w.database.Where("wallet_adminkey = ? COLLATE NOCASE", password).First(user)
	if tx.Error != nil {
		log.Warnf("[LNDHUB] wallet admin key (%s) not found: %v", password, tx.Error)
		return
	}
	api.Proxy(writer, request, internal.Configuration.Lnbits.Url)

}

// parseBasicAuth parses an HTTP Basic Authentication string.
// "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==" returns ("Aladdin", "open sesame", true).
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
