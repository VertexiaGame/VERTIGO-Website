package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

type AvatarService struct {
	avatarRepo *repository.AvatarRepository
	userRepo   *repository.UserRepository
}

func NewAvatarService(avatarRepo *repository.AvatarRepository, userRepo *repository.UserRepository) *AvatarService {
	return &AvatarService{
		avatarRepo: avatarRepo,
		userRepo:   userRepo,
	}
}

func (s *AvatarService) InvalidateRenderCache(userID int) {
	idStr := fmt.Sprintf("%d.png", userID)
	_ = os.Remove(filepath.Join("static", "renders", "avatars", "full", idStr))
	_ = os.Remove(filepath.Join("static", "renders", "avatars", "headshots", idStr))
}

func (s *AvatarService) GetAvatar(userID int) (*models.Avatar, error) {
	if s.avatarRepo == nil {
		return nil, errors.New("avatar repository uninitialized")
	}
	return s.avatarRepo.GetAvatar(userID)
}

func (s *AvatarService) UpdateBodyColor(userID int, bodyPart, color string) error {
	if s.avatarRepo == nil {
		return errors.New("avatar repository uninitialized")
	}
	err := s.avatarRepo.UpdateBodyColor(userID, bodyPart, color)
	if err == nil {
		s.InvalidateRenderCache(userID)
	}
	return err
}

func (s *AvatarService) EquipItem(userID int, itemType string, itemID int) error {
	if s.avatarRepo == nil {
		return errors.New("avatar repository uninitialized")
	}
	err := s.avatarRepo.EquipItem(userID, itemType, itemID)
	if err == nil {
		s.InvalidateRenderCache(userID)
	}
	return err
}

func (s *AvatarService) UnequipItem(userID int, itemType string, itemID int) error {
	if s.avatarRepo == nil {
		return errors.New("avatar repository uninitialized")
	}
	err := s.avatarRepo.UnequipItem(userID, itemType, itemID)
	if err == nil {
		s.InvalidateRenderCache(userID)
	}
	return err
}

func (s *AvatarService) GetInventory(userID int, category, search string) ([]*models.InventoryItem, error) {
	if s.avatarRepo == nil {
		return nil, errors.New("avatar repository uninitialized")
	}
	return s.avatarRepo.GetInventory(userID, category, search)
}

func (s *AvatarService) GetEquippedItems(userID int) ([]*models.InventoryItem, error) {
	if s.avatarRepo == nil {
		return nil, errors.New("avatar repository uninitialized")
	}
	return s.avatarRepo.GetEquippedItems(userID)
}