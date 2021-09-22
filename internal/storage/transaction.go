package storage

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"time"
)

type Transaction struct {
	ID            string `json:"id"`
	Active        bool   `json:"active"`
	InTransaction bool   `json:"intransaction"`
}

func (msg Transaction) Key() string {
	return msg.ID
}
func Lock(s Storable, tx *Transaction, db *DB) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = true
	err := db.Set(s)
	if err != nil {
		return err
	}
	return nil
}

func Release(s Storable, tx *Transaction, db *DB) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = false
	err := db.Set(s)
	if err != nil {
		return err
	}
	return nil
}

func Inactivate(s Storable, tx *Transaction, db *DB) error {
	tx.Active = false
	err := db.Set(s)
	if err != nil {
		return err
	}
	return nil
}

func GetTransaction(s Storable, tx *Transaction, db *DB) (Storable, error) {
	err := db.Get(s)

	// to avoid race conditions, we block the call if there is
	// already an active transaction by loop until InTransaction is false
	ticker := time.NewTicker(time.Second * 10)
	for tx.InTransaction {
		select {
		case <-ticker.C:
			return nil, fmt.Errorf("send timeout")
		default:
			log.Infoln("[send] in transaction")
			time.Sleep(time.Duration(500) * time.Millisecond)
			err = db.Get(s)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get sendData")
	}

	return s, nil
}
