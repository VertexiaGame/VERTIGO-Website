package handlers

import (
	"errors"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/renderer"
	"vertexia-frontend/backend/service"
)

var createCategoryNames = map[string]string{
	"hat":    "Hat",
	"face":   "Face",
	"shirt":  "Shirt",
	"tshirt": "T-Shirt",
	"pants":  "Pants",
	"tool":   "Tool",
}

const (
	createPreviewMaxModelBytes   = 25 << 20
	createPreviewMaxTextureBytes = 8 << 20
	gameThumbnailUploadDir       = "./static/uploads/games"
	gameThumbnailMaxImageBytes   = 8 << 20
)

var gameThumbnailImageExtensions = map[string]bool{".png": true, ".jpg": true, ".jpeg": true}

const gameMaxThumbnails = 5

func gameIconPath(gameID int) string {
	return filepath.Join(gameThumbnailUploadDir, strconv.Itoa(gameID)+".png")
}

func gameBannerPath(gameID int) string {
	return gameThumbPath(gameID, 1)
}

func gameThumbPath(gameID, index int) string {
	if index <= 1 {
		return filepath.Join(gameThumbnailUploadDir, "thumbs", strconv.Itoa(gameID)+".png")
	}
	return filepath.Join(gameThumbnailUploadDir, "thumbs", strconv.Itoa(gameID)+"_"+strconv.Itoa(index)+".png")
}

func gameThumbURL(gameID, index int) string {
	if index <= 1 {
		return "/static/uploads/games/thumbs/" + strconv.Itoa(gameID) + ".png"
	}
	return "/static/uploads/games/thumbs/" + strconv.Itoa(gameID) + "_" + strconv.Itoa(index) + ".png"
}

func gameThumbURLs(gameID int) []string {
	var urls []string
	for i := 1; i <= gameMaxThumbnails; i++ {
		if _, err := os.Stat(gameThumbPath(gameID, i)); err != nil {
			continue
		}
		urls = append(urls, gameThumbURL(gameID, i))
	}
	return urls
}

func clearGameThumbs(gameID int) {
	for i := 1; i <= gameMaxThumbnails; i++ {
		os.Remove(gameThumbPath(gameID, i))
	}
}

func gameThumbCount(gameID int) int {
	n := 0
	for i := 1; i <= gameMaxThumbnails; i++ {
		if _, err := os.Stat(gameThumbPath(gameID, i)); err != nil {
			break
		}
		n++
	}
	return n
}

func removeGameThumb(gameID, index int) error {
	if index < 1 || index > gameMaxThumbnails {
		return errors.New("Invalid thumbnail")
	}
	if _, err := os.Stat(gameThumbPath(gameID, index)); err != nil {
		return errors.New("Thumbnail not found")
	}
	if err := os.Remove(gameThumbPath(gameID, index)); err != nil {
		return errors.New("Could not remove thumbnail")
	}
	for i := index + 1; i <= gameMaxThumbnails; i++ {
		src := gameThumbPath(gameID, i)
		if _, err := os.Stat(src); err != nil {
			break
		}
		dst := gameThumbPath(gameID, i-1)
		os.Remove(dst)
		if err := os.Rename(src, dst); err != nil {
			return errors.New("Could not reorder thumbnails")
		}
	}
	return nil
}

func swapGameThumbs(gameID, a, b int) error {
	if a == b {
		return nil
	}
	if a < 1 || a > gameMaxThumbnails || b < 1 || b > gameMaxThumbnails {
		return errors.New("Invalid thumbnail position")
	}
	pa, pb := gameThumbPath(gameID, a), gameThumbPath(gameID, b)
	if _, err := os.Stat(pa); err != nil {
		return errors.New("Thumbnail not found")
	}
	if _, err := os.Stat(pb); err != nil {
		return errors.New("Thumbnail not found")
	}
	tmp, err := os.CreateTemp(filepath.Dir(pa), "thumbswap*.png")
	if err != nil {
		return errors.New("Could not reorder thumbnails")
	}
	tmpPath := tmp.Name()
	tmp.Close()
	os.Remove(tmpPath)
	if err := os.Rename(pa, tmpPath); err != nil {
		return errors.New("Could not reorder thumbnails")
	}
	if err := os.Rename(pb, pa); err != nil {
		os.Rename(tmpPath, pa)
		return errors.New("Could not reorder thumbnails")
	}
	if err := os.Rename(tmpPath, pb); err != nil {
		os.Rename(pa, pb)
		os.Rename(tmpPath, pa)
		return errors.New("Could not reorder thumbnails")
	}
	return nil
}

