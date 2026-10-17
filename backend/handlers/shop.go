package handlers

import (
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fogleman/fauxgl"
	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

const (
	shopItemUploadDir         = "./assets/uploads/shop"
	shopItemMaxImageBytes     = 8 << 20
	shopItemMaxModelBytes     = 25 << 20
	shopItemMaxModelTriangles = 8192
	shopItemMaxPrice          = 1000000
)

var shopItemImageExtensions = map[string]bool{".png": true, ".jpg": true, ".jpeg": true}

func extractShopFilter(c fiber.Ctx) *models.ShopFilter {
	search := strings.TrimSpace(c.Query("q"))
	if search == "" {
		search = strings.TrimSpace(c.Query("search"))
	}

	category := strings.TrimSpace(c.Query("category"))
	if category == "" {
		category = strings.TrimSpace(c.Query("cat"))
	}
	if category == "" {
		category = "all"
	}

	currency := strings.TrimSpace(c.Query("currency"))
	minPrice, _ := strconv.Atoi(c.Query("min_price", "0"))
	maxPrice, _ := strconv.Atoi(c.Query("max_price", "0"))
	creator := strings.TrimSpace(c.Query("creator"))
	sort := strings.TrimSpace(c.Query("sort", "newest"))
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	return &models.ShopFilter{
		Search:   search,
		Category: category,
		Currency: currency,
		MinPrice: minPrice,
		MaxPrice: maxPrice,
		Creator:  creator,
		Sort:     sort,
		Page:     page,
		Limit:    limit,
	}
}

func ShopGet(c fiber.Ctx) error {
	filter := extractShopFilter(c)

	var items []*models.ShopItem
	totalItems := 0

	if service.Shop != nil {
		var err error
		items, totalItems, err = service.Shop.GetShopItems(filter)
		if err != nil {
			log.Printf("ShopGet error: %v", err)
			items = []*models.ShopItem{}
		}
	}

	totalPages := 1
	if totalItems > 0 && filter.Limit > 0 {
		totalPages = (totalItems + filter.Limit - 1) / filter.Limit
	}

	normCat, normCatName := models.NormalizeShopCategory(filter.Category)

	return Render(c, "pages/shop", fiber.Map{
		"Title":         "Shop - VERTEXIA",
		"Items":         items,
		"TotalItems":    totalItems,
		"TotalPages":    totalPages,
		"CurrentPage":   filter.Page,
		"Filter":        filter,
		"ActiveCat":     normCat,
		"ActiveCatName": normCatName,
	}, "layouts/main")
}

func ShopItemsAPI(c fiber.Ctx) error {
	filter := extractShopFilter(c)

	if service.Shop == nil {
		return c.JSON(fiber.Map{
			"items":       []*models.ShopItem{},
			"total":       0,
			"page":        filter.Page,
			"limit":       filter.Limit,
			"total_pages": 1,
		})
	}

	items, total, err := service.Shop.GetShopItems(filter)
	if err != nil {
		log.Printf("ShopItemsAPI error: %v", err)
		return c.JSON(fiber.Map{
			"items":       []*models.ShopItem{},
			"total":       0,
			"page":        filter.Page,
			"limit":       filter.Limit,
			"total_pages": 1,
		})
	}

	totalPages := 1
	if total > 0 && filter.Limit > 0 {
		totalPages = (total + filter.Limit - 1) / filter.Limit
	}

	return c.JSON(fiber.Map{
		"items":       items,
		"total":       total,
		"page":        filter.Page,
		"limit":       filter.Limit,
		"total_pages": totalPages,
	})
}

func ShopItemDetailAPI(c fiber.Ctx) error {
	itemID, err := strconv.Atoi(c.Params("id"))
	if err != nil || itemID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid item ID"})
	}

	if service.Shop == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Shop service uninitialized"})
	}

	item, err := service.Shop.GetShopItemByID(itemID)
	if err != nil || item == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
	}

	if item.ApprovalState != models.ShopApprovalApproved {
		masked := *item
		masked.Name = "[Content Pending]"
		masked.Description = "[Content Pending]"
		return c.JSON(&masked)
	}

	return c.JSON(item)
}

