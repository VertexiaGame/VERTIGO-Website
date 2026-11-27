package handlers

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/renderer"
	"vertexia-frontend/backend/service"
	"vertexia-frontend/backend/storage"
)

const (
	fallbackRejectPath = "./static/useful/reject.png"
	fallbackTimePath   = "./static/useful/time.png"
)

var assetTypeExtensions = map[string]map[string]bool{
	models.AssetTypeImage: {".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true},
	models.AssetTypeMesh:  {".glb": true, ".obj": true},
	models.AssetTypeSound: {".mp3": true, ".wav": true, ".ogg": true},
}

var assetSizeLimits = map[string]int64{
	models.AssetTypeImage: 8 << 20,
	models.AssetTypeMesh:  25 << 20,
	models.AssetTypeSound: 15 << 20,
}

const assetMeshMaxTriangles = 16384

func assetContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".glb":
		return "model/gltf-binary"
	case ".obj":
		return "text/plain"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}

func parseAssetID(rawID string) (int, error) {
	clean := strings.TrimSpace(rawID)
	if dotIdx := strings.Index(clean, "."); dotIdx != -1 {
		clean = clean[:dotIdx]
	}
	return strconv.Atoi(clean)
}

func validateAssetContent(assetType, ext string, header []byte) error {
	switch assetType {
	case models.AssetTypeImage:
		valid := bytes.HasPrefix(header, []byte{0xFF, 0xD8}) ||
			bytes.HasPrefix(header, []byte{0x89, 'P', 'N', 'G'}) ||
			bytes.HasPrefix(header, []byte("GIF8")) ||
			len(header) > 11 && bytes.HasPrefix(header, []byte("RIFF")) && string(header[8:12]) == "WEBP"
		if !valid {
			return errors.New("invalid image file")
		}
		if ext == ".png" && !bytes.HasPrefix(header, []byte{0x89, 'P', 'N', 'G'}) {
			return errors.New("invalid PNG file")
		}
		if ext == ".gif" && !bytes.HasPrefix(header, []byte("GIF8")) {
			return errors.New("invalid GIF file")
		}
	case models.AssetTypeMesh:
		if ext == ".glb" && !bytes.HasPrefix(header, []byte("glTF")) {
			return errors.New("invalid GLB file")
		}
	case models.AssetTypeSound:
		valid := bytes.HasPrefix(header, []byte("ID3")) ||
			(len(header) > 1 && header[0] == 0xFF && header[1]&0xE0 == 0xE0) ||
			len(header) > 11 && bytes.HasPrefix(header, []byte("RIFF")) && string(header[8:12]) == "WAVE" ||
			bytes.HasPrefix(header, []byte("OggS"))
		if !valid {
			return errors.New("invalid audio file")
		}
	}
	return nil
}

func wantsAssetJSON(c fiber.Ctx) bool {
	accept := c.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	if c.Get("X-Requested-With") == "XMLHttpRequest" {
		return true
	}
	return c.Get("HX-Request") == "true"
}

// isOwnerOrAdminForAsset accepts a bearer access token, the legacy Unikey
// or the browser session.
func isOwnerOrAdminForAsset(c fiber.Ctx, asset *models.Asset) bool {
	user := credentialUser(c)
	return user != nil && (user.ID == asset.UID || user.HasAdminAccess())
}

// isAuthorizedForAsset additionally lets gameservers (header key only) fetch
// any asset.
func isAuthorizedForAsset(c fiber.Ctx, asset *models.Asset) bool {
	return gameserverKeyValid(c) || isOwnerOrAdminForAsset(c, asset)
}

func AssetsPage(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" || service.User == nil {
		return c.Redirect().To("/login")
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Redirect().To("/login")
	}

	tab := c.Query("tab")
	switch tab {
	case "items", "games", "clans":
	default:
		tab = "assets"
	}

	var assets []*models.Asset
	if service.Asset != nil {
		assets, _ = service.Asset.GetUserAssets(user.ID)
	}
	var items []*models.ShopItem
	if service.Shop != nil {
		items, _ = service.Shop.GetUserItems(user.ID)
	}
	var games []*models.Game
	if service.Game != nil {
		games, _ = service.Game.GetUserGames(user.ID)
	}

	var owned, joined []*models.Clan
	if service.Clan != nil {
		owned, _ = service.Clan.GetOwnedClans(user.ID)
		joined, _ = service.Clan.GetJoinedClans(user.ID)
	}
	ownedIDs := make(map[int]bool, len(owned))
	for _, clan := range owned {
		ownedIDs[clan.ID] = true
	}
	crews := make([]*models.Clan, 0, len(joined))
	for _, clan := range joined {
		if !ownedIDs[clan.ID] {
			crews = append(crews, clan)
		}
	}
	_, notice := takeClanNotice(c)
	return Render(c, "pages/assets", fiber.Map{
		"Title": "My Assets - VERTEXIA", "ActiveTab": tab,
		"Assets": assets, "Items": items, "Games": games,
		"Owned": owned, "Joined": crews, "WornClanID": user.PrimaryClan,
		"Notice": notice,
	}, "layouts/main")
}