func hasGameFormKey(c fiber.Ctx, key string) bool {
	if form, err := c.MultipartForm(); err == nil && form != nil {
		if _, ok := form.Value[key]; ok {
			return true
		}
	}
	if args := c.Request().PostArgs(); args != nil && args.Has(key) {
		return true
	}
	if args := c.Request().URI().QueryArgs(); args != nil && args.Has(key) {
		return true
	}
	return false
}

func openGameUpload(c fiber.Ctx, fields ...string) *multipart.FileHeader {
	for _, field := range fields {
		if header, err := c.FormFile(field); err == nil && header != nil && header.Size > 0 {
			return header
		}
	}
	return nil
}

func checkGameUploadImage(header *multipart.FileHeader, kind string) error {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !gameThumbnailImageExtensions[ext] {
		return errors.New(kind + " must be a PNG or JPEG image")
	}
	if header.Size > gameThumbnailMaxImageBytes {
		return errors.New(kind + " file is too large")
	}
	return nil
}

func saveGameUploadImage(header *multipart.FileHeader, kind, dstPath string) error {
	if err := checkGameUploadImage(header, kind); err != nil {
		return err
	}
	src, err := header.Open()
	if err != nil {
		return errors.New("Could not read " + strings.ToLower(kind))
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return errors.New("Could not save upload")
	}
	if err := saveShopItemImage(src, dstPath); err != nil {
		if err.Error() == "invalid image file" {
			return errors.New(kind + " must be a PNG or JPEG image")
		}
		return errors.New("Could not save upload")
	}
	return nil
}

