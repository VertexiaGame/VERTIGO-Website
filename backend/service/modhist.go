package service

import (
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

type ModHistoryService struct {
	modHistRepo *repository.ModHistoryRepository
}

func NewModHistoryService(modHistRepo *repository.ModHistoryRepository) *ModHistoryService {
	return &ModHistoryService{modHistRepo: modHistRepo}
}

func (s *ModHistoryService) GetByUserID(userID int) ([]*models.ModHistory, error) {
	if s.modHistRepo == nil {
		return nil, nil
	}
	return s.modHistRepo.GetByUserID(userID)
}