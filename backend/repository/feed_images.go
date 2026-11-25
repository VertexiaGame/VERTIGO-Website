package repository

import (
	"database/sql"
	"errors"

	"vertexia-frontend/backend/models"
)

const feedImagePostJoins = `
	LEFT JOIN feed f ON i.feed_type = 'worldwide' AND f.id = i.feed_id AND f.removed = 'false'
	LEFT JOIN ffeed ff ON i.feed_type = 'friends' AND ff.id = i.feed_id AND ff.removed = 'false'`

func (r *FeedRepository) CreatePost(userID int, content, feedType string, images []models.ForumImageUpload) (*models.FeedPost, error) {
	if r.db == nil {
		return nil, errors.New("feed is unavailable")
	}
	if len(images) > models.ForumImageMaxCount {
		return nil, errors.New("attach up to 3 images")
	}

	table, authorColumn := "feed", "user_id"
	if feedType == "friends" {
		table, authorColumn = "ffeed", "userid"
	} else {
		feedType = "worldwide"
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := "INSERT INTO " + table + " (id, " + authorColumn + ", status) " +
		"VALUES (LAST_INSERT_ID((SELECT COALESCE(MAX(x.id), 0) + 1 FROM (SELECT id FROM " + table + ") x)), ?, ?)"
	if _, err := tx.Exec(query, userID, content); err != nil {
		return nil, err
	}
	post := &models.FeedPost{UserID: userID, Content: content}
	if err := tx.QueryRow("SELECT LAST_INSERT_ID()").Scan(&post.ID); err != nil {
		return nil, err
	}

	for _, image := range images {
		imageID, err := savePostImage(tx, userID, image)
		if err != nil {
			return nil, err
		}
		attachment, err := tx.Exec("INSERT INTO feed_images (feed_id, feed_type, image_id) VALUES (?, ?, ?)",
			post.ID, feedType, imageID)
		if err != nil {
			return nil, err
		}
		if image.SavedID != 0 {
			id, err := attachment.LastInsertId()
			if err != nil {
				return nil, err
			}
			post.Images = append(post.Images, int(id))
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return post, nil
}

func (r *FeedRepository) GetImage(id, currentUserID int) (models.ForumImageUpload, error) {
	var image models.ForumImageUpload
	if r.db == nil {
		return image, sql.ErrNoRows
	}
	err := r.db.QueryRow(`SELECT a.content_type, a.content
		FROM feed_images i JOIN community_images a ON a.id = i.image_id`+feedImagePostJoins+`
		WHERE i.id = ? AND a.approved = TRUE AND (f.id IS NOT NULL OR (ff.id IS NOT NULL AND
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
	err := r.db.QueryRow(`SELECT a.content_type, a.content FROM feed_images i
		JOIN community_images a ON a.id = i.image_id`+feedImagePostJoins+`
		WHERE i.id = ? AND a.approved = FALSE AND (f.id IS NOT NULL OR ff.id IS NOT NULL)`, id).
		Scan(&image.ContentType, &image.Content)
	return image, err
}

func (r *FeedRepository) PendingImages(after, limit int) ([]models.PendingFeedImage, error) {
	if r.db == nil {
		return nil, errors.New("feed is unavailable")
	}
	rows, err := r.db.Query(`SELECT i.id, i.feed_id, i.feed_type, COALESCE(u.username, 'Player'),
		COALESCE(f.status, ff.status, '')
		FROM feed_images i JOIN community_images a ON a.id = i.image_id`+feedImagePostJoins+`
		LEFT JOIN users u ON u.id = COALESCE(f.user_id, ff.userid)
		WHERE a.approved = FALSE AND i.id > ? AND (f.id IS NOT NULL OR ff.id IS NOT NULL)
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
	query := `DELETE FROM community_images WHERE approved = FALSE
		AND id = (SELECT image_id FROM feed_images WHERE id = ?)`
	if approve {
		query = "UPDATE community_images a JOIN feed_images i ON i.image_id = a.id" + feedImagePostJoins +
			" SET a.approved = TRUE WHERE i.id = ? AND a.approved = FALSE AND (f.id IS NOT NULL OR ff.id IS NOT NULL)"
	}
	result, err := r.db.Exec(query, id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
