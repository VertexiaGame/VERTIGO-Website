package repository

import (
	"database/sql"
	"errors"
	"time"

	"vertexia-frontend/backend/models"
)

type BannerRepository struct {
	db *sql.DB
}

func NewBannerRepository(db *sql.DB) *BannerRepository {
	return &BannerRepository{db: db}
}

const bannerSelectFields = "b.id, b.user_id, COALESCE(u.username, ''), b.banner_type, b.color_a, b.color_b, b.angle, b.status, b.created_at"

func scanBanner(s scannable) (*models.Banner, error) {
	var b models.Banner
	err := s.Scan(
		&b.ID, &b.UserID, &b.Username, &b.Type, &b.ColorA, &b.ColorB, &b.Angle, &b.Status, &b.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

func (r *BannerRepository) Submit(userID int, bannerType, colorA, colorB string, angle int) (int, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()
	if _, err := tx.Exec("DELETE FROM profile_banners WHERE user_id = ? AND status = ?", userID, models.BannerStatusPending); err != nil {
		return 0, err
	}
	res, err := tx.Exec(
		"INSERT INTO profile_banners (user_id, banner_type, color_a, color_b, angle, status) VALUES (?, ?, ?, ?, ?, ?)",
		userID, bannerType, colorA, colorB, angle, models.BannerStatusPending,
	)
	if err != nil {
		return 0, err
	}
	lastID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return int(lastID), nil
}

func (r *BannerRepository) Apply(userID int, bannerType, colorA, colorB string, angle int) (int, error) {
	if r.db == nil {
		return 0, errors.New("database connection is offline")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()
	if _, err := tx.Exec("DELETE FROM profile_banners WHERE user_id = ?", userID); err != nil {
		return 0, err
	}
	res, err := tx.Exec(
		"INSERT INTO profile_banners (user_id, banner_type, color_a, color_b, angle, status) VALUES (?, ?, ?, ?, ?, ?)",
		userID, bannerType, colorA, colorB, angle, models.BannerStatusApproved,
	)
	if err != nil {
		return 0, err
	}
	lastID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return int(lastID), nil
}

func (r *BannerRepository) GetApprovedForUser(userID int) (*models.Banner, error) {
	if r.db == nil {
		return nil, nil
	}
	return scanBanner(r.db.QueryRow(
		"SELECT "+bannerSelectFields+" FROM profile_banners b LEFT JOIN users u ON u.id = b.user_id WHERE b.user_id = ? AND b.status = ? ORDER BY b.id DESC LIMIT 1",
		userID, models.BannerStatusApproved,
	))
}

func (r *BannerRepository) GetPendingForUser(userID int) (*models.Banner, error) {
	if r.db == nil {
		return nil, nil
	}
	return scanBanner(r.db.QueryRow(
		"SELECT "+bannerSelectFields+" FROM profile_banners b LEFT JOIN users u ON u.id = b.user_id WHERE b.user_id = ? AND b.status = ? ORDER BY b.id DESC LIMIT 1",
		userID, models.BannerStatusPending,
	))
}

func (r *BannerRepository) GetQueue(limit, offset int) ([]*models.Banner, int, error) {
	if r.db == nil {
		return nil, 0, errors.New("database connection is offline")
	}
	var total int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM profile_banners WHERE status = ?", models.BannerStatusPending).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(
		"SELECT "+bannerSelectFields+" FROM profile_banners b LEFT JOIN users u ON u.id = b.user_id WHERE b.status = ? ORDER BY b.created_at ASC, b.id ASC LIMIT ? OFFSET ?",
		models.BannerStatusPending, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var banners []*models.Banner
	for rows.Next() {
		b, err := scanBanner(rows)
		if err != nil {
			return nil, 0, err
		}
		banners = append(banners, b)
	}
	return banners, total, nil
}

func (r *BannerRepository) Review(id int, status string) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	if status != models.BannerStatusApproved && status != models.BannerStatusDeclined {
		return errors.New("Unknown review action")
	}
	_, err := r.db.Exec("UPDATE profile_banners SET status = ?, reviewed_at = ? WHERE id = ?", status, time.Now(), id)
	return err
}

func (r *BannerRepository) DeleteForUser(userID int) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec("DELETE FROM profile_banners WHERE user_id = ?", userID)
	return err
}

func (r *BannerRepository) DeleteByID(id int) error {
	if r.db == nil {
		return errors.New("database connection is offline")
	}
	_, err := r.db.Exec("DELETE FROM profile_banners WHERE id = ?", id)
	return err
}
