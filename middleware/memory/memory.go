package memory

// Store interface for memory backends
type Store interface {
	// Get retrieves data for a key
	Get(key string) ([]byte, error)

	// Set stores data for a key
	Set(key string, value []byte) error

	// Delete removes data for a key
	Delete(key string) error

	// List returns all keys
	List() []string

	// Exists checks if a key exists
	Exists(key string) bool
}
