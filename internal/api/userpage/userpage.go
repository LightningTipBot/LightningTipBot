package userpage

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	"github.com/PuerkitoBio/goquery"
	"github.com/fiatjaf/go-lnurl"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type Service struct {
	bot *telegram.TipBot
}

func New(b *telegram.TipBot) Service {
	return Service{
		bot: b,
	}
}

const botImage = "https://avatars.githubusercontent.com/u/88730856?v=7"

//go:embed static
var templates embed.FS
var tmpl = template.Must(template.ParseFS(templates, "static/userpage.html"))

var Client = &http.Client{
	Timeout: 10 * time.Second,
}

// thank you fiatjaf for this code
func (s Service) getTelegramUserPictureURL(username string) (string, error) {
	// with proxy:
	// client, err := s.network.GetHttpClient()
	// if err != nil {
	// 	return "", err
	// }
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://t.me/" + username)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	url, ok := doc.Find(`meta[property="og:image"]`).First().Attr("content")
	if !ok {
		return "", errors.New("no image available for this user")
	}

	return url, nil
}

func (s Service) UserPageHandler(w http.ResponseWriter, r *http.Request) {
	// https://ln.tips/.well-known/lnurlp/<username>
	username := strings.ToLower(mux.Vars(r)["username"])
	callback := fmt.Sprintf("%s/.well-known/lnurlp/%s", internal.Configuration.Bot.LNURLHostName, username)
	log.WithFields(log.Fields{
		"module": "api",
		"func":   "UserPageHandler",
		"user":   username}).Infof("rendering page")
	lnurlEncode, err := lnurl.LNURLEncode(callback)
	if err != nil {
		log.WithFields(log.Fields{
			"module": "api",
			"func":   "UserPageHandler",
			"user":   username,
			"error":  err.Error()}).Errorln("[UserPage]", "error encoding lnurl")
		return
	}
	image, err := s.getTelegramUserPictureURL(username)
	if err != nil || image == "https://telegram.org/img/t_logo.png" {
		// replace the default image
		image = botImage
	}

	if err := tmpl.ExecuteTemplate(w, "userpage", struct {
		Username string
		Image    string
		LNURLPay string
	}{username, image, lnurlEncode}); err != nil {
		log.WithFields(log.Fields{
			"module": "api",
			"func":   "UserPageHandler",
			"user":   username,
			"error":  err.Error()}).Errorf("failed to render template")
	}
}
