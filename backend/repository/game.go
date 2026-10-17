package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"vertexia-frontend/backend/models"
)

type GameRepository struct {
	db *sql.DB
}

func NewGameRepository(db *sql.DB) *GameRepository {
	return &GameRepository{db: db}
}

const gameSelectColumns = "g.id, g.name, g.description, g.creatorid, COALESCE(u.username, 'VERTEXIA'), g.genre, g.visits, g.playing, g.max_players, g.trigger_warnings, g.server, g.private, g.locked, g.createdat, g.editedat, g.tags, (SELECT COUNT(*) FROM game_ratings r WHERE r.game_id = g.id AND r.rating = '1'), (SELECT COUNT(*) FROM game_ratings r WHERE r.game_id = g.id AND r.rating = '-1')"

const gameSelectColumnsLegacy = "g.id, g.name, g.description, g.creatorid, COALESCE(u.username, 'VERTEXIA'), g.genre, g.visits, g.playing, g.server, g.private, g.locked, g.createdat, g.editedat, g.tags, (SELECT COUNT(*) FROM game_ratings r WHERE r.game_id = g.id AND r.rating = '1'), (SELECT COUNT(*) FROM game_ratings r WHERE r.game_id = g.id AND r.rating = '-1')"

func finalizeScannedGame(g *models.Game) {
	if g.MaxPlayers <= 0 {
		g.MaxPlayers = 100
	}
	if g.Description.Valid {
		g.DescriptionText = g.Description.String
	}
	if g.Genre.Valid {
		g.GenreText = g.Genre.String
	}
	if g.TriggerWarnings.Valid {
		g.TriggerWarningsText = g.TriggerWarnings.String
	}
	if total := g.Likes + g.Dislikes; total > 0 {
		g.LikePct = (g.Likes*100 + total/2) / total
	} else {
		g.LikePct = -1
	}
}

func scanGame(rows *sql.Rows) (*models.Game, error) {
	var g models.Game
	if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatorID, &g.CreatorName, &g.Genre, &g.Visits, &g.Playing, &g.MaxPlayers, &g.TriggerWarnings, &g.Server, &g.Private, &g.Locked, &g.CreatedAt, &g.EditedAt, &g.Tags, &g.Likes, &g.Dislikes); err != nil {
		return nil, err
	}
	finalizeScannedGame(&g)
	return &g, nil
}

func scanGameWithoutTriggerWarnings(rows *sql.Rows) (*models.Game, error) {
	var g models.Game
	if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatorID, &g.CreatorName, &g.Genre, &g.Visits, &g.Playing, &g.MaxPlayers, &g.Server, &g.Private, &g.Locked, &g.CreatedAt, &g.EditedAt, &g.Tags, &g.Likes, &g.Dislikes); err != nil {
		return nil, err
	}
	finalizeScannedGame(&g)
	return &g, nil
}

func scanGameLegacy(rows *sql.Rows) (*models.Game, error) {
	var g models.Game
	if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatorID, &g.CreatorName, &g.Genre, &g.Visits, &g.Playing, &g.Server, &g.Private, &g.Locked, &g.CreatedAt, &g.EditedAt, &g.Tags, &g.Likes, &g.Dislikes); err != nil {
		return nil, err
	}
	g.MaxPlayers = 100
	finalizeScannedGame(&g)
	return &g, nil
}

func scanGameLegacyWithTriggerWarnings(rows *sql.Rows) (*models.Game, error) {
	var g models.Game
	if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatorID, &g.CreatorName, &g.Genre, &g.Visits, &g.Playing, &g.TriggerWarnings, &g.Server, &g.Private, &g.Locked, &g.CreatedAt, &g.EditedAt, &g.Tags, &g.Likes, &g.Dislikes); err != nil {
		return nil, err
	}
	g.MaxPlayers = 100
	finalizeScannedGame(&g)
	return &g, nil
}

func isMissingMaxPlayersColumn(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "max_players")
}

func isMissingTriggerWarningsColumn(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "trigger_warnings")
}



