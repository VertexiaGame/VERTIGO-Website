package repository

import (
	"database/sql"
	"errors"

	"vertexia-frontend/backend/models"
)

func savePostImage(tx *sql.Tx, ownerID int, image models.ForumImageUpload) (int64, error) {
	if image.SavedID != 0 {
		var id int64
		err := tx.QueryRow("SELECT id FROM community_images WHERE id = ? AND owner_id = ? AND approved = TRUE",
			image.SavedID, ownerID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, models.ErrSavedImageUnavailable
		}
		return id, err
	}
	result, err := tx.Exec("INSERT INTO community_images (owner_id, name, content_type, content) VALUES (?, ?, ?, ?)",
		ownerID, image.Name, image.ContentType, image.Content)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *ForumRepository) SavedImages(ownerID int, before int64, limit int) ([]models.SavedImage, error) {
	if r.db == nil {
		return nil, errors.New("database connection is offline")
	}
	query := "SELECT id, name, approved FROM community_images WHERE owner_id = ?"
	args := []any{ownerID}
	if before > 0 {
		query += " AND id < ?"
		args = append(args, before)
	}
	rows, err := r.db.Query(query+" ORDER BY id DESC LIMIT ?", append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	images := make([]models.SavedImage, 0)
	for rows.Next() {
		var image models.SavedImage
		if err := rows.Scan(&image.ID, &image.Name, &image.Approved); err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, rows.Err()
}

func (r *ForumRepository) SavedImage(ownerID int, id int64) (models.ForumImageUpload, error) {
	var image models.ForumImageUpload
	if r.db == nil {
		return image, errors.New("database connection is offline")
	}
	err := r.db.QueryRow("SELECT content_type, content FROM community_images WHERE id = ? AND owner_id = ?", id, ownerID).
		Scan(&image.ContentType, &image.Content)
	return image, err
}
