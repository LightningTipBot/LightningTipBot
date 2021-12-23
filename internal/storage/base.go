package storage

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

type Base struct {
	ID        string    `json:"id"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created"`
	UpdatedAt time.Time `json:"updated"`
}

type Option func(b *Base)

func ID(id string) Option {
	return func(btx *Base) {
		btx.ID = id
	}
}

func New(opts ...Option) *Base {
	btx := &Base{
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(btx)
	}
	return btx
}

func (tx Base) Key() string {
	return tx.ID
}

func (tx *Base) Inactivate(s Storable, db *DB) error {
	tx.Active = false
	err := tx.Set(s, db)
	if err != nil {
		log.Tracef("[Bunt Inactivate] %s Error: %s", tx.ID, err.Error())
		return err
	}
	log.Tracef("[Bunt Inactivate] %s", tx.ID)
	return nil
}

func (tx *Base) Get(s Storable, db *DB) (Storable, error) {
	err := db.Get(s)
	if err != nil {
		return s, err
	}
	if err != nil {
		return nil, fmt.Errorf("could not get transaction")
	}

	return s, nil
}

func (tx *Base) Set(s Storable, db *DB) error {
	tx.UpdatedAt = time.Now()
	return db.Set(s)
}

func (tx *Base) Delete(s Storable, db *DB) error {
	tx.UpdatedAt = time.Now()
	return db.Delete(s.Key(), s)
}
