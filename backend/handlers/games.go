package handlers

import (
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

func extractGameFilter(c fiber.Ctx) *models.GameFilter {
	search := strings.TrimSpace(c.Query("q"))
	if search == "" {
		search = strings.TrimSpace(c.Query("search"))
	}

	genre, _ := models.NormalizeGameGenre(c.Query("genre"))
	creator := strings.TrimSpace(c.Query("creator"))
	sort := models.NormalizeGameSort(c.Query("sort", "popular"))
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	return &models.GameFilter{
		Search:  search,
		Genre:   genre,
		Creator: creator,
		Sort:    sort,
		Page:    page,
		Limit:   limit,
	}
}

func PlayGet(c fiber.Ctx) error {
	filter := extractGameFilter(c)

	var popular []*models.Game
	var topPlaying []*models.Game
	var newest []*models.Game
	var classics []*models.Game
	var genreShelves []*models.GenreShelf

	if service.Game != nil {
		var err error
		popular, err = service.Game.GetPopularGames(18)
		if err != nil {
			log.Printf("PlayGet popular error: %v", err)
			popular = []*models.Game{}
		}
		topPlaying, err = service.Game.GetTopPlayingGames(18)
		if err != nil {
			log.Printf("PlayGet top playing error: %v", err)
			topPlaying = []*models.Game{}
		}
		newest, err = service.Game.GetNewestGames(12)
		if err != nil {
			log.Printf("PlayGet newest error: %v", err)
			newest = []*models.Game{}
		}
		classics, err = service.Game.GetClassicGames(12)
		if err != nil {
			log.Printf("PlayGet classics error: %v", err)
			classics = []*models.Game{}
		}
		genreShelves, err = service.Game.GetGenreShelves(3, 12)
		if err != nil {
			log.Printf("PlayGet genre shelves error: %v", err)
			genreShelves = nil
		}
	}

	var items []*models.Game
	totalItems := 0
	if service.Game != nil {
		var err error
		items, totalItems, err = service.Game.GetGamesFiltered(filter)
		if err != nil {
			log.Printf("PlayGet filtered error: %v", err)
			items = []*models.Game{}
		}
	}
	if items == nil {
		items = []*models.Game{}
	}

	totalPages := 1
	if totalItems > 0 && filter.Limit > 0 {
		totalPages = (totalItems + filter.Limit - 1) / filter.Limit
	}

	normGenre, normGenreName := models.NormalizeGameGenre(filter.Genre)
	browsing := filter.Search != "" || normGenre != "all" || filter.Creator != "" || filter.Sort != "popular"

	return Render(c, "pages/play", fiber.Map{
		"Title":         "Play - VERTEXIA",
		"Popular":       popular,
		"TopPlaying":    topPlaying,
		"Newest":        newest,
		"Classics":      classics,
		"GenreShelves":  genreShelves,
		"Items":         items,
		"TotalItems":    totalItems,
		"TotalPages":    totalPages,
		"CurrentPage":   filter.Page,
		"Filter":        filter,
		"ActiveGenre":   normGenre,
		"ActiveGenreNm": normGenreName,
		"Browsing":      browsing,
	}, "layouts/main")
}

func GameDetailGet(c fiber.Ctx) error {
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
	if game.Private || game.Locked {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	related, err := service.Game.GetRelatedGames(game, 6)
	if err != nil {
		log.Printf("GameDetailGet related error: %v", err)
		related = []*models.Game{}
	}
	if related == nil {
		related = []*models.Game{}
	}
	creatorID := 0
	if game.CreatorID.Valid {
		creatorID = int(game.CreatorID.Int64)
	}
	var tags []string
	if game.Tags.Valid {
		for _, t := range strings.Split(game.Tags.String, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}
	viewerRating := ""
	if username := GetActiveUser(c); username != "" && service.User != nil {
		if u, _ := service.User.GetUserByUsername(username); u != nil {
			viewerRating, _ = service.Game.GetUserRating(u.ID, gameID)
		}
	}
	return Render(c, "pages/game", fiber.Map{
		"Title":        game.Name + " - VERTEXIA",
		"Game":         game,
		"Related":      related,
		"CreatorID":    creatorID,
		"GenreName":    models.GameGenreDisplayName(game.GenreText),
		"Tags":         tags,
		"ViewerRating": viewerRating,
		"ThumbURLs":    gameThumbURLs(gameID),
	}, "layouts/main")
}

func GameRatePost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: please log in"})
	}
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid game ID"})
	}
	if service.User == nil || service.Game == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Service unavailable"})
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}
	rating := strings.TrimSpace(c.FormValue("rating"))
	if rating == "" {
		rating = strings.TrimSpace(c.Query("rating"))
	}
	if rating == "" {
		var body map[string]string
		if err := c.Bind().JSON(&body); err == nil {
			if v, ok := body["rating"]; ok {
				rating = strings.TrimSpace(v)
			}
		}
	}
	var action string
	switch strings.ToLower(rating) {
	case "like", "likes", "1":
		action = "like"
	case "dislike", "dislikes", "-1":
		action = "dislike"
	case "clear", "remove", "none", "0":
		action = "clear"
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid rating: use like, dislike or clear"})
	}
	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil || game.Private || game.Locked {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game not found"})
	}
	userRating, err := service.Game.RateGame(user.ID, gameID, action)
	if err != nil {
		log.Printf("GameRatePost error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not save rating"})
	}
	game, err = service.Game.GetGameByID(gameID)
	if err != nil || game == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not load rating"})
	}
	return c.JSON(fiber.Map{
		"success":     true,
		"likes":       game.Likes,
		"dislikes":    game.Dislikes,
		"like_pct":    game.LikePct,
		"has_rating":  game.HasRating(),
		"user_rating": userRating,
	})
}

func GamesAPI(c fiber.Ctx) error {
	filter := extractGameFilter(c)

	if service.Game == nil {
		return c.JSON(fiber.Map{
			"games":       []*models.Game{},
			"total":       0,
			"page":        filter.Page,
			"limit":       filter.Limit,
			"total_pages": 1,
		})
	}

	games, total, err := service.Game.GetGamesFiltered(filter)
	if err != nil {
		log.Printf("GamesAPI error: %v", err)
		return c.JSON(fiber.Map{
			"games":       []*models.Game{},
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
		"games":       games,
		"total":       total,
		"page":        filter.Page,
		"limit":       filter.Limit,
		"total_pages": totalPages,
	})
}
