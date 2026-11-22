package handlers

import (
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

func populateShopItemTabs(c fiber.Ctx, item *models.ShopItem, viewerID int, data fiber.Map) {
	const pageSize = 24
	approvedSpecial := item.IsSpecial() && item.ApprovalState == models.ShopApprovalApproved
	canResell := item.CanResell()
	data["ShowOwners"] = approvedSpecial
	data["CanResell"] = canResell
	data["MaxResalePrice"] = models.MaxShopResalePrice
	tab := c.Query("tab")
	switch {
	case tab == "comments", tab == "related", tab == "owners" && approvedSpecial, tab == "resellers" && canResell:
	case canResell:
		tab = "resellers"
	default:
		tab = "comments"
	}
	data["ActiveTab"] = tab

	if approvedSpecial && viewerID > 0 {
		copy, err := service.Shop.GetSpecialCopy(item.ID, viewerID)
		if err != nil {
			log.Printf("Shop item %d copy lookup failed: %v", item.ID, err)
			data["SpecialError"] = "Your serial could not be loaded."
		} else {
			data["OwnCopy"] = copy
		}
	}

	page, _ := strconv.Atoi(c.Query("page"))
	page = min(max(page, 1), max(1, (item.OriginalStock-1)/pageSize+1))
	offset := (page - 1) * pageSize
	var err error
	var count int
	switch tab {
	case "owners":
		var owners []models.SpecialOwner
		owners, err = service.Shop.GetSpecialOwners(item.ID, pageSize+1, offset)
		count = len(owners)
		data["SpecialOwners"] = owners[:min(count, pageSize)]
	case "resellers":
		var listings []models.SpecialListing
		listings, err = service.Shop.GetSpecialListings(item.ID, pageSize+1, offset)
		count = len(listings)
		data["SpecialListings"] = listings[:min(count, pageSize)]
	default:
		return
	}
	if err != nil {
		log.Printf("Shop item %d %s lookup failed: %v", item.ID, tab, err)
		data["SpecialError"] = "This tab is temporarily unavailable."
		return
	}
	if page > 1 {
		data["PrevSpecialPage"] = page - 1
	}
	if count > pageSize {
		data["NextSpecialPage"] = page + 1
	}
}

func ShopSpecialPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Please log in"})
	}
	if !csrfValid(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Invalid session token. Refresh the page and try again."})
	}
	if service.Shop == nil || service.User == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": models.ErrSpecialUnavailable.Error()})
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Please log in"})
	}

	itemID, itemErr := strconv.Atoi(c.Params("id"))
	serial, serialErr := strconv.Atoi(c.Params("serial"))
	version, versionErr := strconv.ParseInt(c.FormValue("version"), 10, 64)
	if itemErr != nil || itemID <= 0 || serialErr != nil || serial < 1 || serial > models.MaxShopSpecialStock || versionErr != nil || version < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid serial or listing"})
	}

	switch c.Params("action") {
	case "list":
		price, err := strconv.Atoi(strings.TrimSpace(c.FormValue("price")))
		if err != nil || price < 1 || price > models.MaxShopResalePrice {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Enter a whole price between 1 and " + strconv.Itoa(models.MaxShopResalePrice) + " Vertices"})
		}
		err = service.Shop.SetSpecialListing(user.ID, itemID, serial, price, version)
		if err != nil {
			return shopSpecialError(c, err)
		}
	case "remove":
		if err := service.Shop.SetSpecialListing(user.ID, itemID, serial, 0, version); err != nil {
			return shopSpecialError(c, err)
		}
	case "buy":
		sellerID, err := service.Shop.BuySpecialListing(user.ID, itemID, serial, version)
		if err != nil {
			return shopSpecialError(c, err)
		}
		refreshAvatarRendersAsync(sellerID)
	default:
		return c.SendStatus(fiber.StatusNotFound)
	}
	redirect := "/shop/item/" + strconv.Itoa(itemID) + "?tab=resellers"
	if c.Accepts("html", "json") == "json" {
		return c.JSON(fiber.Map{"success": true, "redirect": redirect})
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To(redirect)
}

func shopSpecialError(c fiber.Ctx, err error) error {
	status := fiber.StatusConflict
	switch {
	case errors.Is(err, models.ErrSpecialListingChanged), errors.Is(err, models.ErrSpecialAlreadyOwned):
	case errors.Is(err, models.ErrSpecialOwnListing), errors.Is(err, models.ErrSpecialNotOwner), errors.Is(err, models.ErrSpecialNotResellable):
		status = fiber.StatusForbidden
	case errors.Is(err, models.ErrSpecialInsufficientFunds):
		status = fiber.StatusPaymentRequired
	case errors.Is(err, models.ErrSpecialUnavailable):
		status = fiber.StatusServiceUnavailable
	default:
		log.Printf("Special item transaction failed: %v", err)
		status = fiber.StatusInternalServerError
		err = errors.New("The transaction could not be completed. Please try again.")
	}
	return c.Status(status).JSON(fiber.Map{"error": err.Error()})
}
