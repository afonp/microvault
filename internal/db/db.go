package db

import (
	"database/sql"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// store handles interactions with the metadata database
type Store struct {
	db *sql.DB
}

// new_store initializes the database connection and schema
func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// create blobs table if it doesn't exist
	// key: the blob key (user provided or hash)
	// volume_id: the identifier of the volume where the blob is stored
	// for now we might just store the full url or a simple id
	query := `
	CREATE TABLE IF NOT EXISTS blobs (
		key TEXT PRIMARY KEY,
		volume_id TEXT
	);`

	if _, err := db.Exec(query); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// put_blob records a blob's location(s)
func (s *Store) PutBlob(key string, volumeIDs []string) error {
	val := strings.Join(volumeIDs, ",")
	_, err := s.db.Exec("INSERT OR REPLACE INTO blobs (key, volume_id) VALUES (?, ?)", key, val)
	return err
}

// get_blob retrieves a blob's location(s)
func (s *Store) GetBlob(key string) ([]string, error) {
	var val string
	err := s.db.QueryRow("SELECT volume_id FROM blobs WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	return strings.Split(val, ","), nil
}

// list_keys returns all keys in the store
func (s *Store) ListKeys() ([]string, error) {
	rows, err := s.db.Query("SELECT key FROM blobs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// delete_blob removes a blob's metadata
func (s *Store) DeleteBlob(key string) error {
	_, err := s.db.Exec("DELETE FROM blobs WHERE key = ?", key)
	return err
}