func (r *GameRepository) queryGames(query string, args ...any) ([]*models.Game, error) {
	if r.db == nil {
		return nil, nil
	}
	rows, err := r.db.Query(query, args...)
	if err != nil {
		if isMissingTriggerWarningsColumn(err) && strings.Contains(query, "g.trigger_warnings") {
			return r.queryGames(strings.Replace(query, "g.trigger_warnings, ", "", 1), args...)
		}
		if isMissingMaxPlayersColumn(err) && strings.Contains(query, "g.max_players") {
			return r.queryGames(strings.Replace(query, "g.max_players, ", "", 1), args...)
		}
		return nil, err
	}
	defer rows.Close()

	hasMaxPlayers := strings.Contains(query, "g.max_players")
	hasTriggerWarnings := strings.Contains(query, "g.trigger_warnings")
	var games []*models.Game
	for rows.Next() {
		var g *models.Game
		switch {
		case hasMaxPlayers && hasTriggerWarnings:
			g, err = scanGame(rows)
		case hasMaxPlayers:
			g, err = scanGameWithoutTriggerWarnings(rows)
		case hasTriggerWarnings:
			g, err = scanGameLegacyWithTriggerWarnings(rows)
		default:
			g, err = scanGameLegacy(rows)
		}
		if err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, nil
}

func (r *GameRepository) GetGameByID(id int) (*models.Game, error) {
	games, err := r.queryGames("SELECT "+gameSelectColumns+" FROM games g LEFT JOIN users u ON g.creatorid = u.id WHERE g.id = ? LIMIT 1", id)
	if err != nil || len(games) == 0 {
		return nil, err
	}
	return games[0], nil
}

func (r *GameRepository) GetUserRating(userID, gameID int) (string, error) {
	if r.db == nil || userID <= 0 || gameID <= 0 {
		return "", nil
	}
	var rating string
	err := r.db.QueryRow("SELECT rating FROM game_ratings WHERE user_id = ? AND game_id = ?", userID, gameID).Scan(&rating)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return rating, nil
}

func (r *GameRepository) SetUserRating(userID, gameID int, rating string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec("INSERT INTO game_ratings (user_id, game_id, rating) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE rating = VALUES(rating)", userID, gameID, rating)
	return err
}

func (r *GameRepository) ClearUserRating(userID, gameID int) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec("DELETE FROM game_ratings WHERE user_id = ? AND game_id = ?", userID, gameID)
	return err
}

func (r *GameRepository) Create(creatorID int, name, description, genre, tags string, private bool) (int, error) {
	return r.CreateWithMaxPlayers(creatorID, name, description, genre, tags, private, 100, "")
}

func (r *GameRepository) CreateWithMaxPlayers(creatorID int, name, description, genre, tags string, private bool, maxPlayers int, triggerWarnings string) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database connection is offline")
	}
	if maxPlayers < 2 {
		maxPlayers = 2
	}
	if maxPlayers > 100 {
		maxPlayers = 100
	}
	var genreArg any
	if genre != "" {
		genreArg = genre
	}
	var tagsArg any
	if tags != "" {
		tagsArg = tags
	}
	var descArg any
	if description != "" {
		descArg = description
	}
	var triggerArg any
	if triggerWarnings != "" {
		triggerArg = triggerWarnings
	}
	res, err := r.db.Exec("INSERT INTO games (name, description, creatorid, genre, private, tags, max_players, trigger_warnings) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", name, descArg, creatorID, genreArg, private, tagsArg, maxPlayers, triggerArg)
	if err != nil {
		if isMissingTriggerWarningsColumn(err) {
			res, err = r.db.Exec("INSERT INTO games (name, description, creatorid, genre, private, tags, max_players) VALUES (?, ?, ?, ?, ?, ?, ?)", name, descArg, creatorID, genreArg, private, tagsArg, maxPlayers)
			if err != nil && isMissingMaxPlayersColumn(err) {
				res, err = r.db.Exec("INSERT INTO games (name, description, creatorid, genre, private, tags) VALUES (?, ?, ?, ?, ?, ?)", name, descArg, creatorID, genreArg, private, tagsArg)
			}
		} else if isMissingMaxPlayersColumn(err) {
			res, err = r.db.Exec("INSERT INTO games (name, description, creatorid, genre, private, tags, trigger_warnings) VALUES (?, ?, ?, ?, ?, ?, ?)", name, descArg, creatorID, genreArg, private, tagsArg, triggerArg)
			if err != nil && isMissingTriggerWarningsColumn(err) {
				res, err = r.db.Exec("INSERT INTO games (name, description, creatorid, genre, private, tags) VALUES (?, ?, ?, ?, ?, ?)", name, descArg, creatorID, genreArg, private, tagsArg)
			}
		}
		if err != nil {
			return 0, err
		}
	} 
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (r *GameRepository) SetMaxPlayers(id, maxPlayers int) error {
	if r.db == nil {
		return fmt.Errorf("database connection is offline")
	}
	if maxPlayers < 2 {
		maxPlayers = 2
	}
	if maxPlayers > 100 {
		maxPlayers = 100
	}
	_, err := r.db.Exec("UPDATE games SET max_players = ? WHERE id = ?", maxPlayers, id)
	return err
}

