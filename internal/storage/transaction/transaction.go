package transaction

import (
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/storage"
)

type Base struct {
	ID            string    `json:"id"`
	Active        bool      `json:"active"`
	InTransaction bool      `json:"intransaction"`
	CreatedAt     time.Time `json:"created"`
	UpdatedAt     time.Time `json:"updated"`
}

type Option func(b *Base)

func ID(id string) Option {
	return func(btx *Base) {
		btx.ID = id
	}
}

func New(opts ...Option) *Base {
	btx := &Base{
		Active:        true,
		InTransaction: false,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	for _, opt := range opts {
		opt(btx)
	}
	return btx
}

func (tx Base) Key() string {
	return tx.ID
}
func (tx *Base) Lock(s storage.Storable, db *storage.DB) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = true
	err := tx.Set(s, db)
	if err != nil {
		return err
	}
	return nil
}

func (tx *Base) Release(s storage.Storable, db *storage.DB) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = false
	err := tx.Set(s, db)
	if err != nil {
		return err
	}
	return nil
}

func (tx *Base) Inactivate(s storage.Storable, db *storage.DB) error {
	tx.Active = false
	err := tx.Set(s, db)
	if err != nil {
		return err
	}
	return nil
}

func (tx *Base) Get(s storage.Storable, db *storage.DB) (storage.Storable, error) {
	err := db.Get(s)
	if err != nil {
		return s, err
	}
	// to avoid race conditions, we block the call if there is
	// already an active transaction by loop until InTransaction is false
	ticker := time.NewTicker(time.Millisecond * 100)
	for tx.InTransaction {
		select {
		case <-ticker.C:
			return nil, fmt.Errorf("transaction timeout")
		default:
			time.Sleep(time.Duration(100) * time.Millisecond)
			err = db.Get(s)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get transaction")
	}

	return s, nil
}

func (tx *Base) Set(s storage.Storable, db *storage.DB) error {
	tx.UpdatedAt = time.Now()
	return db.Set(s)
}
