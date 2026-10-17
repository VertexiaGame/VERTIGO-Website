package models

import (
	"database/sql"
	"fmt"
	"html/template"
	"strings"
	"time"
)

type Game struct {
	ID                  int            `json:"id"`
	Name                string         `json:"name"`
	Description         sql.NullString `json:"-"`
	DescriptionText     string         `json:"description"`
	CreatorID           sql.NullInt64  `json:"-"`
	CreatorName         string         `json:"creator_name"`
	Genre               sql.NullString `json:"-"`
	GenreText           string         `json:"genre"`
	Visits              int            `json:"visits"`
	Playing             int            `json:"playing"`
	MaxPlayers          int            `json:"max_players"`
	Likes               int            `json:"likes"`
	Dislikes            int            `json:"dislikes"`
	LikePct             int            `json:"like_pct"`
	Server              sql.NullString `json:"-"`
	Private             bool           `json:"-"`
	Locked              bool           `json:"-"`
	CreatedAt           time.Time      `json:"-"`
	EditedAt            time.Time      `json:"-"`
	Tags                sql.NullString `json:"-"`
	TriggerWarnings     sql.NullString `json:"-"`
	TriggerWarningsText string         `json:"trigger_warnings"`
}

type GameFilter struct {
	Search  string `json:"search"`
	Genre   string `json:"genre"`
	Creator string `json:"creator"`
	Sort    string `json:"sort"`
	Page    int    `json:"page"`
	Limit   int    `json:"limit"`
}

func NormalizeGameGenre(genre string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(genre)) {
	case "any":
		return "all", "All Genres"
	case "adventure":
		return "adventure", "Adventure"
	case "obby":
		return "obby", "Obby"
	case "tycoon":
		return "tycoon", "Tycoon"
	case "roleplay":
		return "roleplay", "Roleplay"
	case "horror":
		return "horror", "Horror"
	case "simulator", "sim":
		return "simulator", "Simulator"
	case "shooter":
		return "shooter", "Shooter"
	case "racing", "race":
		return "racing", "Racing"
	case "puzzle":
		return "puzzle", "Puzzle"
	case "sandbox":
		return "sandbox", "Sandbox"
	default:
		return "all", "All Genres"
	}
}

func NormalizeGameSort(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "playing":
		return "playing"
	case "newest":
		return "newest"
	case "oldest":
		return "oldest"
	default:
		return "popular"
	}
}

type GenreShelf struct {
	Genre string
	Name  string
	Games []*Game
}

func (g *Game) HasRating() bool {
	return g.LikePct >= 0
}

func (g *Game) LikeColor() template.CSS {
	if g.LikePct < 0 {
		return ""
	}
	return template.CSS(fmt.Sprintf("hsl(%d, 75%%, 38%%)", g.LikePct*120/100))
}

func GameGenreDisplayName(raw string) string {
	raw = strings.TrimSpace(raw)
	if norm, name := NormalizeGameGenre(raw); norm != "all" {
		return name
	}
	if raw == "" {
		return ""
	}
	return strings.ToUpper(raw[:1]) + raw[1:]
}