func gameUploadFiles(c fiber.Ctx, field string, max int, kind string) ([]*multipart.FileHeader, error) {
	form, err := c.MultipartForm()
	if err != nil || form == nil {
		return nil, nil
	}
	var out []*multipart.FileHeader
	for _, h := range form.File[field] {
		if h != nil && h.Size > 0 {
			out = append(out, h)
		}
	}
	if len(out) > max {
		return nil, errors.New("Up to 5 thumbnails allowed")
	}
	for _, h := range out {
		if err := checkGameUploadImage(h, kind); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func normalizeCreateType(value string) string {
	switch value {
	case "game", "shop", "asset":
		return value
	default:
		return "shop"
	}
}

func normalizeCreateCategory(value string) (string, string) {
	if name, ok := createCategoryNames[value]; ok {
		return value, name
	}
	return "hat", "Hat"
}

func normalizeAssetCategory(value string) string {
	switch value {
	case "image", "mesh", "sound":
		return value
	default:
		return "image"
	}
}

func CreateGet(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Redirect().To("/login")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Redirect().To("/login")
	}

	accountAge := time.Since(user.CreationDate)
	canUploadImage := user.HasAdminAccess() || accountAge >= 3*24*time.Hour
	canUploadMeshSound := user.HasAdminAccess() || accountAge >= 7*24*time.Hour
	canUploadClothing := user.HasAdminAccess() || accountAge >= 1*24*time.Hour
	canUploadAccessories := user.HasAdminAccess() || accountAge >= 3*24*time.Hour
	canCreatePublicGame := user.HasAdminAccess() || accountAge >= 1*24*time.Hour

	activeType := normalizeCreateType(c.Query("type", "shop"))
	activeCat, activeCatName := normalizeCreateCategory(c.Query("cat", "hat"))

	return Render(c, "pages/create", fiber.Map{
		"Title":                "Create - VERTEXIA",
		"ActiveType":           activeType,
		"ActiveCat":            activeCat,
		"ActiveCatName":        activeCatName,
		"ActiveAssetType":      normalizeAssetCategory(c.Query("asset", "image")),
		"CanUploadImage":       canUploadImage,
		"CanUploadMeshSound":   canUploadMeshSound,
		"CanUploadClothing":    canUploadClothing,
		"CanUploadAccessories": canUploadAccessories,
		"CanCreateGame":        true,
		"CanCreatePublicGame":  canCreatePublicGame,
		"AccountAgeDays":       int(accountAge.Hours() / 24),
	}, "layouts/main")
}

func CreatePreviewPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	category := c.FormValue("category", "")
	if _, ok := createCategoryNames[category]; !ok {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid category")
	}

	accountAge := time.Since(user.CreationDate)
	if category == "shirt" || category == "pants" || category == "tshirt" {
		if accountAge < 1*24*time.Hour && !user.HasAdminAccess() {
			return c.Status(fiber.StatusForbidden).SendString("Your account must be at least 1 day old to create clothing")
		}
	} else if category == "hat" || category == "tool" || category == "face" {
		if accountAge < 3*24*time.Hour && !user.HasAdminAccess() {
			return c.Status(fiber.StatusForbidden).SendString("Your account must be at least 3 days old to create hats, tools or faces")
		}
	}

	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader.Size <= 0 {
		imgBytes, rerr := renderer.RenderCreatePreview(category, "", "")
		if rerr != nil {
			if strings.Contains(rerr.Error(), "server busy") {
				return c.Status(fiber.StatusServiceUnavailable).SendString(rerr.Error())
			}
			return c.Status(fiber.StatusInternalServerError).SendString(rerr.Error())
		}
		c.Set("Content-Type", "image/png")
		return c.Send(imgBytes)
	}
	if fileHeader.Size > createPreviewMaxModelBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).SendString("Model file is too large")
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	isModel := category == "hat" || category == "tool"
	if isModel && ext != ".obj" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid model file")
	}
	if !isModel && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid image file")
	}

	src, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Could not read upload")
	}
	defer src.Close()

	tmpFile, err := os.CreateTemp("", "crtpreview*"+ext)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Could not save upload")
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmpFile, src); err != nil {
		tmpFile.Close()
		return c.Status(fiber.StatusInternalServerError).SendString("Could not save upload")
	}
	tmpFile.Close()

	var previewTexture string
	if isModel {
		if texHeader, err := c.FormFile("texture"); err == nil && texHeader != nil && texHeader.Size > 0 {
			if texHeader.Size > createPreviewMaxTextureBytes {
				return c.Status(fiber.StatusRequestEntityTooLarge).SendString("Texture file is too large")
			}
			texExt := strings.ToLower(filepath.Ext(texHeader.Filename))
			if texExt == "" {
				texExt = ".png"
			}
			if texExt == ".png" || texExt == ".jpg" || texExt == ".jpeg" {
				texSrc, err := texHeader.Open()
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).SendString("Could not read texture")
				}
				tmpTex, err := os.CreateTemp("", "crtpreviewtex*"+texExt)
				if err != nil {
					texSrc.Close()
					return c.Status(fiber.StatusInternalServerError).SendString("Could not save texture")
				}
				texTmpPath := tmpTex.Name()
				defer os.Remove(texTmpPath)
				if _, err := io.Copy(tmpTex, texSrc); err != nil {
					tmpTex.Close()
					texSrc.Close()
					return c.Status(fiber.StatusInternalServerError).SendString("Could not save texture")
				}
				tmpTex.Close()
				texSrc.Close()
				previewTexture = texTmpPath
			}
		}
	}

	imgBytes, err := renderer.RenderCreatePreview(category, tmpPath, previewTexture)
	if err != nil {
		if strings.Contains(err.Error(), "server busy") {
			return c.Status(fiber.StatusServiceUnavailable).SendString(err.Error())
		}
		msg := err.Error()
		if msg == "model has no geometry" || msg == "model is too complex for preview, reduce the triangle count" || msg == "could not parse model, make sure it is a valid OBJ file" {
			return c.Status(fiber.StatusBadRequest).SendString(msg)
		}
		return c.Status(fiber.StatusInternalServerError).SendString(msg)
	}

	c.Set("Content-Type", "image/png")
	return c.Send(imgBytes)
}

const maxGamesPerUser = 25

func gameCreationAgeBlocked(accountAge time.Duration, isAdmin, private bool) bool {
	return !private && accountAge < 24*time.Hour && !isAdmin
}

func gameLimitReached(gameCount int, isAdmin bool) bool {
	return !isAdmin && gameCount >= maxGamesPerUser
}

func gamePublishAgeBlocked(accountAge time.Duration, isAdmin, currentlyPrivate, wantsPublic bool) bool {
	return currentlyPrivate && wantsPublic && accountAge < 24*time.Hour && !isAdmin
}

