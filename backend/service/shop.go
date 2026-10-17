package service

import (
	"errors"
	"strings"

	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

type ShopService struct {
	shopRepo *repository.ShopRepository
}

func NewShopService(shopRepo *repository.ShopRepository) *ShopService {
	return &ShopService{shopRepo: shopRepo}
}

func (s *ShopService) GetShopItems(filter *models.ShopFilter) ([]*models.ShopItem, int, error) {
	if s.shopRepo == nil {
		return []*models.ShopItem{}, 0, nil
	}

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 12
	}

	filter.Search = strings.TrimSpace(filter.Search)
	filter.Creator = strings.TrimSpace(filter.Creator)

	return s.shopRepo.GetItems(filter)
}

func (s *ShopService) CreateItem(uid int, name, description, category string, bucks, bits int, transform string) (int, string, error) {
	if s.shopRepo == nil {
		return 0, "", errors.New("shop repository uninitialized")
	}
	if uid <= 0 {
		return 0, "", errors.New("invalid user")
	}

	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return 0, "", errors.New("item name is required")
	}
	if len(name) > 60 {
		return 0, "", errors.New("item name is too long")
	}
	if len(description) > 500 {
		return 0, "", errors.New("item description is too long")
	}

	normType, _ := models.NormalizeShopCategory(category)
	if normType == "all" {
		return 0, "", errors.New("invalid item category")
	}
	if bucks < 0 || bucks > 1000000 || bits < 0 || bits > 1000000 {
		return 0, "", errors.New("prices must be between 0 and 1000000")
	}

	transform, err := models.ValidateShopTransform(transform)
	if err != nil {
		return 0, "", err
	}

	id, err := s.shopRepo.Create(uid, name, description, normType, bucks, bits, transform)
	if err != nil {
		return 0, "", err
	}
	return id, normType, nil
}

func (s *ShopService) GetUserItems(uid int) ([]*models.ShopItem, error) {
	if s.shopRepo == nil {
		return nil, nil
	}
	return s.shopRepo.GetByCreator(uid)
}

func (s *ShopService) GetApprovalQueue(category string, page, limit int) ([]*models.ShopItem, int, error) {
	if s.shopRepo == nil {
		return nil, 0, errors.New("shop repository uninitialized")
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 12
	}
	return s.shopRepo.GetApprovalQueue(category, limit, (page-1)*limit)
}

func (s *ShopService) ReviewItem(id int, state string) error {
	if s.shopRepo == nil {
		return errors.New("shop repository uninitialized")
	}
	if id <= 0 {
		return errors.New("invalid item ID")
	}
	if state == "rejected" {
		state = models.ShopApprovalDeclined
	}
	if state != models.ShopApprovalApproved && state != models.ShopApprovalDeclined {
		return errors.New("invalid review state")
	}

	item, err := s.shopRepo.GetItemByID(id)
	if err != nil {
		return err
	}
	if item == nil {
		return errors.New("item not found")
	}
	if !item.IsPending() {
		return errors.New("item has already been reviewed")
	}

	return s.shopRepo.UpdateApproval(id, state)
}

func (s *ShopService) DeleteItem(uid, id int) error {
	if s.shopRepo == nil {
		return errors.New("shop repository uninitialized")
	}
	return s.shopRepo.DeletePending(id, uid)
}

func (s *ShopService) GetShopItemByID(id int) (*models.ShopItem, error) {
	if s.shopRepo == nil {
		return nil, errors.New("shop repository uninitialized")
	}
	if id <= 0 {
		return nil, errors.New("invalid item ID")
	}
	return s.shopRepo.GetItemByID(id)
}

func (s *ShopService) GetRelatedItems(item *models.ShopItem, limit int) ([]*models.ShopItem, error) {
	if s.shopRepo == nil || item == nil {
		return []*models.ShopItem{}, nil
	}
	return s.shopRepo.GetRelatedItems(item, limit)
}

func (s *ShopService) HasItem(userID, itemID int) bool {
	if s.shopRepo == nil {
		return false
	}
	return s.shopRepo.HasItem(userID, itemID)
}

func (s *ShopService) IsEquipped(userID, itemID int, itemType string) bool {
	if s.shopRepo == nil {
		return false
	}
	return s.shopRepo.IsEquipped(userID, itemID, itemType)
}

func (s *ShopService) GetSalesCount(itemID int) int {
	if s.shopRepo == nil {
		return 0
	}
	return s.shopRepo.GetSalesCount(itemID)
}

func (s *ShopService) PurchaseItem(userID, itemID int, currency string) (*models.Crate, error) {
	if s.shopRepo == nil {
		return nil, errors.New("shop repository uninitialized")
	}
	if userID <= 0 || itemID <= 0 {
		return nil, errors.New("invalid id")
	}
	currency = strings.ToLower(strings.TrimSpace(currency))
	if currency == "bucks" {
		currency = "vertices"
	}
	if currency == "bits" {
		currency = "tickets"
	}
	if currency != "" && currency != models.BoughtViaFree && currency != models.BoughtViaVertices && currency != models.BoughtViaTickets && currency != "free" {
		return nil, errors.New("invalid currency")
	}
	return s.shopRepo.PurchaseItem(userID, itemID, currency)
}

func (s *ShopService) GrantItem(userID, itemID int, bv string, aid int, price int) error {
	if s.shopRepo == nil {
		return errors.New("shop repository uninitialized")
	}
	return s.shopRepo.GrantItem(userID, itemID, bv, aid, price)
}

func (s *ShopService) GetCrate(userID, itemID int) (bool, error) {
	if s.shopRepo == nil {
		return false, errors.New("shop repository uninitialized")
	}
	return s.shopRepo.GetCrateEntry(userID, itemID)
}