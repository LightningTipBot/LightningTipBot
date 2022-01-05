package admin

import (
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

func (s Service) UnbanUser(w http.ResponseWriter, r *http.Request) {
	user, err := s.getUserByTelegramId(r)
	if err != nil {
		log.Errorf("[ADMIN] could not ban user: %v", err)
		return
	}
	if !user.Banned {
		log.Infof("[ADMIN] user already banned")
		return
	}
	user.Banned = false
	adminSlice := strings.Split(user.Wallet.Adminkey, "_")
	user.Wallet.Adminkey = adminSlice[len(adminSlice)-1]
	s.db.Save(user)
}

func (s Service) BanUser(w http.ResponseWriter, r *http.Request) {
	user, err := s.getUserByTelegramId(r)
	if err != nil {
		log.Errorf("[ADMIN] could not ban user: %v", err)
		return
	}
	if user.Banned {
		log.Infof("[ADMIN] user already banned")
		return
	}
	user.Banned = true
	if reason := r.URL.Query().Get("reason"); reason != "" {
		user.Wallet.Adminkey = fmt.Sprintf("%s_%s", r.URL.Query().Get("reason"), user.Wallet.Adminkey)
	}
	user.Wallet.Adminkey = fmt.Sprintf("%s_%s", "banned", user.Wallet.Adminkey)
	s.db.Save(user)
}

func (s Service) getUserByTelegramId(r *http.Request) (*lnbits.User, error) {
	user := &lnbits.User{}
	v := mux.Vars(r)
	if v["id"] == "" {
		return nil, fmt.Errorf("invalid id")
	}
	tx := s.db.Where("telegram_id = ? COLLATE NOCASE", v["id"]).First(user)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return user, nil
}