func AssetUploadPost(c fiber.Ctx) error {
	jsonResponse := wantsAssetJSON(c)

	username := GetActiveUser(c)
	if username == "" {
		if jsonResponse {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Your session expired. Please log in again, then retry the upload."})
		}
		return c.Redirect().To("/login")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if jsonResponse {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Your session expired. Please log in again, then retry the upload."})
		}
		return c.Redirect().To("/login")
	}

	assetType, _ := models.NormalizeAssetType(c.FormValue("type"))

	redirectError := func(msg string) error {
		return c.Redirect().To("/create?type=asset&asset=" + assetType + "&error=" + url.QueryEscape(msg))
	}

	fail := func(msg string, status int) error {
		if jsonResponse {
			return c.Status(status).JSON(fiber.Map{"error": msg})
		}
		return redirectError(msg)
	}

	succeed := func(assetID int) error {
		if jsonResponse {
			return c.JSON(fiber.Map{"success": true, "id": assetID})
		}
		return c.Redirect().To("/create/assets")
	}

	accountAge := time.Since(user.CreationDate)
	if assetType == models.AssetTypeImage {
		if accountAge < 3*24*time.Hour && !user.HasAdminAccess() {
			return fail("Your account must be at least 3 days old to upload image assets", fiber.StatusForbidden)
		}
	} else if assetType == models.AssetTypeMesh || assetType == models.AssetTypeSound {
		if accountAge < 7*24*time.Hour && !user.HasAdminAccess() {
			return fail("Your account must be at least 7 days old to upload "+models.AssetTypeNames[assetType]+" assets", fiber.StatusForbidden)
		}
	}

	extensions, ok := assetTypeExtensions[assetType]
	if !ok {
		return fail("Invalid asset type", fiber.StatusBadRequest)
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return fail("Missing file", fiber.StatusBadRequest)
	}

	// The client filename only gates obviously wrong uploads; the stored
	// extension and key are derived from the content and database ids.
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !extensions[ext] {
		return fail("Invalid file type for "+models.AssetTypeNames[assetType], fiber.StatusBadRequest)
	}

	if fileHeader.Size <= 0 {
		return fail("Empty file", fiber.StatusBadRequest)
	}
	data, err := readUpload(fileHeader, assetSizeLimits[assetType])
	if err != nil {
		return fail(assetUploadErrorMessage(err), fiber.StatusBadRequest)
	}

	storedExt, err := validateAssetUpload(assetType, ext, data)
	if err != nil {
		return fail(err.Error(), fiber.StatusBadRequest)
	}

	fileName, err := models.GenerateRandomString(24)
	if err != nil {
		recordInternalError(c, err)
		return fail("Could not save upload", fiber.StatusInternalServerError)
	}

	relPath := strconv.Itoa(user.ID) + "/" + fileName + storedExt
	fileKey, err := assetFileKey(relPath)
	if err != nil {
		recordInternalError(c, err)
		return fail("Could not save upload", fiber.StatusInternalServerError)
	}

	var texData []byte
	var texKey, texExt string
	if assetType == models.AssetTypeMesh {
		if texHeader, err := c.FormFile("texture"); err == nil && texHeader != nil && texHeader.Size > 0 {
			texClientExt := strings.ToLower(filepath.Ext(texHeader.Filename))
			if assetTypeExtensions[models.AssetTypeImage][texClientExt] {
				if raw, err := readUpload(texHeader, assetSizeLimits[models.AssetTypeImage]); err == nil {
					if e, err := validateAssetUpload(models.AssetTypeImage, texClientExt, raw); err == nil {
						texData, texExt = raw, e
						texKey, _ = assetFileKey(strconv.Itoa(user.ID) + "/" + fileName + "_tex" + texExt)
					}
				}
			}
		}
	}

	// Meshes are parsed from a private temporary file; it is reused for the
	// first render and removed afterwards.
	var meshTmp, texTmp string
	removeTemps := func() {
		if meshTmp != "" {
			os.Remove(meshTmp)
		}
		if texTmp != "" {
			os.Remove(texTmp)
		}
	}
	if assetType == models.AssetTypeMesh {
		meshTmp, err = writeTempFile("asset-mesh-*"+storedExt, data)
		if err != nil {
			recordInternalError(c, err)
			return fail("Could not save upload", fiber.StatusInternalServerError)
		}
		triangles, err := renderer.MeshTriangleCount(meshTmp)
		if err != nil || triangles == 0 {
			removeTemps()
			return fail("Invalid mesh file", fiber.StatusBadRequest)
		}
		if triangles > assetMeshMaxTriangles {
			removeTemps()
			return fail("Mesh has too many triangles (max 16384)", fiber.StatusBadRequest)
		}
		if texKey != "" {
			if texTmp, err = writeTempFile("asset-tex-*"+texExt, texData); err != nil {
				texTmp = ""
			}
		}
	}

	if service.Asset == nil {
		removeTemps()
		return fail("Asset service unavailable", fiber.StatusInternalServerError)
	}

	store := storage.Default()
	ctx, cancel := storeCtx(c)
	defer cancel()
	removeStored := func() {
		dctx, dcancel := bgStoreCtx()
		defer dcancel()
		_ = store.Delete(dctx, fileKey)
		if texKey != "" {
			_ = store.Delete(dctx, texKey)
		}
	}
	if err := storage.PutBytes(ctx, store, fileKey, data, assetContentType(storedExt)); err != nil {
		removeTemps()
		recordInternalError(c, err)
		return fail("Could not save upload", fiber.StatusInternalServerError)
	}
	if texKey != "" {
		if err := storage.PutBytes(ctx, store, texKey, texData, assetContentType(texExt)); err != nil {
			if texTmp != "" {
				os.Remove(texTmp)
			}
			texKey, texTmp = "", ""
		}
	}

	assetID, err := service.Asset.CreateAsset(user.ID, c.FormValue("name"), c.FormValue("description"), assetType, relPath)
	if err != nil {
		removeTemps()
		removeStored()
		return fail(err.Error(), fiber.StatusBadRequest)
	}

	if assetType == models.AssetTypeMesh {
		meshPath, texPath := meshTmp, texTmp
		go func() {
			defer removeTemps()
			if imgBytes, err := renderer.RenderMeshWithTexture(meshPath, texPath); err == nil && len(imgBytes) > 0 {
				if key, err := meshRenderKey(assetID); err == nil {
					pctx, pcancel := bgStoreCtx()
					defer pcancel()
					_ = storage.PutBytes(pctx, store, key, imgBytes, "image/png")
				}
			}
		}()
	} else {
		removeTemps()
	}

	return succeed(assetID)
}

