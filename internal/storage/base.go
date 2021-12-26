package storage

import (
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"time"

	"github.com/eko/gocache/store"
	gocache "github.com/patrickmn/go-cache"

	log "github.com/sirupsen/logrus"
)

var transactionCache = store.NewGoCache(gocache.New(5*time.Minute, 10*time.Minute), nil)

type Base struct {
	ID        string    `json:"id"`
	Active    bool      `json:"active"`
	Canceled  bool      `json:"canceled"`
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
	cacheTx, err := transactionCache.Get(s.Key())
	if err != nil {
		err := db.Get(s)
		if err != nil {
			return s, err
		}
		log.Tracef("[Bunt] get object %s", s.Key())
		return s, transactionCache.Set(s.Key(), s, &store.Options{Expiration: 5 * time.Minute})
	}
	log.Tracef("[Bunt Cache] get object %s", s.Key())
	return cacheTx.(Storable), err

}

func (tx *Base) Set(s Storable, db *DB) error {
	tx.UpdatedAt = time.Now()
	err := db.Set(s)
	if err != nil {
		log.Errorf("[Bunt] could not set object: %v", err.Error())
		return err
	}
	log.Tracef("[Bunt] set object %s", s.Key())
	err = transactionCache.Set(s.Key(), s, &store.Options{Expiration: 5 * time.Minute})
	if err != nil {
		log.Errorf("[Bunt Cache] could not set object: %v", err.Error())
	}
	log.Tracef("[Bunt Cache] set object: %s", s.Key())
	return err
}

func (tx *Base) Delete(s Storable, db *DB) error {
	tx.UpdatedAt = time.Now()
	runtime.IgnoreError(transactionCache.Delete(s.Key()))
	return db.Delete(s.Key(), s)
}
