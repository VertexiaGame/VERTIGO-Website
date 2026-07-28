package repository

import (
	"database/sql"

	"vertexia-frontend/backend/models"
)

type GameRepository struct {
	db *sql.DB
}

func NewGameRepository(db *sql.DB) *GameRepository {
	return &GameRepository{db: db}
}

func (r *GameRepository) GetPopularGames(limit int) ([]*models.Game, error) {
	if r.db == nil {
		return nil, nil
	}
	query := "SELECT id, name, description, creatorid, genre, visits, playing, server, private, locked, createdat, editedat, tags FROM games WHERE private = 0 AND locked = 0 ORDER BY visits DESC, id DESC LIMIT ?"
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*models.Game
	for rows.Next() {
		var g models.Game
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatorID, &g.Genre, &g.Visits, &g.Playing, &g.Server, &g.Private, &g.Locked, &g.CreatedAt, &g.EditedAt, &g.Tags); err != nil {
			return nil, err
		}
		games = append(games, &g)
	}
	return games, nil
}