func assetUploadErrorMessage(err error) string {
	switch {
	case errors.Is(err, errUploadTooLarge):
		return "File is too large"
	case errors.Is(err, errUploadEmpty):
		return "Empty file"
	default:
		return "Could not read upload"
	}
}

// validateAssetUpload runs the per-type content checks and returns the
// extension to store, derived from the content rather than the filename.
func validateAssetUpload(assetType, clientExt string, data []byte) (string, error) {
	header := data[:min(len(data), 512)]
	if err := validateAssetContent(assetType, clientExt, header); err != nil {
		return "", err
	}
	switch assetType {
	case models.AssetTypeImage:
		format, _, err := sniffImage(data, imgPNG, imgJPEG, imgGIF, imgWEBP)
		if err != nil {
			if errors.Is(err, errImageTooLarge) {
				return "", err
			}
			return "", errors.New("invalid image file")
		}
		return imageFormatExt[format], nil
	case models.AssetTypeMesh:
		if bytes.HasPrefix(data, []byte("glTF")) {
			return ".glb", nil
		}
		if clientExt != ".obj" {
			return "", errors.New("invalid GLB file")
		}
		return ".obj", nil
	case models.AssetTypeSound:
		switch {
		case bytes.HasPrefix(data, []byte("OggS")):
			return ".ogg", nil
		case len(data) > 11 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WAVE":
			return ".wav", nil
		default:
			return ".mp3", nil
		}
	}
	return "", errors.New("invalid asset type")
}

