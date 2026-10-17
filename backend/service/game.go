package service

import (
	"errors"
	"strings"

	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

type GameService struct {
	gameRepo *repository.GameRepository
}

func NewGameService(gameRepo *repository.GameRepository) *GameService {
	return &GameService{gameRepo: gameRepo}
}

func (s *GameService) GetGameByID(id int) (*models.Game, error) {
	return s.gameRepo.GetGameByID(id)
}

func (s *GameService) GetRelatedGames(game *models.Game, limit int) ([]*models.Game, error) {
	if game != nil && game.Genre.Valid && game.Genre.String != "" {
		related, err := s.gameRepo.GetGamesByGenre(game.Genre.String, limit+1)
		if err != nil {
			return nil, err
		}
		filtered := related[:0]
		for _, g := range related {
			if g.ID != game.ID {
				filtered = append(filtered, g)
			}
			if len(filtered) >= limit {
				break
			}
		}
		if len(filtered) > 0 {
			return filtered, nil
		}
	}
	return s.gameRepo.GetPopularGames(limit)
}

func (s *GameService) GetUserRating(userID, gameID int) (string, error) {
	return s.gameRepo.GetUserRating(userID, gameID)
}

func (s *GameService) RateGame(userID, gameID int, action string) (string, error) {
	var rating string
	switch action {
	case "like":
		rating = "1"
	case "dislike":
		rating = "-1"
	case "clear":
		rating = ""
	default:
		return "", nil
	}
	current, err := s.gameRepo.GetUserRating(userID, gameID)
	if err != nil {
		return "", err
	}
	if rating == "" || current == rating {
		if err := s.gameRepo.ClearUserRating(userID, gameID); err != nil {
			return "", err
		}
		return "", nil
	}
	if err := s.gameRepo.SetUserRating(userID, gameID, rating); err != nil {
		return "", err
	}
	return rating, nil
}

func validateGameInput(name, description, genre, tags string) (string, string, string, string, error) {
	name, description, normGenre, normTags, _, err := validateGameInputWithWarnings(name, description, genre, tags, "")
	return name, description, normGenre, normTags, err
}

func validateGameInputWithWarnings(name, description, genre, tags, triggerWarnings string) (string, string, string, string, string, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	triggerWarnings = strings.TrimSpace(triggerWarnings)
	if name == "" {
		return "", "", "", "", "", errors.New("game name is required")
	}
	if len(name) > 60 {
		return "", "", "", "", "", errors.New("game name is too long")
	}
	if len(description) > 1000 {
		return "", "", "", "", "", errors.New("game description is too long")
	}
	if len(triggerWarnings) > 500 {
		return "", "", "", "", "", errors.New("game trigger warnings are too long")
	}
	normGenre := ""
	if strings.TrimSpace(genre) != "" {
		if norm, _ := models.NormalizeGameGenre(genre); norm != "all" {
			normGenre = norm
		}
	}
	var tagList []string
	for _, t := range strings.Split(tags, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tagList = append(tagList, t)
		}
	}
	normTags := strings.Join(tagList, ", ")
	if len(normTags) > 255 {
		return "", "", "", "", "", errors.New("game tags are too long")
	}
	return name, description, normGenre, normTags, triggerWarnings, nil
}

func NormalizeMaxPlayers(raw int, fallback int) int {
	if raw <= 0 {
		if fallback >= 2 && fallback <= 100 {
			return fallback
		}
		return 100
	}
	if raw < 2 {
		return 2
	}
	if raw > 100 {
		return 100
	}
	return raw
}

func (s *GameService) CreateGame(uid int, name, description, genre, tags string, private bool) (int, error) {
	return s.CreateGameWithMaxPlayers(uid, name, description, genre, tags, private, 100)
}

func (s *GameService) CreateGameWithMaxPlayers(uid int, name, description, genre, tags string, private bool, maxPlayers int) (int, error) {
	return s.CreateGameWithWarnings(uid, name, description, genre, tags, private, maxPlayers, "")
}

func (s *GameService) CreateGameWithWarnings(uid int, name, description, genre, tags string, private bool, maxPlayers int, triggerWarnings string) (int, error) {
	if s.gameRepo == nil {
		return 0, errors.New("game repository uninitialized")
	}
	if uid <= 0 {
		return 0, errors.New("invalid user")
	}
	name, description, normGenre, normTags, normWarnings, err := validateGameInputWithWarnings(name, description, genre, tags, triggerWarnings)
	if err != nil {
		return 0, err
	}
	return s.gameRepo.CreateWithMaxPlayers(uid, name, description, normGenre, normTags, private, NormalizeMaxPlayers(maxPlayers, 100), normWarnings)
}

