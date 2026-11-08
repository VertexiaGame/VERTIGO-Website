package handlers

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

const usersPageSize = 10
const usersMaxPage = 1000

func userListParams(query, sort string, onlineOnly bool, page int) url.Values {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}
	if sort != "" && sort != "active" {
		values.Set("sort", sort)
	}
	if onlineOnly {
		values.Set("online", "1")
	}
	if page > 1 {
		values.Set("page", strconv.Itoa(page))
	}
	return values
}

func userDirectoryURL(query, sort string, onlineOnly bool, page int) string {
	if encoded := userListParams(query, sort, onlineOnly, page).Encode(); encoded != "" {
		return "/users?" + encoded
	}
	return "/users"
}

func userCardsURL(query, sort string, onlineOnly bool, page int) string {
	if encoded := userListParams(query, sort, onlineOnly, page).Encode(); encoded != "" {
		return "/users/cards?" + encoded
	}
	return "/users/cards"
}

func parseUsersQuery(c fiber.Ctx) (query, sort string, onlineOnly bool, page int) {
	query = strings.TrimSpace(c.Query("q"))
	if runes := []rune(query); len(runes) > 80 {
		query = string(runes[:80])
	}

	sort = strings.ToLower(strings.TrimSpace(c.Query("sort")))
	switch sort {
	case "active", "relevance", "newest", "oldest", "name":
	default:
		sort = ""
	}
	if sort == "" {
		if query != "" {
			sort = "relevance"
		} else {
			sort = "active"
		}
	}
	if sort == "relevance" && query == "" {
		sort = "active"
	}

	onlineValue := strings.TrimSpace(c.Query("online"))
	onlineOnly = onlineValue == "1" || strings.EqualFold(onlineValue, "true")

	page, _ = strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	if page > usersMaxPage {
		page = usersMaxPage
	}
	return query, sort, onlineOnly, page
}

func usersCountLabel(shown, total int) string {
	if total <= 0 {
		return "No users"
	}
	if shown > total {
		shown = total
	}
	if shown < 0 {
		shown = 0
	}
	noun := "users"
	if total == 1 {
		noun = "user"
	}
	return fmt.Sprintf("Showing %d of %d %s", shown, total, noun)
}

func UsersGet(c fiber.Ctx) error {
	query, sort, onlineOnly, page := parseUsersQuery(c)

	users := []*models.User{}
	totalItems := 0
	onlineCount := -1
	loadFailed := service.User == nil
	if service.User != nil {
		var err error
		users, totalItems, err = service.User.DiscoverUsers(query, sort, onlineOnly, usersPageSize, (page-1)*usersPageSize)
		if err != nil {
			log.Printf("UsersGet discovery error: %v", err)
			users = []*models.User{}
			totalItems = 0
			loadFailed = true
		}
		onlineCount, err = service.User.GetOnlineUserCount()
		if err != nil {
			log.Printf("UsersGet online count error: %v", err)
			onlineCount = -1
		}
	}

	totalPages := 1
	if totalItems > 0 {
		totalPages = (totalItems + usersPageSize - 1) / usersPageSize
	}
	if !loadFailed && totalItems > 0 && page > totalPages {
		page = totalPages
		var err error
		users, _, err = service.User.DiscoverUsers(query, sort, onlineOnly, usersPageSize, (page-1)*usersPageSize)
		if err != nil {
			log.Printf("UsersGet page error: %v", err)
			users = []*models.User{}
			loadFailed = true
		}
	}

	resultTitle := "All Users"
	if query != "" {
		resultTitle = "Search Results"
	} else if onlineOnly {
		resultTitle = "Online Now"
	} else if sort == "newest" {
		resultTitle = "New Members"
	}

	allSort := sort
	if allSort == "newest" {
		allSort = "active"
	}

	errorMessage := ""
	if loadFailed {
		errorMessage = "We couldn't load users right now. Please try again."
	}

	shown := page * usersPageSize
	hasMore := !loadFailed && page*usersPageSize < totalItems
	nextURL := ""
	if hasMore {
		nextURL = userCardsURL(query, sort, onlineOnly, page+1)
	}

	return Render(c, "pages/users", fiber.Map{
		"Title":       "Users - VERTEXIA",
		"Users":       users,
		"Query":       query,
		"Sort":        sort,
		"OnlineOnly":  onlineOnly,
		"OnlineCount": onlineCount,
		"TotalItems":  totalItems,
		"ResultTitle": resultTitle,
		"CountLabel":  usersCountLabel(shown, totalItems),
		"HasMore":     hasMore,
		"NextURL":     nextURL,
		"AllURL":      userDirectoryURL(query, allSort, false, 1),
		"OnlineURL":   userDirectoryURL(query, sort, true, 1),
		"NewestURL":   userDirectoryURL(query, "newest", false, 1),
		"ResetURL":    "/users",
		"Error":       errorMessage,
	}, "layouts/main")
}

func UsersCardsGet(c fiber.Ctx) error {
	query, sort, onlineOnly, page := parseUsersQuery(c)

	if service.User == nil {
		return c.Render("components/users/batch", fiber.Map{
			"Users":      []*models.User{},
			"CountLabel": "",
			"HasMore":    false,
		})
	}

	users, totalItems, err := service.User.DiscoverUsers(query, sort, onlineOnly, usersPageSize, (page-1)*usersPageSize)
	if err != nil {
		log.Printf("UsersCardsGet discovery error: %v", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	hasMore := page*usersPageSize < totalItems
	nextURL := ""
	if hasMore {
		nextURL = userCardsURL(query, sort, onlineOnly, page+1)
	}

	return c.Render("components/users/batch", fiber.Map{
		"Users":      users,
		"CountLabel": usersCountLabel(page*usersPageSize, totalItems),
		"HasMore":    hasMore,
		"NextURL":    nextURL,
	})
}
