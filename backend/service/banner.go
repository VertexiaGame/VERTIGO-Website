package service

import (
	"errors"
	"fmt"

	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

type BannerService struct {
	bannerRepo *repository.BannerRepository
	cooldown   *CooldownService
}

func NewBannerService(bannerRepo *repository.BannerRepository, cooldown *CooldownService) *BannerService {
	return &BannerService{
		bannerRepo: bannerRepo,
		cooldown:   cooldown,
	}
}

func (s *BannerService) SubmitBanner(userID int, bannerType, colorA, colorB string, angle int) (int, error) {
	if s.bannerRepo == nil {
		return 0, errors.New("banner service unavailable")
	}
	if s.cooldown != nil {
		key := fmt.Sprintf("action:banner:%d", userID)
		if allowed, remaining := s.cooldown.Allow(key); !allowed {
			return 0, fmt.Errorf("Please wait %s before performing this action again", s.cooldown.FormatRemaining(remaining))
		}
	}
	switch bannerType {
	case models.BannerTypeColor, models.BannerTypeGradient, models.BannerTypeImage:
	default:
		return 0, errors.New("Invalid banner type")
	}
	return s.bannerRepo.Submit(userID, bannerType, colorA, colorB, angle)
}

func (s *BannerService) ApplyBanner(userID int, bannerType, colorA, colorB string, angle int) (int, error) {
	if s.bannerRepo == nil {
		return 0, errors.New("banner service unavailable")
	}
	if s.cooldown != nil {
		key := fmt.Sprintf("action:banner:%d", userID)
		if allowed, remaining := s.cooldown.Allow(key); !allowed {
			return 0, fmt.Errorf("Please wait %s before performing this action again", s.cooldown.FormatRemaining(remaining))
		}
	}
	switch bannerType {
	case models.BannerTypeColor, models.BannerTypeGradient:
	default:
		return 0, errors.New("Invalid banner type")
	}
	return s.bannerRepo.Apply(userID, bannerType, colorA, colorB, angle)
}

func (s *BannerService) GetApprovedBanner(userID int) (*models.Banner, error) {
	if s.bannerRepo == nil {
		return nil, nil
	}
	return s.bannerRepo.GetApprovedForUser(userID)
}

func (s *BannerService) GetPendingBanner(userID int) (*models.Banner, error) {
	if s.bannerRepo == nil {
		return nil, nil
	}
	return s.bannerRepo.GetPendingForUser(userID)
}

func (s *BannerService) GetQueue(page, limit int) ([]*models.Banner, int, error) {
	if s.bannerRepo == nil {
		return []*models.Banner{}, 0, nil
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	return s.bannerRepo.GetQueue(limit, (page-1)*limit)
}

func (s *BannerService) ReviewBanner(id int, action string) (string, error) {
	if s.bannerRepo == nil {
		return "", errors.New("banner service unavailable")
	}
	var status string
	switch action {
	case "approve":
		status = models.BannerStatusApproved
	case "reject":
		status = models.BannerStatusDeclined
	default:
		return "", errors.New("Unknown review action")
	}
	if err := s.bannerRepo.Review(id, status); err != nil {
		return "", err
	}
	return status, nil
}

func (s *BannerService) RemoveBanners(userID int) error {
	if s.bannerRepo == nil {
		return errors.New("banner service unavailable")
	}
	return s.bannerRepo.DeleteForUser(userID)
}

func (s *BannerService) DeleteBanner(id int) error {
	if s.bannerRepo == nil {
		return errors.New("banner service unavailable")
	}
	return s.bannerRepo.DeleteByID(id)
}