func (s *GameService) UpdateGame(id int, name, description, genre, tags string, private bool) error {
	return s.UpdateGameWithMaxPlayers(id, name, description, genre, tags, private, 0)
}

func (s *GameService) UpdateGameWithMaxPlayers(id int, name, description, genre, tags string, private bool, maxPlayers int) error {
	return s.UpdateGameWithWarnings(id, name, description, genre, tags, private, maxPlayers, "", false)
}

func (s *GameService) UpdateGameWithWarnings(id int, name, description, genre, tags string, private bool, maxPlayers int, triggerWarnings string, updateTriggerWarnings bool) error {
	if s.gameRepo == nil {
		return errors.New("game repository uninitialized")
	}
	if id <= 0 {
		return errors.New("invalid game")
	}
	name, description, normGenre, normTags, normWarnings, err := validateGameInputWithWarnings(name, description, genre, tags, triggerWarnings)
	if err != nil {
		return err
	}
	if !updateTriggerWarnings {
		normWarnings = ""
	}
	return s.gameRepo.UpdateWithTriggerWarnings(id, name, description, normGenre, normTags, private, maxPlayers, normWarnings, updateTriggerWarnings)
}

func (s *GameService) SetGameMaxPlayers(id, maxPlayers int) error {
	if s.gameRepo == nil {
		return errors.New("game repository uninitialized")
	}
	if id <= 0 {
		return errors.New("invalid game")
	}
	return s.gameRepo.SetMaxPlayers(id, NormalizeMaxPlayers(maxPlayers, 100))
}

func (s *GameService) SetPlaying(id, playing int) error {
	if s.gameRepo == nil {
		return errors.New("game repository uninitialized")
	}
	if id <= 0 {
		return errors.New("invalid game")
	}
	if playing < 0 {
		playing = 0
	}
	return s.gameRepo.SetPlaying(id, playing)
}

func (s *GameService) AddVisit(id int) error {
	if s.gameRepo == nil {
		return errors.New("game repository uninitialized")
	}
	if id <= 0 {
		return errors.New("invalid game")
	}
	return s.gameRepo.AddVisit(id)
}

func (s *GameService) GetUserGames(uid int) ([]*models.Game, error) {
	if s.gameRepo == nil {
		return nil, nil
	}
	return s.gameRepo.GetByCreator(uid)
}

func (s *GameService) CountUserGames(uid int) (int, error) {
	if s.gameRepo == nil {
		return 0, nil
	}
	return s.gameRepo.CountByCreator(uid)
}

func (s *GameService) DeleteGame(id int) error {
	if s.gameRepo == nil {
		return errors.New("game repository uninitialized")
	}
	return s.gameRepo.Delete(id)
}



func (s *GameService) GetPopularGames(limit int) ([]*models.Game, error) {
	return s.gameRepo.GetPopularGames(limit)
}

func (s *GameService) GetTopPlayingGames(limit int) ([]*models.Game, error) {
	return s.gameRepo.GetTopPlayingGames(limit)
}

func (s *GameService) GetNewestGames(limit int) ([]*models.Game, error) {
	return s.gameRepo.GetNewestGames(limit)
}

func (s *GameService) GetClassicGames(limit int) ([]*models.Game, error) {
	return s.gameRepo.GetClassicGames(limit)
}

func (s *GameService) GetGenreShelves(maxShelves, perShelf int) ([]*models.GenreShelf, error) {
	genres, err := s.gameRepo.GetTopGenres(maxShelves)
	if err != nil || len(genres) == 0 {
		return nil, err
	}
	var shelves []*models.GenreShelf
	for _, genre := range genres {
		games, err := s.gameRepo.GetGamesByGenre(genre, perShelf)
		if err != nil || len(games) == 0 {
			continue
		}
		shelves = append(shelves, &models.GenreShelf{
			Genre: genre,
			Name:  models.GameGenreDisplayName(genre),
			Games: games,
		})
	}
	return shelves, nil
}

func (s *GameService) GetGamesFiltered(filter *models.GameFilter) ([]*models.Game, int, error) {
	return s.gameRepo.GetGamesFiltered(filter)
}
