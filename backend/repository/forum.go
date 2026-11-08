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
	repo := &ForumRepository{db: db}
	repo.ensureTables()
	return repo
}

func (r *ForumRepository) ensureTables() {
	if r.db == nil {
		return
	}
	_, _ = r.db.Exec(`CREATE TABLE IF NOT EXISTS forum_threads (
		id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
		board_id   INT UNSIGNED NOT NULL,
		user_id    INT          NOT NULL,
		title      VARCHAR(120) NOT NULL,
		body       TEXT         NOT NULL,
		pinned     TINYINT(1)   NOT NULL DEFAULT 0,
		locked     TINYINT(1)   NOT NULL DEFAULT 0,
		created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		KEY idx_forum_threads_board (board_id, pinned, created_at),
		KEY idx_forum_threads_user (user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`)

	_, _ = r.db.Exec(`CREATE TABLE IF NOT EXISTS forum_replies (
		id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
		thread_id  INT UNSIGNED NOT NULL,
		user_id    INT          NOT NULL,
		body       TEXT         NOT NULL,
		created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		KEY idx_forum_replies_thread (thread_id, created_at),
		KEY idx_forum_replies_user (user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`)
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

	orderBy := "t.pinned DESC, t.created_at DESC, t.id DESC"
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

func (r *ForumRepository) CreateThread(boardID, userID int, title, body string) (int, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	res, err := r.db.Exec(
		"INSERT INTO forum_threads (board_id, user_id, title, body) VALUES (?, ?, ?, ?)",
		boardID, userID, title, body,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}
