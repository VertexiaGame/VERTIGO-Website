package models

import (
	"database/sql"
	"time"
)

type ModHistory struct {
	ID           int
	UID          int
	AdminID      int
	AdminName    string
	ActionType   string
	Reason       string
	Note         sql.NullString
	Status       string
	CreationDate time.Time
	ExpiresAt    sql.NullTime
}