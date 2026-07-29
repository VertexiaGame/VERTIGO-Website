package repository

import (
	"database/sql"

	"vertexia-frontend/backend/models"
)

type ModHistoryRepository struct {
	db *sql.DB
}

func NewModHistoryRepository(db *sql.DB) *ModHistoryRepository {
	return &ModHistoryRepository{db: db}
}

func (r *ModHistoryRepository) GetByUserID(userID int) ([]*models.ModHistory, error) {
	if r.db == nil {
		return nil, nil
	}

	query := `SELECT m.id, m.uid, m.admin_id, COALESCE(u.username, 'System'), m.action_type, m.reason, m.note, m.status, m.creation_date, m.expires_at
              FROM modhist m
              LEFT JOIN users u ON m.admin_id = u.id
              WHERE m.uid = ?
              ORDER BY m.creation_date DESC, m.id DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.ModHistory
	for rows.Next() {
		var mh models.ModHistory
		if err := rows.Scan(
			&mh.ID, &mh.UID, &mh.AdminID, &mh.AdminName,
			&mh.ActionType, &mh.Reason, &mh.Note, &mh.Status,
			&mh.CreationDate, &mh.ExpiresAt,
		); err != nil {
			return nil, err
		}
		list = append(list, &mh)
	}

	return list, nil
}