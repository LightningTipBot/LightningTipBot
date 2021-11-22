package transaction

import (
	"fmt"
	"sync"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	log "github.com/sirupsen/logrus"
)

type Base struct {
	ID            string    `json:"id"`
	Active        bool      `json:"active"`
	InTransaction bool      `json:"intransaction"`
	CreatedAt     time.Time `json:"created"`
	UpdatedAt     time.Time `json:"updated"`
}

func init() {
	transactionMutex = make(map[string]*sync.Mutex, 0)
	transactionMapMutex = &sync.Mutex{}
}

var transactionMutex map[string]*sync.Mutex
var transactionMapMutex *sync.Mutex

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
		log.Debugf("[Bunt Lock] %s Error: %s", tx.ID, err.Error())
		return err
	}
	log.Debugf("[Lock] %s", tx.ID)
	return nil
}
func unlock(id string) {
	transactionMapMutex.Lock()
	if transactionMutex[id] != nil {
		transactionMutex[id].Unlock()
		log.Tracef("[TX mutex] Release %s", id)
	}
	transactionMapMutex.Unlock()

}

func (tx *Base) Release(s storage.Storable, db *storage.DB) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = false
	err := tx.Set(s, db)
	if err != nil {
		log.Debugf("[Bunt Release] %s Error: %s", tx.ID, err.Error())
		return err
	}
	log.Debugf("[Bunt Release] %s", tx.ID)
	unlock(tx.ID)
	return nil
}

func (tx *Base) Inactivate(s storage.Storable, db *storage.DB) error {
	tx.Active = false
	err := tx.Set(s, db)
	if err != nil {
		log.Debugf("[Bunt Inactivate] %s Error: %s", tx.ID, err.Error())
		return err
	}
	log.Debugf("[Bunt Inactivate] %s", tx.ID)
	return nil
}

func (tx *Base) Get(s storage.Storable, db *storage.DB) (storage.Storable, error) {
	transactionMapMutex.Lock()
	if transactionMutex[tx.ID] == nil {
		transactionMutex[tx.ID] = &sync.Mutex{}
	}
	transactionMapMutex.Unlock()
	transactionMutex[tx.ID].Lock()
	log.Tracef("[TX mutex] Lock %s", tx.ID)

	err := db.Get(s)
	if err != nil {
		unlock(tx.ID)
		return s, err
	}
	// to avoid race conditions, we block the call if there is
	// already an active transaction by loop until InTransaction is false
	ticker := time.NewTicker(time.Millisecond * 100)
	for tx.InTransaction {
		select {
		case <-ticker.C:
			unlock(tx.ID)
			return nil, fmt.Errorf("[Bunt Lock] transaction timeout %s", tx.ID)
		default:
			time.Sleep(time.Duration(75) * time.Millisecond)
			err = db.Get(s)
		}
	}
	if err != nil {
		unlock(tx.ID)
		return nil, fmt.Errorf("could not get transaction")
	}

	return s, nil
}

func (tx *Base) Set(s storage.Storable, db *storage.DB) error {
	tx.UpdatedAt = time.Now()
	return db.Set(s)
}

func (tx *Base) Delete(s storage.Storable, db *storage.DB) error {
	tx.UpdatedAt = time.Now()
	return db.Delete(s.Key(), s)
}
