package storage

import (
	"encoding/json"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/buntdb"
)

// Storable items must provide a function to retrieve the database key
type Storable interface {
	Key() string
	Index() string
}

type DB struct {
	*buntdb.DB
}

func NewBunt() *DB {
	db, err := buntdb.Open("data/bunt.db")
	if err != nil {
		log.Fatal(err)
	}
	err = db.CreateIndex("messages", "*", buntdb.IndexJSON("Message.message_id"))
	if err != nil {
		panic(err)
	}

	return &DB{db}
}

// Exists checks is storable item exists
func (db *DB) Exists(storable Storable) (ok bool, err error) {
	ok = false
	err = db.View(func(tx *buntdb.Tx) error {
		_, err := tx.Get(storable.Key())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		if err == buntdb.ErrNotFound {
			return
		}
		return
	}
	ok = true
	return

}

// Get a storable item
func (db *DB) Get(object Storable) error {
	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(object.Key())
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(val), object)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

// Set a storable item.
func (db *DB) Set(object Storable) error {
	err := db.Update(func(tx *buntdb.Tx) error {
		b, err := json.Marshal(object)
		if err != nil {
			return err
		}
		_, _, err = tx.Set(object.Key(), string(b), nil)

		return err
	})
	return err
}

// Delete a storable item.
// todo -- not ascend users index
func (db *DB) Delete(object Storable) error {
	return db.Update(func(tx *buntdb.Tx) error {
		var delkeys []string
		runtime.IgnoreError(
			tx.Ascend(object.Index(), func(key, value string) bool {
				if key == object.Key() {
					delkeys = append(delkeys, key)
				}
				return true
			}),
		)
		for _, k := range delkeys {
			if _, err := tx.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})

}