func (r *GameRepository) SetPlaying(id, playing int) error {
	if r.db == nil {
		return nil
	}
	if playing < 0 {
		playing = 0
	}
	_, err := r.db.Exec("UPDATE games SET playing = ? WHERE id = ?", playing, id)
	return err
}

func (r *GameRepository) AddVisit(id int) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec("UPDATE games SET visits = visits + 1 WHERE id = ?", id)
	return err
}

func (r *GameRepository) Delete(id int) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec("DELETE FROM games WHERE id = ?", id)
	return err
}



func (r *GameRepository) GetByCreator(uid int) ([]*models.Game, error) {
	return r.queryGames("SELECT "+gameSelectColumns+" FROM games g LEFT JOIN users u ON g.creatorid = u.id WHERE g.creatorid = ? ORDER BY g.createdat DESC, g.id DESC", uid)
}

func (r *GameRepository) CountByCreator(uid int) (int, error) {
	if r.db == nil {
		return 0, nil
	}
	var n int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM games WHERE creatorid = ?", uid).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *GameRepository) Update(id int, name, description, genre, tags string, private bool) error {
	return r.UpdateWithMaxPlayers(id, name, description, genre, tags, private, 0)
}

func (r *GameRepository) UpdateWithMaxPlayers(id int, name, description, genre, tags string, private bool, maxPlayers int) error {
	return r.UpdateWithMaxPlayersAndWarnings(id, name, description, genre, tags, private, maxPlayers, "", false)
}

func (r *GameRepository) UpdateWithMaxPlayersAndWarnings(id int, name, description, genre, tags string, private bool, maxPlayers int, triggerWarnings string, updateTriggerWarnings bool) error {
	return r.UpdateWithTriggerWarnings(id, name, description, genre, tags, private, maxPlayers, triggerWarnings, updateTriggerWarnings)
}

func (r *GameRepository) UpdateWithTriggerWarnings(id int, name, description, genre, tags string, private bool, maxPlayers int, triggerWarnings string, updateTriggerWarnings bool) error {
	if r.db == nil {
		return fmt.Errorf("database connection is offline")
	}
	var genreArg any
	if genre != "" {
		genreArg = genre
	}
	var tagsArg any
	if tags != "" {
		tagsArg = tags
	}
	var descArg any
	if description != "" {
		descArg = description
	}
	var triggerArg any
	if triggerWarnings != "" {
		triggerArg = triggerWarnings
	}
	if maxPlayers < 2 && maxPlayers != 0 {
		maxPlayers = 2
	}
	if maxPlayers > 100 {
		maxPlayers = 100
	}
	execUpdate := func(includeMax, includeTrigger bool) error {
		sets := "name = ?, description = ?, genre = ?, tags = ?, private = ?"
		args := []any{name, descArg, genreArg, tagsArg, private}
		if includeMax {
			sets += ", max_players = ?"
			args = append(args, maxPlayers)
		}
		if includeTrigger {
			sets += ", trigger_warnings = ?"
			args = append(args, triggerArg)
		}
		args = append(args, id)
		_, err := r.db.Exec("UPDATE games SET "+sets+" WHERE id = ?", args...)
		return err
	}
	includeMax := maxPlayers > 0
	includeTrigger := updateTriggerWarnings
	err := execUpdate(includeMax, includeTrigger)
	if err != nil && includeTrigger && isMissingTriggerWarningsColumn(err) {
		includeTrigger = false
		err = execUpdate(includeMax, false)
	}
	if err != nil && includeMax && isMissingMaxPlayersColumn(err) {
		includeMax = false
		err = execUpdate(false, includeTrigger)
		if err != nil && includeTrigger && isMissingTriggerWarningsColumn(err) {
			err = execUpdate(false, false)
		}
	}
	return err
}