// assetFileKey maps an assets.file_path value ("<uid>/<name>.<ext>") to its
// storage key, rejecting anything that would leave uploads/assets.
func assetFileKey(filePath string) (string, error) {
	key, err := storage.Key(keyAssetUploads, filePath)
	if err != nil || !storage.HasPrefix(key, keyAssetUploads) {
		return "", storage.ErrInvalidKey
	}
	return key, nil
}

func meshRenderKey(assetID int) (string, error) {
	if assetID <= 0 {
		return "", storage.ErrInvalidKey
	}
	return storage.Key(keyMeshRenders, strconv.Itoa(assetID)+".png")
}

func writeTempFile(pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

// assetCacheControl keeps moderated content out of shared caches until it is
// approved.
func assetCacheControl(asset *models.Asset) string {
	if asset.ApprovalState == models.AssetApprovalApproved {
		return "public, max-age=86400"
	}
	return "private, no-store"
}

func AssetFileGet(c fiber.Ctx) error {
	assetID, err := parseAssetID(c.Params("id"))
	if err != nil || assetID <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid asset ID")
	}

	if service.Asset == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Asset service unavailable")
	}

	asset, err := service.Asset.GetByID(assetID)
	if err != nil || asset == nil {
		return c.Status(fiber.StatusNotFound).SendString("Asset not found")
	}

	if !isAuthorizedForAsset(c, asset) {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	if asset.ApprovalState == models.AssetApprovalRejected {
		c.Set("Content-Type", "image/png")
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Set("X-Asset-Status", models.AssetApprovalRejected)
		c.Set("X-Asset-Fallback", "true")
		return c.SendFile(fallbackRejectPath)
	}

	if asset.ApprovalState != models.AssetApprovalApproved && !isOwnerOrAdminForAsset(c, asset) {
		c.Set("Content-Type", "image/png")
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Set("X-Asset-Status", models.AssetApprovalPending)
		c.Set("X-Asset-Fallback", "true")
		return c.SendFile(fallbackTimePath)
	}

	key, err := assetFileKey(asset.FilePath)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid asset path")
	}

	c.Set("X-Asset-Status", asset.ApprovalState)
	err = sendStoredObject(c, key, assetContentType(path.Ext(key)), assetCacheControl(asset))
	if errors.Is(err, storage.ErrNotFound) {
		c.Set("Cache-Control", "no-store")
		return c.Status(fiber.StatusNotFound).SendString("Asset file not found")
	}
	if err != nil {
		recordInternalError(c, err)
		return c.Status(fiber.StatusServiceUnavailable).SendString("Asset storage unavailable")
	}
	return nil
}

func AssetStatusGet(c fiber.Ctx) error {
	assetID, err := parseAssetID(c.Params("id"))
	if err != nil || assetID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid asset ID"})
	}

	if service.Asset == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Asset service unavailable"})
	}

	asset, err := service.Asset.GetByID(assetID)
	if err != nil || asset == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Asset not found"})
	}

	return c.JSON(fiber.Map{
		"id":             asset.ID,
		"type":           asset.Type,
		"approval_state": asset.ApprovalState,
		"uid":            asset.UID,
	})
}

