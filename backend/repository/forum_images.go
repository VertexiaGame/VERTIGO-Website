package repository

import (
	"database/sql"
	"errors"
	"strings"

	"vertexia-frontend/backend/models"
)

func (r *ForumRepository) createForumPost(query string, args []any, threadID int, images []models.ForumImageUpload) (int, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	if len(images) > models.ForumImageMaxCount {
		return 0, errors.New("attach up to 3 images")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	var replyID any
	if threadID == 0 {
		threadID = int(id)
	} else {
		replyID = id
	}
	for _, image := range images {
		if _, err := tx.Exec(`INSERT INTO forum_images (thread_id, reply_id, content_type, content)
			VALUES (?, ?, ?, ?)`, threadID, replyID, image.ContentType, image.Content); err != nil {
			return 0, err
		}
	}
	return int(id), tx.Commit()
}

func (r *ForumRepository) LoadApprovedImages(thread *models.ForumThread, replies []*models.ForumReply) error {
	args := []any{thread.ID}
	repliesByID := make(map[int]*models.ForumReply, len(replies))
	query := "SELECT id, reply_id FROM forum_images WHERE thread_id = ? AND approved = TRUE AND (reply_id IS NULL"
	if len(replies) > 0 {
		query += " OR reply_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(replies)), ",") + ")"
		for _, reply := range replies {
			args = append(args, reply.ID)
			repliesByID[reply.ID] = reply
		}
	}
	rows, err := r.db.Query(query+") ORDER BY id", args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var replyID sql.NullInt64
		if err := rows.Scan(&id, &replyID); err != nil {
			return err
		}
		if !replyID.Valid {
			thread.Images = append(thread.Images, id)
		} else if reply := repliesByID[int(replyID.Int64)]; reply != nil {
			reply.Images = append(reply.Images, id)
		}
	}
	return rows.Err()
}

func (r *ForumRepository) GetForumImage(id int, approved bool) (models.ForumImageUpload, error) {
	var image models.ForumImageUpload
	if r.db == nil {
		return image, errors.New("database connection is offline")
	}
	err := r.db.QueryRow("SELECT content_type, content FROM forum_images WHERE id = ? AND approved = ?", id, approved).
		Scan(&image.ContentType, &image.Content)
	return image, err
}

func (r *ForumRepository) PendingForumImages(after, limit int) ([]models.PendingForumImage, error) {
	if r.db == nil {
		return nil, errors.New("database connection is offline")
	}
	rows, err := r.db.Query(`SELECT i.id, t.id, COALESCE(i.reply_id, 0), t.title, COALESCE(u.username, 'Player'),
		COALESCE(r.body, t.body)
		FROM forum_images i
		JOIN forum_threads t ON t.id = i.thread_id
		LEFT JOIN forum_replies r ON r.id = i.reply_id
		LEFT JOIN users u ON u.id = COALESCE(r.user_id, t.user_id)
		WHERE i.approved = FALSE AND i.id > ? ORDER BY i.id LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var images []models.PendingForumImage
	for rows.Next() {
		var image models.PendingForumImage
		if err := rows.Scan(&image.ID, &image.ThreadID, &image.ReplyID, &image.Title, &image.Author, &image.Body); err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, rows.Err()
}

func (r *ForumRepository) ReviewForumImage(id int, approve bool) (bool, error) {
	if r.db == nil {
		return false, errors.New("database connection is offline")
	}
	query := "DELETE FROM forum_images WHERE id = ? AND approved = FALSE"
	if approve {
		query = "UPDATE forum_images SET approved = TRUE WHERE id = ? AND approved = FALSE"
	}
	result, err := r.db.Exec(query, id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