func GameCreatePost(c fiber.Ctx) error {
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

	redirectError := func(msg string) error {
		return c.Redirect().To("/create?type=game&error=" + url.QueryEscape(msg))
	}

	fail := func(msg string, status int) error {
		if jsonResponse {
			return c.Status(status).JSON(fiber.Map{"error": msg})
		}
		return redirectError(msg)
	}

	succeed := func(gameID int) error {
		if jsonResponse {
			return c.JSON(fiber.Map{"success": true, "id": gameID})
		}
		return c.Redirect().To("/play/" + strconv.Itoa(gameID))
	}

	if service.Game == nil {
		return fail("Game service unavailable", fiber.StatusInternalServerError)
	}

	private := strings.ToLower(strings.TrimSpace(c.FormValue("visibility"))) == "private"
	maxPlayers, _ := strconv.Atoi(strings.TrimSpace(c.FormValue("max_players")))

	if gameCreationAgeBlocked(time.Since(user.CreationDate), user.HasAdminAccess(), private) {
		return fail("Your account must be at least 1 day old to create public games. You can still create a private game.", fiber.StatusForbidden)
	}

	if count, err := service.Game.CountUserGames(user.ID); err == nil && gameLimitReached(count, user.HasAdminAccess()) {
		return fail("You have reached the maximum number of games you can publish.", fiber.StatusTooManyRequests)
	}

	gameID, err := service.Game.CreateGameWithWarnings(user.ID, c.FormValue("name"), c.FormValue("description"), c.FormValue("genre"), c.FormValue("tags"), private, service.NormalizeMaxPlayers(maxPlayers, 100), c.FormValue("trigger_warnings"))
	if err != nil {
		return fail(err.Error(), fiber.StatusBadRequest)
	}

	thumbs, err := gameUploadFiles(c, "thumbnail", gameMaxThumbnails, "Thumbnail")
	if err != nil {
		_ = service.Game.DeleteGame(gameID)
		return fail(err.Error(), fiber.StatusBadRequest)
	}

	cleanup := func() {
		os.Remove(gameIconPath(gameID))
		clearGameThumbs(gameID)
		_ = service.Game.DeleteGame(gameID)
	}

	if header := openGameUpload(c, "icon", "file"); header != nil {
		if err := saveGameUploadImage(header, "Icon", gameIconPath(gameID)); err != nil {
			cleanup()
			return fail(err.Error(), fiber.StatusBadRequest)
		}
	}
	for i, header := range thumbs {
		if err := saveGameUploadImage(header, "Thumbnail", gameThumbPath(gameID, i+1)); err != nil {
			cleanup()
			return fail(err.Error(), fiber.StatusBadRequest)
		}
	}

	return succeed(gameID)
}

func MyGamesPage(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Redirect().To("/login")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Redirect().To("/login")
	}

	var games []*models.Game
	if service.Game != nil {
		games, _ = service.Game.GetUserGames(user.ID)
	}
	if games == nil {
		games = []*models.Game{}
	}

	return Render(c, "pages/mygames", fiber.Map{
		"Title":    "My Games - VERTEXIA",
		"Username": username,
		"Games":    games,
	}, "layouts/main")
}

func GameEditPage(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Redirect().To("/login")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Redirect().To("/login")
	}

	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return serveErrorPage(c, fiber.StatusNotFound)
	}

	if service.Game == nil {
		return serveErrorPage(c, fiber.StatusNotFound)
	}

	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	ownerID := 0
	if game.CreatorID.Valid {
		ownerID = int(game.CreatorID.Int64)
	}
	if ownerID != user.ID && !user.HasAdminAccess() {
		return serveErrorPage(c, fiber.StatusForbidden)
	}

	tags := ""
	if game.Tags.Valid {
		tags = game.Tags.String
	}
	visibility := "public"
	if game.Private {
		visibility = "private"
	}

	return Render(c, "pages/editgame", fiber.Map{
		"Title":      "Edit " + game.Name + " - VERTEXIA",
		"Game":       game,
		"TagsString": tags,
		"Visibility": visibility,
		"ThumbURLs":  gameThumbURLs(gameID),
	}, "layouts/main")
}

