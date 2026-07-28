package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/service"
)

func FriendsGet(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Redirect().To("/login")
	}
	if service.User == nil || service.Friend == nil {
		return c.Redirect().To("/")
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Redirect().To("/login")
	}

	idParam := c.Params("id")
	if idParam != "" {
		targetID, err := strconv.Atoi(idParam)
		if err == nil && targetID > 0 && targetID != user.ID {
			targetUser, err := service.User.GetUserByID(targetID)
			if err != nil || targetUser == nil {
				return c.Status(fiber.StatusNotFound).SendString("User not found")
			}
			friends, err := service.Friend.GetFriendsList(targetID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Error loading friends list")
			}
			return Render(c, "pages/friends", fiber.Map{
				"Title":            targetUser.DisplayName + " (@" + targetUser.Username + ")'s Friends - VERTEXIA",
				"ActiveTab":        "friends",
				"FriendsList":      friends,
				"IsOwnFriendsPage": false,
				"TargetUser":       targetUser,
			}, "layouts/main")
		}
	}

	incoming, outgoing, friends, err := service.Friend.GetFriendRequestsPageData(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error loading friend requests")
	}

	return Render(c, "pages/friends", fiber.Map{
		"Title":            "Friends & Requests - VERTEXIA",
		"ActiveTab":        "requests",
		"IncomingRequests": incoming,
		"OutgoingRequests": outgoing,
		"FriendsList":      friends,
		"IsOwnFriendsPage": true,
	}, "layouts/main")
}

func FriendSendPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil || targetID <= 0 {
		return c.Redirect().To("/")
	}
	if service.User == nil || service.Friend == nil {
		return c.Redirect().To("/user/" + strconv.Itoa(targetID))
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}

	_ = service.Friend.SendFriendRequest(user.ID, targetID)

	if c.Get("HX-Request") == "true" {
		ref := c.Get("Referer")
		if strings.Contains(ref, "/user/") {
			targetUser, _ := service.User.GetUserByID(targetID)
			if targetUser != nil {
				friendStatus, _ := service.Friend.GetFriendStatus(user.ID, targetID)
				return c.Render("components/profile/actions", fiber.Map{
					"ProfileUser":  targetUser,
					"FriendStatus": friendStatus,
					"IsOwnProfile": false,
				})
			}
		}
		return c.SendString("")
	}

	ref := c.Get("Referer")
	if ref != "" {
		return c.Redirect().To(ref)
	}
	return c.Redirect().To("/user/" + strconv.Itoa(targetID))
}

func FriendAcceptPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil || targetID <= 0 {
		return c.Redirect().To("/")
	}
	if service.User == nil || service.Friend == nil {
		return c.Redirect().To("/user/" + strconv.Itoa(targetID))
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}

	_ = service.Friend.AcceptFriendRequest(user.ID, targetID)

	if c.Get("HX-Request") == "true" {
		ref := c.Get("Referer")
		if strings.Contains(ref, "/user/") {
			targetUser, _ := service.User.GetUserByID(targetID)
			if targetUser != nil {
				friendStatus, _ := service.Friend.GetFriendStatus(user.ID, targetID)
				return c.Render("components/profile/actions", fiber.Map{
					"ProfileUser":  targetUser,
					"FriendStatus": friendStatus,
					"IsOwnProfile": false,
				})
			}
		}
		return c.SendString("")
	}

	ref := c.Get("Referer")
	if ref != "" {
		return c.Redirect().To(ref)
	}
	return c.Redirect().To("/user/" + strconv.Itoa(targetID))
}

func FriendDeclinePost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil || targetID <= 0 {
		return c.Redirect().To("/")
	}
	if service.User == nil || service.Friend == nil {
		return c.Redirect().To("/friends")
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}

	_ = service.Friend.DeclineFriendRequest(user.ID, targetID)

	if c.Get("HX-Request") == "true" {
		return c.SendString("")
	}

	ref := c.Get("Referer")
	if ref != "" {
		return c.Redirect().To(ref)
	}
	return c.Redirect().To("/friends")
}

func FriendCancelPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil || targetID <= 0 {
		return c.Redirect().To("/")
	}
	if service.User == nil || service.Friend == nil {
		return c.Redirect().To("/user/" + strconv.Itoa(targetID))
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}

	_ = service.Friend.CancelFriendRequest(user.ID, targetID)

	if c.Get("HX-Request") == "true" {
		ref := c.Get("Referer")
		if strings.Contains(ref, "/user/") {
			targetUser, _ := service.User.GetUserByID(targetID)
			if targetUser != nil {
				friendStatus, _ := service.Friend.GetFriendStatus(user.ID, targetID)
				return c.Render("components/profile/actions", fiber.Map{
					"ProfileUser":  targetUser,
					"FriendStatus": friendStatus,
					"IsOwnProfile": false,
				})
			}
		}
		return c.SendString("")
	}

	ref := c.Get("Referer")
	if ref != "" {
		return c.Redirect().To(ref)
	}
	return c.Redirect().To("/user/" + strconv.Itoa(targetID))
}

func FriendRemovePost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil || targetID <= 0 {
		return c.Redirect().To("/")
	}
	if service.User == nil || service.Friend == nil {
		return c.Redirect().To("/user/" + strconv.Itoa(targetID))
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}

	_ = service.Friend.RemoveFriend(user.ID, targetID)

	if c.Get("HX-Request") == "true" {
		ref := c.Get("Referer")
		if strings.Contains(ref, "/user/") {
			targetUser, _ := service.User.GetUserByID(targetID)
			if targetUser != nil {
				friendStatus, _ := service.Friend.GetFriendStatus(user.ID, targetID)
				return c.Render("components/profile/actions", fiber.Map{
					"ProfileUser":  targetUser,
					"FriendStatus": friendStatus,
					"IsOwnProfile": false,
				})
			}
		}
		return c.SendString("")
	}

	ref := c.Get("Referer")
	if ref != "" {
		return c.Redirect().To(ref)
	}
	return c.Redirect().To("/user/" + strconv.Itoa(targetID))
}