func (r *GameRepository) GetPopularGames(limit int) ([]*models.Game, error) {
	return r.queryGames("SELECT "+gameSelectColumns+" FROM games g LEFT JOIN users u ON g.creatorid = u.id WHERE g.private = 0 AND g.locked = 0 ORDER BY g.visits DESC, g.id DESC LIMIT ?", limit)
}

func (r *GameRepository) GetTopPlayingGames(limit int) ([]*models.Game, error) {
	return r.queryGames("SELECT "+gameSelectColumns+" FROM games g LEFT JOIN users u ON g.creatorid = u.id WHERE g.private = 0 AND g.locked = 0 ORDER BY g.playing DESC, g.visits DESC, g.id DESC LIMIT ?", limit)
}

func (r *GameRepository) GetNewestGames(limit int) ([]*models.Game, error) {
	return r.queryGames("SELECT "+gameSelectColumns+" FROM games g LEFT JOIN users u ON g.creatorid = u.id WHERE g.private = 0 AND g.locked = 0 ORDER BY g.createdat DESC, g.id DESC LIMIT ?", limit)
}

func (r *GameRepository) GetClassicGames(limit int) ([]*models.Game, error) {
	return r.queryGames("SELECT "+gameSelectColumns+" FROM games g LEFT JOIN users u ON g.creatorid = u.id WHERE g.private = 0 AND g.locked = 0 ORDER BY g.createdat ASC, g.id ASC LIMIT ?", limit)
}

func (r *GameRepository) GetTopGenres(limit int) ([]string, error) {
	if r.db == nil {
		return nil, nil
	}
	rows, err := r.db.Query("SELECT LOWER(g.genre) FROM games g WHERE g.private = 0 AND g.locked = 0 AND g.genre IS NOT NULL AND g.genre <> '' GROUP BY LOWER(g.genre) ORDER BY COUNT(*) DESC, LOWER(g.genre) ASC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var genres []string
	for rows.Next() {
		var genre string
		if err := rows.Scan(&genre); err != nil {
			return nil, err
		}
		genres = append(genres, genre)
	}
	return genres, nil
}

func (r *GameRepository) GetGamesByGenre(genre string, limit int) ([]*models.Game, error) {
	return r.queryGames("SELECT "+gameSelectColumns+" FROM games g LEFT JOIN users u ON g.creatorid = u.id WHERE g.private = 0 AND g.locked = 0 AND LOWER(g.genre) = LOWER(?) ORDER BY g.visits DESC, g.id DESC LIMIT ?", genre, limit)
}

func (r *GameRepository) GetGamesFiltered(filter *models.GameFilter) ([]*models.Game, int, error) {
	if r.db == nil {
		return []*models.Game{}, 0, nil
	}

	var where []string
	var args []any
	where = append(where, "g.private = 0 AND g.locked = 0")

	if filter.Search != "" {
		where = append(where, "(g.name LIKE ? OR g.description LIKE ? OR g.tags LIKE ?)")
		like := "%" + filter.Search + "%"
		args = append(args, like, like, like)
	}
	if filter.Genre != "" && filter.Genre != "all" {
		where = append(where, "g.genre = ?")
		args = append(args, filter.Genre)
	}
	if filter.Creator != "" {
		where = append(where, "u.username LIKE ?")
		args = append(args, "%"+filter.Creator+"%")
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM games g LEFT JOIN users u ON g.creatorid = u.id %s", whereSQL)
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := "g.visits DESC, g.id DESC"
	switch filter.Sort {
	case "playing":
		order = "g.playing DESC, g.visits DESC, g.id DESC"
	case "newest":
		order = "g.createdat DESC, g.id DESC"
	case "oldest":
		order = "g.createdat ASC, g.id ASC"
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	dataQuery := fmt.Sprintf("SELECT %s FROM games g LEFT JOIN users u ON g.creatorid = u.id %s ORDER BY %s LIMIT ? OFFSET ?", gameSelectColumns, whereSQL, order)
	args = append(args, limit, offset)

	games, err := r.queryGames(dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	if games == nil {
		games = []*models.Game{}
	}
	return games, total, nil
}
