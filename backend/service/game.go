package service

import (
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

type GameService struct {
	gameRepo *repository.GameRepository
}

func NewGameService(gameRepo *repository.GameRepository) *GameService {
	return &GameService{gameRepo: gameRepo}
}

func (s *GameService) GetPopularGames(limit int) ([]*models.Game, error) {
	return s.gameRepo.GetPopularGames(limit)
}