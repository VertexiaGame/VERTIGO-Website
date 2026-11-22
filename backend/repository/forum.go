package repository

import (
	"database/sql"
	"errors"

	"vertexia-frontend/backend/models"
)

type ForumRepository struct {
	db *sql.DB
}

func NewForumRepository(db *sql.DB) *ForumRepository {
	return &ForumRepository{db: db}
}

func (r *ForumRepository) GetBoards() ([]*models.ForumBoard, error) {
	if r.db == nil {
		return nil, nil
	}

	rows, err := r.db.Query("SELECT id, name, COALESCE(description, '') FROM forum_boards ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []*models.ForumBoard
	for rows.Next() {
		var b models.ForumBoard
		if err := rows.Scan(&b.ID, &b.Name, &b.Description); err != nil {
			return nil, err
		}
		boards = append(boards, &b)
	}
	return boards, rows.Err()
}

func (r *ForumRepository) GetBoardByID(id int) (*models.ForumBoard, error) {
	if r.db == nil || id <= 0 {
		return nil, nil
	}

	var b models.ForumBoard
	err := r.db.QueryRow("SELECT id, name, COALESCE(description, '') FROM forum_boards WHERE id = ?", id).
		Scan(&b.ID, &b.Name, &b.Description)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

const forumThreadSelect = `
	SELECT t.id, t.board_id, t.user_id,
	       COALESCE(u.username, 'Player'), COALESCE(u.displayname, ''),
	       t.title, t.body,
	       (SELECT COUNT(*) FROM forum_replies r WHERE r.thread_id = t.id) AS replies_count,
	       t.pinned, t.locked, t.created_at, t.updated_at
	FROM forum_threads t
	LEFT JOIN users u ON u.id = t.user_id`

func scanForumThread(scanner interface{ Scan(dest ...any) error }) (*models.ForumThread, error) {
	var t models.ForumThread
	var pinned, locked int
	err := scanner.Scan(
		&t.ID, &t.BoardID, &t.UserID,
		&t.AuthorUsername, &t.AuthorDisplayName,
		&t.Title, &t.Body,
		&t.RepliesCount,
		&pinned, &locked, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	t.Pinned = pinned != 0
	t.Locked = locked != 0
	return &t, nil
}

func (r *ForumRepository) GetThreadsByBoard(boardID int, sortBy string, limit, offset int) ([]*models.ForumThread, error) {
	if r.db == nil || boardID <= 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	orderBy := "t.pinned DESC, GREATEST(t.created_at, COALESCE((SELECT MAX(r.created_at) FROM forum_replies r WHERE r.thread_id = t.id), t.created_at)) DESC, t.id DESC"
	switch sortBy {
	case "replies":
		orderBy = "t.pinned DESC, replies_count DESC, t.created_at DESC, t.id DESC"
	case "oldest":
		orderBy = "t.pinned DESC, t.created_at ASC, t.id ASC"
	}

	rows, err := r.db.Query(
		forumThreadSelect+" WHERE t.board_id = ? ORDER BY "+orderBy+" LIMIT ? OFFSET ?",
		boardID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []*models.ForumThread
	for rows.Next() {
		t, err := scanForumThread(rows)
		if err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

func (r *ForumRepository) GetThreadsByUser(userID, limit, offset int) ([]*models.ForumThread, error) {
	if r.db == nil || userID <= 0 {
		return nil, errors.New("database connection or user unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.Query(forumThreadSelect+" WHERE t.user_id = ? ORDER BY t.created_at DESC, t.id DESC LIMIT ? OFFSET ?", userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []*models.ForumThread
	for rows.Next() {
		thread, err := scanForumThread(rows)
		if err != nil {
			return nil, err
		}
		threads = append(threads, thread)
	}
	return threads, rows.Err()
}

func (r *ForumRepository) CountThreadsByUser(userID int) (int, error) {
	if r.db == nil || userID <= 0 {
		return 0, errors.New("database connection or user unavailable")
	}
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM forum_threads WHERE user_id = ?", userID).Scan(&count)
	return count, err
}

func (r *ForumRepository) GetThreadByID(id int) (*models.ForumThread, error) {
	if r.db == nil || id <= 0 {
		return nil, nil
	}
	t, err := scanForumThread(r.db.QueryRow(forumThreadSelect+" WHERE t.id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

func (r *ForumRepository) CreateThread(boardID, userID int, title, body string, images ...models.ForumImageUpload) (int, error) {
	return r.createForumPost("INSERT INTO forum_threads (board_id, user_id, title, body) VALUES (?, ?, ?, ?)", []any{boardID, userID, title, body}, 0, images)
}

func (r *ForumRepository) CountThreads(boardID int) (int, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM forum_threads WHERE board_id = ?", boardID).Scan(&count)
	return count, err
}

func (r *ForumRepository) GetReplies(threadID, limit, offset int) ([]*models.ForumReply, error) {
	if r.db == nil {
		return nil, errors.New("database connection is offline")
	}
	rows, err := r.db.Query(`SELECT r.id, r.thread_id, r.user_id, COALESCE(u.username, 'Player'), COALESCE(u.displayname, ''), r.body, r.created_at
 FROM forum_replies r LEFT JOIN users u ON u.id = r.user_id WHERE r.thread_id = ? ORDER BY r.created_at ASC, r.id ASC LIMIT ? OFFSET ?`, threadID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	replies := []*models.ForumReply{}
	for rows.Next() {
		var reply models.ForumReply
		if err := rows.Scan(&reply.ID, &reply.ThreadID, &reply.UserID, &reply.AuthorUsername, &reply.AuthorDisplayName, &reply.Body, &reply.CreatedAt); err != nil {
			return nil, err
		}
		replies = append(replies, &reply)
	}
	return replies, rows.Err()
}

func (r *ForumRepository) CreateReply(threadID, userID int, body string, images ...models.ForumImageUpload) (int, error) {
	return r.createForumPost(`INSERT INTO forum_replies (thread_id, user_id, body)
		SELECT id, ?, ? FROM forum_threads WHERE id = ? AND locked = 0`, []any{userID, body, threadID}, threadID, images)
}
