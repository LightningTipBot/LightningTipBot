package storage

import (
	"fmt"
	"time"
)

type BaseTransaction struct {
	ID            string    `json:"id"`
	Active        bool      `json:"active"`
	InTransaction bool      `json:"intransaction"`
	CreatedAt     time.Time `json:"created"`
	UpdatedAt     time.Time `json:"updated"`
}

func (msg BaseTransaction) Key() string {
	return msg.ID
}
func Lock(s Storable, tx *BaseTransaction, db *DB) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = true
	tx.UpdatedAt = time.Now()
	err := db.Set(s)
	if err != nil {
		return err
	}
	return nil
}

func Release(s Storable, tx *BaseTransaction, db *DB) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = false
	tx.UpdatedAt = time.Now()
	err := db.Set(s)
	if err != nil {
		return err
	}
	return nil
}

func Inactivate(s Storable, tx *BaseTransaction, db *DB) error {
	tx.Active = false
	tx.UpdatedAt = time.Now()
	err := db.Set(s)
	if err != nil {
		return err
	}
	return nil
}

func GetTransaction(s Storable, tx *BaseTransaction, db *DB) (Storable, error) {
	err := db.Get(s)
	if err != nil {
		return s, err
	}
	// to avoid race conditions, we block the call if there is
	// already an active transaction by loop until InTransaction is false
	ticker := time.NewTicker(time.Second * 10)
	for tx.InTransaction {
		select {
		case <-ticker.C:
			return nil, fmt.Errorf("transaction timeout")
		default:
			time.Sleep(time.Duration(500) * time.Millisecond)
			err = db.Get(s)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get transaction")
	}

	return s, nil
}
