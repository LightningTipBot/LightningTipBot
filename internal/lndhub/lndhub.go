package lndhub

import (
	"encoding/base64"
	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/api"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

type LndHub struct {
}

func New() LndHub {
	return LndHub{}
}
func (w LndHub) Handle(writer http.ResponseWriter, request *http.Request) {
	api.Proxy(writer, request, internal.Configuration.Lnbits.Url)
	auth := request.Header.Get("Authorization")
	if auth == "" {
		return
	}
	username, password, ok := parseBearerAuth(auth)
	if !ok {
		return
	}
	log.Tracef("[LNDHUB] %s, %s", username, password)
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
