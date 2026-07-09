package models

import (
	"database/sql"
	"time"
)

type Game struct {
	ID          int
	Name        string
	Description sql.NullString
	CreatorID   sql.NullInt64
	Genre       sql.NullString
	Visits      int
	Playing     int
	Server      sql.NullString
	Private     bool
	Locked      bool
	CreatedAt   time.Time
	EditedAt    time.Time
	Tags        sql.NullString
}

func GetPopularGames(db *sql.DB, limit int) ([]*Game, error) {
	if db == nil {
		return nil, nil
	}
	query := "SELECT id, name, description, creatorid, genre, visits, playing, server, private, locked, createdat, editedat, tags FROM games WHERE private = 0 AND locked = 0 ORDER BY visits DESC, id DESC LIMIT ?"
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*Game
	for rows.Next() {
		var g Game
		err := rows.Scan(
			&g.ID,
			&g.Name,
			&g.Description,
			&g.CreatorID,
			&g.Genre,
			&g.Visits,
			&g.Playing,
			&g.Server,
			&g.Private,
			&g.Locked,
			&g.CreatedAt,
			&g.EditedAt,
			&g.Tags,
		)
		if err != nil {
			return nil, err
		}
		games = append(games, &g)
	}
	return games, nil
}