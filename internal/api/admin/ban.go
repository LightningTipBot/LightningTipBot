package admin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func (s Service) UnbanUser(w http.ResponseWriter, r *http.Request) {
	user, err := s.getUserByTelegramId(r)
	if err != nil {
		log.Errorf("[ADMIN] could not ban user: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !user.Banned && !strings.HasPrefix(user.Wallet.Adminkey, "banned_") {
		log.Infof("[ADMIN] user is not banned. Aborting.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	user.Banned = false
	adminSlice := strings.Split(user.Wallet.Adminkey, "_")
	user.Wallet.Adminkey = adminSlice[len(adminSlice)-1]
	err = telegram.UpdateUserRecord(user, *s.bot)
	if err != nil {
		log.Errorf("[ADMIN] could not update user: %v", err)
		return
	}
	log.Infof("[ADMIN] Unbanned user (%s)", user.ID)
	w.WriteHeader(http.StatusOK)
}

func (s Service) BanUser(w http.ResponseWriter, r *http.Request) {
	user, err := s.getUserByTelegramId(r)
	if err != nil {
		log.Errorf("[ADMIN] could not ban user: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if user.Banned {
		w.WriteHeader(http.StatusBadRequest)
		log.Infof("[ADMIN] user is already banned. Aborting.")
		return
	}
	user.Banned = true
	if reason := r.URL.Query().Get("reason"); reason != "" {
		user.Wallet.Adminkey = fmt.Sprintf("%s_%s", reason, user.Wallet.Adminkey)
	}
	user.Wallet.Adminkey = fmt.Sprintf("%s_%s", "banned", user.Wallet.Adminkey)
	err = telegram.UpdateUserRecord(user, *s.bot)
	if err != nil {
		log.Errorf("[ADMIN] could not update user: %v", err)
		return
	}

	log.Infof("[ADMIN] Banned user (%s)", user.ID)
	w.WriteHeader(http.StatusOK)
}

func (s Service) getUserByTelegramId(r *http.Request) (*lnbits.User, error) {
	user := &lnbits.User{}
	v := mux.Vars(r)
	if v["id"] == "" {
		return nil, fmt.Errorf("invalid id")
	}
	tx := s.bot.DB.Users.Where("telegram_id = ? COLLATE NOCASE", v["id"]).First(user)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return user, nil
}
