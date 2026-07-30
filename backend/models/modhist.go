package models

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	ActionScrubDescription = "scrub_description"
	ActionScrubUsername    = "scrub_username"
	ActionScrubDisplayName = "scrub_displayname"
	ActionScrubPronouns    = "scrub_pronouns"

	StatusActive    = "active"
	StatusRetracted = "retracted"
	StatusExpired   = "expired"

	ScrubbedDescription = "[ Content Removed ]"
	ScrubbedDisplayName = "[removed]"
)

func ScrubbedUsername(userID int) string {
	return fmt.Sprintf("[ Content Removed [%d] ]", userID)
}

type ModHistory struct {
	ID           int
	UID          int
	AdminID      int
	AdminName    string
	AdminPower   int
	ActionType   string
	Reason       string
	Note         sql.NullString
	Status       string
	CreationDate time.Time
	ExpiresAt    sql.NullTime
}

func (m *ModHistory) ActionLabel() string {
	switch m.ActionType {
	case ActionScrubDescription:
		return "Scrub Description"
	case ActionScrubUsername:
		return "Scrub Username"
	case ActionScrubDisplayName:
		return "Scrub Display Name"
	case ActionScrubPronouns:
		return "Scrub Pronouns"
	default:
		return m.ActionType
	}
}

func (m *ModHistory) StatusLabel() string {
	switch m.Status {
	case StatusActive:
		return "Active"
	case StatusRetracted:
		return "Retracted"
	case StatusExpired:
		return "Expired"
	default:
		return m.Status
	}
}
