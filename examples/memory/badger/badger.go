package badger

import (
	"github.com/calque-ai/go-calque/pkg/middleware/memory"
	"github.com/dgraph-io/badger/v4"
)

type BadgerStore struct {
	db *badger.DB
}

func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{db: db}, nil
}

// Verify it implements the interface
var _ memory.Store = (*BadgerStore)(nil)

func (s *BadgerStore) Get(key string) ([]byte, error) {
	var result []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			return nil // Not found
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			result = append([]byte(nil), val...) // Copy
			return nil
		})
	})
	return result, err
}

func (s *BadgerStore) Set(key string, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
}

func (s *BadgerStore) Delete(key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

func (s *BadgerStore) List() []string {
	var keys []string
	s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			keys = append(keys, string(it.Item().Key()))
		}
		return nil
	})
	return keys
}

func (s *BadgerStore) Exists(key string) bool {
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key))
		return err
	})
	return err == nil
}