func ShopItemPage(c fiber.Ctx) error {
	itemID, err := strconv.Atoi(c.Params("id"))
	if err != nil || itemID <= 0 {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	if service.Shop == nil {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	item, err := service.Shop.GetShopItemByID(itemID)
	if err != nil || item == nil {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	var related []*models.ShopItem
	if service.Shop != nil {
		related, _ = service.Shop.GetRelatedItems(item, 6)
	}
	username := GetActiveUser(c)
	isLoggedIn := username != ""
	var currentUserID int
	var isOwned bool
	var isEquipped bool
	if isLoggedIn && service.User != nil && service.Shop != nil {
		u, _ := service.User.GetUserByUsername(username)
		if u != nil {
			currentUserID = u.ID
			isOwned = service.Shop.HasItem(u.ID, item.ID)
			isEquipped = service.Shop.IsEquipped(u.ID, item.ID, item.Type)
		}
	}
	isOnSale := item.OnSale != "false" && item.OffSale != "true"
	isFree := item.Bucks == 0 && item.Bits == 0
	creatorID := item.CID
	sales := 0
	if service.Shop != nil {
		sales = service.Shop.GetSalesCount(item.ID)
	}
	canViewFull := item.ApprovalState == models.ShopApprovalApproved ||
		(isLoggedIn && currentUserID != 0 && currentUserID == creatorID)
	pageTitle := item.Name + " - VERTEXIA"
	if !canViewFull {
		pageTitle = "[Content Pending] - VERTEXIA"
	}
	return Render(c, "pages/item", fiber.Map{
		"Title":         pageTitle,
		"Item":          item,
		"Related":       related,
		"CanViewFull":   canViewFull,
		"IsLoggedIn":    isLoggedIn,
		"CurrentUserID": currentUserID,
		"IsOwned":       isOwned,
		"IsEquipped":    isEquipped,
		"IsOnSale":      isOnSale,
		"IsFree":        isFree,
		"CreatorID":     creatorID,
		"Sales":         sales,
	}, "layouts/main")
}

func ShopItemRedirect(c fiber.Ctx) error {
	itemID, err := strconv.Atoi(c.Params("id"))
	if err != nil || itemID <= 0 {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	return c.Redirect().To("/shop/item/" + strconv.Itoa(itemID))
}

func ShopPurchaseAPI(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: please log in"})
	}
	itemID, err := strconv.Atoi(c.Params("id"))
	if err != nil || itemID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid item ID"})
	}
	if service.User == nil || service.Shop == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Service unavailable"})
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}
	currency := strings.TrimSpace(c.FormValue("currency"))
	if currency == "" {
		currency = strings.TrimSpace(c.Query("currency"))
	}
	if currency == "" {
		var body map[string]string
		if err := c.Bind().JSON(&body); err == nil {
			if v, ok := body["currency"]; ok {
				currency = strings.TrimSpace(v)
			}
		}
	}
	currency = strings.ToLower(currency)
	if currency == "bucks" {
		currency = models.BoughtViaVertices
	}
	if currency == "bits" {
		currency = models.BoughtViaTickets
	}

	crate, err := service.Shop.PurchaseItem(user.ID, itemID, currency)
	if err != nil {
		msg := err.Error()
		lower := strings.ToLower(msg)
		status := fiber.StatusBadRequest
		switch {
		case strings.Contains(lower, "already owned"):
			status = fiber.StatusConflict
		case strings.Contains(lower, "insufficient"):
			status = fiber.StatusPaymentRequired
		case strings.Contains(lower, "offsale"):
			status = fiber.StatusForbidden
		case strings.Contains(lower, "not found"):
			status = fiber.StatusNotFound
		case strings.Contains(lower, "choose currency"):
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(fiber.Map{"error": msg})
	}

	updated, _ := service.User.GetUserByUsername(username)
	bucks := 0
	bits := 0
	if updated != nil {
		bucks = updated.Bucks
		bits = updated.Bits
	}
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Purchase successful",
		"crate":   crate,
		"bucks":   bucks,
		"bits":    bits,
	})
}

func ShopPurchasePost(c fiber.Ctx) error {
	return ShopPurchaseAPI(c)
}

func ItemsPage(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Redirect().To("/login")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Redirect().To("/login")
	}

	var items []*models.ShopItem
	if service.Shop != nil {
		items, _ = service.Shop.GetUserItems(user.ID)
	}
	if items == nil {
		items = []*models.ShopItem{}
	}

	return Render(c, "pages/items", fiber.Map{
		"Title":    "My Items - VERTEXIA",
		"Username": username,
		"Items":    items,
	}, "layouts/main")
}