func AssetRenderGet(c fiber.Ctx) error {
	assetID, err := parseAssetID(c.Params("id"))
	if err != nil || assetID <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid asset ID")
	}

	if service.Asset == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Asset service unavailable")
	}

	asset, err := service.Asset.GetByID(assetID)
	if err != nil || asset == nil {
		return c.Status(fiber.StatusNotFound).SendString("Asset not found")
	}

	if !isAuthorizedForAsset(c, asset) {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	if asset.ApprovalState == models.AssetApprovalRejected {
		c.Set("Content-Type", "image/png")
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Set("X-Asset-Status", models.AssetApprovalRejected)
		c.Set("X-Asset-Fallback", "true")
		return c.SendFile(fallbackRejectPath)
	}

	if asset.ApprovalState != models.AssetApprovalApproved && !isOwnerOrAdminForAsset(c, asset) {
		c.Set("Content-Type", "image/png")
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Set("X-Asset-Status", models.AssetApprovalPending)
		c.Set("X-Asset-Fallback", "true")
		return c.SendFile(fallbackTimePath)
	}

	if asset.Type == models.AssetTypeImage {
		return AssetFileGet(c)
	}

	if asset.Type != models.AssetTypeMesh {
		return c.Status(fiber.StatusBadRequest).SendString("Asset is not a mesh")
	}

	cacheKey, err := meshRenderKey(assetID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid asset ID")
	}
	cacheControl := assetCacheControl(asset)
	store := storage.Default()
	ctx, cancel := storeCtx(c)
	defer cancel()
	if imgBytes, err := storage.GetBytes(ctx, store, cacheKey, 16<<20); err == nil && len(imgBytes) > 0 {
		c.Set("Content-Type", "image/png")
		c.Set("Cache-Control", cacheControl)
		c.Set("X-Content-Type-Options", "nosniff")
		return c.Send(imgBytes)
	}

	meshKey, err := assetFileKey(asset.FilePath)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid asset path")
	}
	meshPath, meshCleanup, err := storage.LocalCopy(ctx, store, meshKey, assetSizeLimits[models.AssetTypeMesh])
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).SendString("Asset file not found")
		}
		recordInternalError(c, err)
		return c.Status(fiber.StatusServiceUnavailable).SendString("Asset storage unavailable")
	}
	defer meshCleanup()

	var texPath string
	base := strings.TrimSuffix(meshKey, path.Ext(meshKey))
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif"} {
		texKey := base + "_tex" + ext
		if ok, _ := storage.Exists(ctx, store, texKey); !ok {
			continue
		}
		p, texCleanup, err := storage.LocalCopy(ctx, store, texKey, assetSizeLimits[models.AssetTypeImage])
		if err == nil {
			defer texCleanup()
			texPath = p
		}
		break
	}

	imgBytes, err := renderer.RenderMeshWithTexture(meshPath, texPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	if len(imgBytes) > 0 {
		_ = storage.PutBytes(ctx, store, cacheKey, imgBytes, "image/png")
	}

	c.Set("Content-Type", "image/png")
	c.Set("Cache-Control", cacheControl)
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Send(imgBytes)
}

func AdminAssetsAPI(c fiber.Ctx) error {
	_, err := getAdminUser(c)
	if err != nil {
		return adminAPIError(c, err)
	}

	category := c.Query("category")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "12"))

	if service.Asset == nil {
		return c.JSON(fiber.Map{
			"assets":      []*models.Asset{},
			"total":       0,
			"page":        page,
			"limit":       limit,
			"total_pages": 1,
		})
	}

	assets, total, err := service.Asset.GetQueue(category, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	totalPages := 1
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	return c.JSON(fiber.Map{
		"assets":      assets,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

func AdminAssetReviewPost(c fiber.Ctx) error {
	adminUser, err := getAdminUser(c)
	if err != nil {
		return adminAuthError(c, err)
	}

	if !csrfValid(c) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	assetID, err := strconv.Atoi(c.Params("id"))
	if err != nil || assetID <= 0 {
		return adminModerationError(c, "Invalid asset ID")
	}

	if service.Asset == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Asset service unavailable")
	}

	asset, err := service.Asset.GetByID(assetID)
	if err != nil || asset == nil {
		return adminModerationError(c, "Asset not found")
	}

	action := c.Params("action")
	state := models.AssetApprovalPending
	actionType := models.ActionApproveAsset
	switch action {
	case "approve":
		state = models.AssetApprovalApproved
		actionType = models.ActionApproveAsset
	case "reject":
		state = models.AssetApprovalRejected
		actionType = models.ActionRejectAsset
	default:
		return adminModerationError(c, "Unknown review action")
	}

	note := strings.TrimSpace(c.FormValue("note"))
	if err := service.Asset.Review(assetID, state, adminUser.ID, note); err != nil {
		return adminModerationError(c, err.Error())
	}

	if service.ModHistory != nil {
		_ = service.ModHistory.RecordAssetReview(adminUser, asset.UID, asset.ID, asset.Name, actionType, note)
	}

	if c.Get("Accept") == "application/json" || c.Get("X-Requested-With") == "XMLHttpRequest" || strings.HasPrefix(c.Path(), "/api/") {
		return c.JSON(fiber.Map{
			"success": true,
			"id":      assetID,
			"state":   state,
			"action":  action,
		})
	}

	return adminRedirectAssets(c)
}

func adminRedirectAssets(c fiber.Ctx) error {
	path := "/admin?tab=assets"
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", path)
		return c.SendStatus(fiber.StatusOK)
	}
	return c.Redirect().To(path)
}
