package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
	"vertexia-frontend/backend/storage"
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

// AvatarFullPath returns the storage key of a user's cached full-body render.
func AvatarFullPath(userID int) string {
	return "uploads/av/" + strconv.Itoa(userID) + ".png"
}

// AvatarHeadPath returns the storage key of a user's cached headshot render.
func AvatarHeadPath(userID int) string {
	return "uploads/av/" + strconv.Itoa(userID) + "_head.png"
}

func avatarDeleteStored(keys ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store := storage.Default()
	for _, key := range keys {
		if err := store.Delete(ctx, key); err != nil {
			log.Printf("storage: delete %s: %v", key, err)
		}
	}
}

func (s *AvatarService) InvalidateRenderCache(userID int) {
	avatarDeleteStored(AvatarFullPath(userID), AvatarHeadPath(userID))
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

func (s *AvatarService) GetOutfits(userID int) ([]*models.Outfit, error) {
	if s.avatarRepo == nil {
		return nil, errors.New("avatar repository uninitialized")
	}
	return s.avatarRepo.GetOutfits(userID)
}

func (s *AvatarService) CreateOutfit(userID int, name string) (*models.Outfit, error) {
	if s.avatarRepo == nil {
		return nil, errors.New("avatar repository uninitialized")
	}
	name = html.EscapeString(strings.TrimSpace(name))
	if name == "" {
		return nil, errors.New("outfit name cannot be empty")
	}
	if len(name) > 30 {
		name = name[:30]
	}
	outfits, _ := s.avatarRepo.GetOutfits(userID)
	if len(outfits) >= 20 {
		return nil, errors.New("maximum limit of 20 saved outfits reached")
	}
	return s.avatarRepo.CreateOutfit(userID, name)
}

func (s *AvatarService) WearOutfit(userID, outfitID int) error {
	if s.avatarRepo == nil {
		return errors.New("avatar repository uninitialized")
	}
	err := s.avatarRepo.WearOutfit(userID, outfitID)
	if err == nil {
		s.InvalidateRenderCache(userID)
	}
	return err
}

func (s *AvatarService) DeleteOutfit(userID, outfitID int) error {
	if s.avatarRepo == nil {
		return errors.New("avatar repository uninitialized")
	}
	err := s.avatarRepo.DeleteOutfit(userID, outfitID)
	if err == nil {
		avatarDeleteStored(fmt.Sprintf("static/renders/outfits/%d.png", outfitID))
	}
	return err
}

func (s *AvatarService) ResetAvatar(userID int) error {
	if s.avatarRepo == nil {
		return errors.New("avatar repository uninitialized")
	}
	err := s.avatarRepo.ResetAvatar(userID)
	if err == nil {
		s.InvalidateRenderCache(userID)
	}
	return err
}