func saveShopItemImage(src io.Reader, dstPath string) error {
	img, _, err := image.Decode(src)
	if err != nil {
		return errors.New("invalid image file")
	}
	dst, err := os.Create(dstPath)
	if err != nil {
		return errors.New("could not save upload")
	}
	defer dst.Close()
	if err := png.Encode(dst, img); err != nil {
		return errors.New("could not save upload")
	}
	return nil
}

func ShopItemCreatePost(c fiber.Ctx) error {
	jsonResponse := wantsAssetJSON(c)

	username := GetActiveUser(c)
	if username == "" {
		if jsonResponse {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Your session expired. Please log in again, then retry."})
		}
		return c.Redirect().To("/login")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if jsonResponse {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Your session expired. Please log in again, then retry."})
		}
		return c.Redirect().To("/login")
	}

	rawCategory := c.FormValue("category")
	normType, _ := models.NormalizeShopCategory(rawCategory)

	redirectError := func(msg string) error {
		return c.Redirect().To("/create?type=shop&cat=" + rawCategory + "&error=" + url.QueryEscape(msg))
	}

	fail := func(msg string, status int) error {
		if jsonResponse {
			return c.Status(status).JSON(fiber.Map{"error": msg})
		}
		return redirectError(msg)
	}

	succeed := func(itemID int) error {
		if jsonResponse {
			return c.JSON(fiber.Map{"success": true, "id": itemID})
		}
		return c.Redirect().To("/create/items")
	}

	if normType == "all" {
		return fail("Invalid item category", fiber.StatusBadRequest)
	}

	accountAge := time.Since(user.CreationDate)
	if normType == "shirts" || normType == "tshirts" || normType == "pants" {
		if accountAge < 24*time.Hour && !user.HasAdminAccess() {
			return fail("Your account must be at least 1 day old to create clothing", fiber.StatusForbidden)
		}
	} else if accountAge < 3*24*time.Hour && !user.HasAdminAccess() {
		return fail("Your account must be at least 3 days old to create hats, tools or faces", fiber.StatusForbidden)
	}

	parsePrice := func(field string) (int, error) {
		raw := strings.TrimSpace(c.FormValue(field))
		if raw == "" {
			return 0, nil
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > shopItemMaxPrice {
			return 0, errors.New("prices must be whole numbers between 0 and 1000000")
		}
		return value, nil
	}

	bucks, err := parsePrice("bucks")
	if err != nil {
		return fail(err.Error(), fiber.StatusBadRequest)
	}
	bits, err := parsePrice("bits")
	if err != nil {
		return fail(err.Error(), fiber.StatusBadRequest)
	}

	if service.Shop == nil {
		return fail("Shop service unavailable", fiber.StatusInternalServerError)
	}

	itemID, itemType, err := service.Shop.CreateItem(user.ID, c.FormValue("name"), c.FormValue("description"), normType, bucks, bits, c.FormValue("transform"))
	if err != nil {
		return fail(err.Error(), fiber.StatusBadRequest)
	}

	var savedPaths []string
	cleanup := func() {
		for _, path := range savedPaths {
			os.Remove(path)
		}
		_ = service.Shop.DeleteItem(user.ID, itemID)
	}

	dir := filepath.Join(shopItemUploadDir, filepath.Base(itemType))
	if err := os.MkdirAll(dir, 0755); err != nil {
		cleanup()
		return fail("Could not save upload", fiber.StatusInternalServerError)
	}

	imagePath := filepath.Join(dir, strconv.Itoa(itemID)+".png")
	saveImageField := func(field, kind string) error {
		header, err := c.FormFile(field)
		if err != nil {
			return errors.New("missing " + kind)
		}
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if !shopItemImageExtensions[ext] {
			return errors.New("invalid " + kind + " file type")
		}
		if header.Size <= 0 {
			return errors.New("empty " + kind + " file")
		}
		if header.Size > shopItemMaxImageBytes {
			return errors.New(kind + " file is too large")
		}
		src, err := header.Open()
		if err != nil {
			return errors.New("could not read " + kind)
		}
		defer src.Close()
		if err := saveShopItemImage(src, imagePath); err != nil {
			return err
		}
		savedPaths = append(savedPaths, imagePath)
		return nil
	}

	if itemType == "hat" || itemType == "gear" {
		modelHeader, err := c.FormFile("model")
		if err != nil {
			cleanup()
			return fail("Missing 3D model", fiber.StatusBadRequest)
		}
		if strings.ToLower(filepath.Ext(modelHeader.Filename)) != ".obj" {
			cleanup()
			return fail("Invalid 3D model file type", fiber.StatusBadRequest)
		}
		if modelHeader.Size <= 0 {
			cleanup()
			return fail("Empty 3D model file", fiber.StatusBadRequest)
		}
		if modelHeader.Size > shopItemMaxModelBytes {
			cleanup()
			return fail("3D model file is too large", fiber.StatusBadRequest)
		}
		modelSrc, err := modelHeader.Open()
		if err != nil {
			cleanup()
			return fail("Could not read 3D model", fiber.StatusBadRequest)
		}
		modelPath := filepath.Join(dir, strconv.Itoa(itemID)+".obj")
		modelDst, err := os.Create(modelPath)
		if err != nil {
			modelSrc.Close()
			cleanup()
			return fail("Could not save upload", fiber.StatusInternalServerError)
		}
		if _, err := io.Copy(modelDst, modelSrc); err != nil {
			modelSrc.Close()
			modelDst.Close()
			cleanup()
			return fail("Could not save upload", fiber.StatusInternalServerError)
		}
		modelSrc.Close()
		modelDst.Close()
		savedPaths = append(savedPaths, modelPath)

		if mesh, err := fauxgl.LoadOBJ(modelPath); err != nil || mesh == nil || len(mesh.Triangles) == 0 {
			cleanup()
			return fail("Invalid 3D model file", fiber.StatusBadRequest)
		} else if len(mesh.Triangles) > shopItemMaxModelTriangles {
			cleanup()
			return fail("3D model has too many triangles (max 8192)", fiber.StatusBadRequest)
		}

		if err := saveImageField("file", "texture"); err != nil {
			cleanup()
			return fail(err.Error(), fiber.StatusBadRequest)
		}
	} else {
		if err := saveImageField("file", "image"); err != nil {
			cleanup()
			return fail(err.Error(), fiber.StatusBadRequest)
		}
	}

	if err := service.Shop.GrantItem(user.ID, itemID, models.BoughtViaAwarded, 0, 0); err != nil {
		cleanup()
		return fail("Could not save upload", fiber.StatusInternalServerError)
	}

	return succeed(itemID)
}

func AdminShopQueueAPI(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}

	category := c.Query("category")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "12"))

	if service.Shop == nil {
		return c.JSON(fiber.Map{
			"items":       []*models.ShopItem{},
			"total":       0,
			"page":        page,
			"limit":       limit,
			"total_pages": 1,
		})
	}

	items, total, err := service.Shop.GetApprovalQueue(category, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	totalPages := 1
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	return c.JSON(fiber.Map{
		"items":       items,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

func AdminShopReviewPost(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAuthError(c, err)
	}

	if !csrfValid(c) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	itemID, err := strconv.Atoi(c.Params("id"))
	if err != nil || itemID <= 0 {
		return adminModerationError(c, "Invalid item ID")
	}

	if service.Shop == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Shop service unavailable")
	}

	action := c.Params("action")
	var state string
	switch action {
	case "approve":
		state = models.ShopApprovalApproved
	case "reject":
		state = models.ShopApprovalDeclined
	default:
		return adminModerationError(c, "Unknown review action")
	}

	if err := service.Shop.ReviewItem(itemID, state); err != nil {
		return adminModerationError(c, err.Error())
	}

	if c.Get("Accept") == "application/json" || c.Get("X-Requested-With") == "XMLHttpRequest" || strings.HasPrefix(c.Path(), "/api/") {
		return c.JSON(fiber.Map{
			"success": true,
			"id":      itemID,
			"state":   state,
			"action":  action,
		})
	}

	return c.Redirect().To("/admin")
}

func CrateGetAPI(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	if service.User == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Service unavailable"})
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}
	itemIDStr := c.Params("id")
	if itemIDStr != "" {
		itemID, err := strconv.Atoi(itemIDStr)
		if err != nil || itemID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid item ID"})
		}
		owned := false
		if service.Shop != nil {
			owned = service.Shop.HasItem(user.ID, itemID)
		}
		return c.JSON(fiber.Map{"owned": owned, "item_id": itemID})
	}
	return c.JSON(fiber.Map{"user_id": user.ID})
}