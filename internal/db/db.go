package db

import (
	"database/sql"

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

// put_blob records a blob's location
func (s *Store) PutBlob(key, volumeID string) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO blobs (key, volume_id) VALUES (?, ?)", key, volumeID)
	return err
}

// get_blob retrieves a blob's location
func (s *Store) GetBlob(key string) (string, error) {
	var volumeID string
	err := s.db.QueryRow("SELECT volume_id FROM blobs WHERE key = ?", key).Scan(&volumeID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return volumeID, err
}

// delete_blob removes a blob's metadata
func (s *Store) DeleteBlob(key string) error {
	_, err := s.db.Exec("DELETE FROM blobs WHERE key = ?", key)
	return err
}
