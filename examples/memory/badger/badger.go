// Package badger provides a BadgerDB implementation of the memory.Store interface.
// It offers persistent storage for conversation and context memory using the
// high-performance, embedded key-value database BadgerDB.
package badger

import (
	"github.com/dgraph-io/badger/v4"

	"github.com/calque-ai/go-calque/pkg/middleware/memory"
)

// Store implements the memory.Store interface using BadgerDB
type Store struct {
	db *badger.DB
}

// NewStore initializes a new Store at the given path
func NewStore(path string) (*Store, error) {
	opts := badger.DefaultOptions(path)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Verify it implements the interface
var _ memory.Store = (*Store)(nil)

// Get retrieves a value by key, returns nil if not found
func (s *Store) Get(key string) ([]byte, error) {
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

// Set stores a value by key
func (s *Store) Set(key string, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
}

// Delete removes a value by key
func (s *Store) Delete(key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// List returns all keys in the store
func (s *Store) List() []string {
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

// Exists checks if a key exists
func (s *Store) Exists(key string) bool {
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key))
		return err
	})
	return err == nil
}
