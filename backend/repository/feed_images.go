package repository

import (
	"database/sql"
	"errors"

	"vertexia-frontend/backend/models"
)

const feedImagePostJoins = `
	LEFT JOIN feed f ON i.feed_type = 'worldwide' AND f.id = i.feed_id AND f.removed = 'false'
	LEFT JOIN ffeed ff ON i.feed_type = 'friends' AND ff.id = i.feed_id AND ff.removed = 'false'`

func (r *FeedRepository) CreatePost(userID int, content, feedType string, images []models.ForumImageUpload) (int, error) {
	if r.db == nil {
		return 0, errors.New("feed is unavailable")
	}
	if len(images) > models.ForumImageMaxCount {
		return 0, errors.New("attach up to 3 images")
	}

	table, authorColumn := "feed", "user_id"
	if feedType == "friends" {
		table, authorColumn = "ffeed", "userid"
	} else {
		feedType = "worldwide"
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := "INSERT INTO " + table + " (id, " + authorColumn + ", status) " +
		"VALUES (LAST_INSERT_ID((SELECT COALESCE(MAX(x.id), 0) + 1 FROM (SELECT id FROM " + table + ") x)), ?, ?)"
	if _, err := tx.Exec(query, userID, content); err != nil {
		return 0, err
	}
	var postID int
	if err := tx.QueryRow("SELECT LAST_INSERT_ID()").Scan(&postID); err != nil {
		return 0, err
	}

	for _, image := range images {
		if _, err := tx.Exec("INSERT INTO feed_images (feed_id, feed_type, content_type, content) VALUES (?, ?, ?, ?)",
			postID, feedType, image.ContentType, image.Content); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return postID, nil
}

func (r *FeedRepository) GetImage(id, currentUserID int) (models.ForumImageUpload, error) {
	var image models.ForumImageUpload
	if r.db == nil {
		return image, sql.ErrNoRows
	}
	err := r.db.QueryRow(`SELECT i.content_type, i.content
		FROM feed_images i`+feedImagePostJoins+`
		WHERE i.id = ? AND i.approved = TRUE AND (f.id IS NOT NULL OR (ff.id IS NOT NULL AND
			(ff.userid = ? OR EXISTS (
				SELECT 1 FROM friends WHERE state = 'accepted'
				AND ((uid = ? AND fid = ff.userid) OR (fid = ? AND uid = ff.userid))
			))))`, id, currentUserID, currentUserID, currentUserID).
		Scan(&image.ContentType, &image.Content)
	return image, err
}

func (r *FeedRepository) GetPendingImage(id int) (models.ForumImageUpload, error) {
	var image models.ForumImageUpload
	if r.db == nil {
		return image, sql.ErrNoRows
	}
	err := r.db.QueryRow(`SELECT i.content_type, i.content FROM feed_images i`+feedImagePostJoins+`
		WHERE i.id = ? AND i.approved = FALSE AND (f.id IS NOT NULL OR ff.id IS NOT NULL)`, id).
		Scan(&image.ContentType, &image.Content)
	return image, err
}

func (r *FeedRepository) PendingImages(after, limit int) ([]models.PendingFeedImage, error) {
	if r.db == nil {
		return nil, errors.New("feed is unavailable")
	}
	rows, err := r.db.Query(`SELECT i.id, i.feed_id, i.feed_type, COALESCE(u.username, 'Player'),
		COALESCE(f.status, ff.status, '')
		FROM feed_images i`+feedImagePostJoins+`
		LEFT JOIN users u ON u.id = COALESCE(f.user_id, ff.userid)
		WHERE i.approved = FALSE AND i.id > ? AND (f.id IS NOT NULL OR ff.id IS NOT NULL)
		ORDER BY i.id LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var images []models.PendingFeedImage
	for rows.Next() {
		var image models.PendingFeedImage
		if err := rows.Scan(&image.ID, &image.FeedID, &image.FeedType, &image.Author, &image.Body); err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, rows.Err()
}

func (r *FeedRepository) ReviewImage(id int, approve bool) (bool, error) {
	if r.db == nil {
		return false, errors.New("feed is unavailable")
	}
	query := "DELETE FROM feed_images WHERE id = ? AND approved = FALSE"
	if approve {
		query = "UPDATE feed_images i" + feedImagePostJoins +
			" SET i.approved = TRUE WHERE i.id = ? AND i.approved = FALSE AND (f.id IS NOT NULL OR ff.id IS NOT NULL)"
	}
	result, err := r.db.Exec(query, id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
