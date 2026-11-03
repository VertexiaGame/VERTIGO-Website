package repository

import (
	"context"
	"database/sql"
)

type JevCacheRepository struct {
	db *sql.DB
}

func NewJevCacheRepository(db *sql.DB) *JevCacheRepository {
	return &JevCacheRepository{db: db}
}

func (r *JevCacheRepository) Get(ctx context.Context, key []byte) ([]byte, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, nil
	}
	var result []byte
	if err := r.db.QueryRowContext(ctx, "SELECT result FROM jev_cache WHERE content_hash = ?", key).Scan(&result); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return result, true, nil
}

func (r *JevCacheRepository) Put(ctx context.Context, key, result []byte) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, "INSERT IGNORE INTO jev_cache (content_hash, result) VALUES (?, ?)", key, result)
	return err
}