func GameUpdatePost(c fiber.Ctx) error {
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

	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		if jsonResponse {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid game ID"})
		}
		return c.Redirect().To("/create/games")
	}

	fail := func(msg string, status int) error {
		if jsonResponse {
			return c.Status(status).JSON(fiber.Map{"error": msg})
		}
		return c.Redirect().To("/create/games/" + strconv.Itoa(gameID) + "/edit?error=" + url.QueryEscape(msg))
	}

	if service.Game == nil {
		return fail("Game service unavailable", fiber.StatusInternalServerError)
	}

	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil {
		return fail("Game not found", fiber.StatusNotFound)
	}
	ownerID := 0
	if game.CreatorID.Valid {
		ownerID = int(game.CreatorID.Int64)
	}
	if ownerID != user.ID && !user.HasAdminAccess() {
		return fail("You do not own this game", fiber.StatusForbidden)
	}

	private := game.Private
	if hasGameFormKey(c, "visibility") {
		private = strings.ToLower(strings.TrimSpace(c.FormValue("visibility"))) == "private"
	}
	if gamePublishAgeBlocked(time.Since(user.CreationDate), user.HasAdminAccess(), game.Private, !private) {
		return fail("Your account must be at least 1 day old to make your game public", fiber.StatusForbidden)
	}
	maxPlayersRaw := strings.TrimSpace(c.FormValue("max_players"))
	maxPlayers := 0
	if maxPlayersRaw != "" {
		if parsed, err := strconv.Atoi(maxPlayersRaw); err == nil {
			maxPlayers = service.NormalizeMaxPlayers(parsed, 100)
		} else {
			return fail("Max players must be a number between 2 and 100", fiber.StatusBadRequest)
		}
	}
	triggerWarnings := c.FormValue("trigger_warnings")
	updateTriggerWarnings := hasGameFormKey(c, "trigger_warnings")

	if err := service.Game.UpdateGameWithWarnings(gameID, c.FormValue("name"), c.FormValue("description"), c.FormValue("genre"), c.FormValue("tags"), private, maxPlayers, triggerWarnings, updateTriggerWarnings); err != nil {
		return fail(err.Error(), fiber.StatusBadRequest)
	}

	if header := openGameUpload(c, "icon"); header != nil {
		if err := saveGameUploadImage(header, "Icon", gameIconPath(gameID)); err != nil {
			return fail(err.Error(), fiber.StatusBadRequest)
		}
	}
	thumbs, err := gameUploadFiles(c, "thumbnail", gameMaxThumbnails, "Thumbnail")
	if err != nil {
		return fail(err.Error(), fiber.StatusBadRequest)
	}
	if strings.TrimSpace(c.FormValue("clear_thumbnails")) == "1" || len(thumbs) > 0 {
		clearGameThumbs(gameID)
	}
	for i, header := range thumbs {
		if err := saveGameUploadImage(header, "Thumbnail", gameThumbPath(gameID, i+1)); err != nil {
			return fail(err.Error(), fiber.StatusBadRequest)
		}
	}

	if jsonResponse {
		return c.JSON(fiber.Map{"success": true, "id": gameID})
	}
	return c.Redirect().To("/play/" + strconv.Itoa(gameID))
}

func requireGameThumbOwner(c fiber.Ctx) (int, int, error) {
	username := GetActiveUser(c)
	if username == "" {
		return 0, fiber.StatusUnauthorized, errors.New("Your session expired. Please log in again, then retry.")
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return 0, fiber.StatusUnauthorized, errors.New("Your session expired. Please log in again, then retry.")
	}
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return 0, fiber.StatusBadRequest, errors.New("Invalid game ID")
	}
	if service.Game == nil {
		return 0, fiber.StatusInternalServerError, errors.New("Game service unavailable")
	}
	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil {
		return 0, fiber.StatusNotFound, errors.New("Game not found")
	}
	ownerID := 0
	if game.CreatorID.Valid {
		ownerID = int(game.CreatorID.Int64)
	}
	if ownerID != user.ID && !user.HasAdminAccess() {
		return 0, fiber.StatusForbidden, errors.New("You do not own this game")
	}
	return gameID, 0, nil
}

func gameThumbsPayload(gameID int) fiber.Map {
	return fiber.Map{"success": true, "id": gameID, "count": gameThumbCount(gameID), "thumbs": gameThumbURLs(gameID)}
}

func GameThumbRemovePost(c fiber.Ctx) error {
	gameID, status, err := requireGameThumbOwner(c)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{"error": err.Error()})
	}
	index, _ := strconv.Atoi(strings.TrimSpace(c.FormValue("index")))
	if err := removeGameThumb(gameID, index); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(gameThumbsPayload(gameID))
}

func GameThumbMovePost(c fiber.Ctx) error {
	gameID, status, err := requireGameThumbOwner(c)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{"error": err.Error()})
	}
	from, _ := strconv.Atoi(strings.TrimSpace(c.FormValue("from")))
	to, _ := strconv.Atoi(strings.TrimSpace(c.FormValue("to")))
	if err := swapGameThumbs(gameID, from, to); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(gameThumbsPayload(gameID))
}