package repository

import (
	"database/sql"
	"errors"
	"strings"

	"vertexia-frontend/backend/models"
)

type CommentRepository struct {
	db *sql.DB
}

func NewCommentRepository(db *sql.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

const commentColumns = `c.id, c.target_type, c.target_id, c.user_id, COALESCE(c.parent_id, 0),
	COALESCE(u.username, ''), c.body, c.removed, c.edited, c.creation_date`

func scanComment(row scannable) (*models.Comment, error) {
	var c models.Comment
	if err := row.Scan(&c.ID, &c.TargetType, &c.TargetID, &c.UserID, &c.ParentID,
		&c.Username, &c.Body, &c.Removed, &c.Edited, &c.CreationDate); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CommentRepository) queryComments(query string, args ...any) ([]*models.Comment, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CommentRepository) Create(kind string, targetID, userID, parentID int, body string) (int, error) {
	var parent any
	if parentID > 0 {
		parent = parentID
	}
	res, err := r.db.Exec(`INSERT INTO comments (target_type, target_id, user_id, parent_id, body)
		VALUES (?, ?, ?, ?, ?)`, kind, targetID, userID, parent, body)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func (r *CommentRepository) Get(id int) (*models.Comment, error) {
	c, err := scanComment(r.db.QueryRow(`SELECT `+commentColumns+`
		FROM comments c LEFT JOIN users u ON u.id = c.user_id WHERE c.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func (r *CommentRepository) ListTop(kind string, targetID, beforeID, limit int) ([]*models.Comment, error) {
	query := `SELECT ` + commentColumns + `
		FROM comments c JOIN users u ON u.id = c.user_id
		WHERE c.target_type = ? AND c.target_id = ? AND c.removed = 0 AND c.parent_id IS NULL`
	args := []any{kind, targetID}
	if beforeID > 0 {
		query += ` AND c.id < ?`
		args = append(args, beforeID)
	}
	query += ` ORDER BY c.id DESC LIMIT ?`
	args = append(args, limit)
	return r.queryComments(query, args...)
}

func (r *CommentRepository) ListReplies(parentIDs []int) ([]*models.Comment, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}
	args := make([]any, len(parentIDs))
	for i, id := range parentIDs {
		args[i] = id
	}
	return r.queryComments(`SELECT `+commentColumns+`
		FROM comments c JOIN users u ON u.id = c.user_id
		WHERE c.parent_id IN (?`+strings.Repeat(", ?", len(parentIDs)-1)+`) AND c.removed = 0
		ORDER BY c.id ASC`, args...)
}

func (r *CommentRepository) Remove(id int) error {
	_, err := r.db.Exec(`UPDATE comments SET removed = 1 WHERE id = ?`, id)
	return err
}
