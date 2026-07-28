package handlers

import (
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"vertexia-frontend/backend/service"
)

func isAJAXRequest(c fiber.Ctx) bool {
	return c.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(c.Get("Accept"), "application/json")
}

func SettingsGet(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Redirect().To("/login")
	}

	if service.User == nil {
		return c.Redirect().To("/")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Redirect().To("/login")
	}

	changesLeft := 2
	if left, err := service.User.GetUsernameChangesLeft(user.ID); err == nil {
		changesLeft = left
	}

	errParam := c.Query("error")
	successParam := c.Query("success")
	tabParam := c.Query("tab", "account")

	return Render(c, "pages/settings", fiber.Map{
		"Title":               "Settings - VERTEXIA",
		"Error":               errParam,
		"Success":             successParam,
		"ActiveTab":           tabParam,
		"UserBio":             user.Description,
		"UserPronouns":        user.Pronouns,
		"UserSocials":         user.ParsedSocials(),
		"CurUsername":         user.Username,
		"CurDisplayName":      user.DisplayName,
		"UsernameChangesLeft": changesLeft,
	}, "layouts/main")
}

func SettingsDisplayNamePost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		return c.Redirect().To("/login")
	}

	if service.User == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Database offline"})
		}
		return c.Redirect().To("/settings?error=Database offline")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Redirect().To("/login")
	}

	newDisplayName := c.FormValue("displayname")
	if err := service.User.ChangeDisplayName(user.ID, newDisplayName); err != nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect().To("/settings?error=" + err.Error())
	}

	if isAJAXRequest(c) {
		return c.JSON(fiber.Map{
			"success":     "Display name changed successfully",
			"displayname": newDisplayName,
		})
	}

	return c.Redirect().To("/settings?success=Display name changed successfully")
}

func SettingsUsernamePost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		return c.Redirect().To("/login")
	}

	if service.User == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Database offline"})
		}
		return c.Redirect().To("/settings?error=Database offline")
	}

	sess := session.FromContext(c)
	if sess == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Session expired"})
		}
		return c.Redirect().To("/login")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Redirect().To("/login")
	}

	newUsername := c.FormValue("username")
	if err := service.User.ChangeUsername(user.ID, username, newUsername); err != nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect().To("/settings?error=" + err.Error())
	}

	sess.Set("username", newUsername)
	SetHashedCookie(c, user.ID)

	updatedUser, _ := service.User.GetUserByID(user.ID)
	newBucks := user.Bucks - 100
	if updatedUser != nil {
		newBucks = updatedUser.Bucks
	}

	changesLeft := 0
	if left, err := service.User.GetUsernameChangesLeft(user.ID); err == nil {
		changesLeft = left
	}

	if isAJAXRequest(c) {
		return c.JSON(fiber.Map{
			"success":      "Username changed successfully",
			"username":     newUsername,
			"bucks":        newBucks,
			"changes_left": changesLeft,
		})
	}

	return c.Redirect().To("/settings?success=Username changed successfully")
}

func SettingsPasswordPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		return c.Redirect().To("/login")
	}

	if service.User == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Database offline"})
		}
		return c.Redirect().To("/settings?error=Database offline")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Redirect().To("/login")
	}

	currentPassword := c.FormValue("current_password")
	newPassword := c.FormValue("new_password")
	retypePassword := c.FormValue("retype_password")

	if err := service.User.ChangePassword(user.ID, currentPassword, newPassword, retypePassword); err != nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect().To("/settings?error=" + err.Error())
	}

	if isAJAXRequest(c) {
		return c.JSON(fiber.Map{
			"success": "Password updated successfully",
		})
	}

	return c.Redirect().To("/settings?success=Password updated successfully")
}

func SettingsBioPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		return c.Redirect().To("/login")
	}

	if service.User == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Database offline"})
		}
		return c.Redirect().To("/settings?error=Database offline")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Redirect().To("/login")
	}

	bio := c.FormValue("bio")
	if err := service.User.UpdateBio(user.ID, bio); err != nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect().To("/settings?error=" + err.Error())
	}

	if isAJAXRequest(c) {
		return c.JSON(fiber.Map{
			"success": "Bio updated successfully",
			"bio":     bio,
		})
	}

	return c.Redirect().To("/settings?success=Bio updated successfully")
}

func SettingsPronounsPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		return c.Redirect().To("/login")
	}

	if service.User == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Database offline"})
		}
		return c.Redirect().To("/settings?error=Database offline")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Redirect().To("/login")
	}

	pronouns := c.FormValue("pronouns")
	if err := service.User.UpdatePronouns(user.ID, pronouns); err != nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect().To("/settings?error=" + err.Error())
	}

	if isAJAXRequest(c) {
		return c.JSON(fiber.Map{
			"success":  "Pronouns saved successfully",
			"pronouns": pronouns,
		})
	}

	return c.Redirect().To("/settings?success=Pronouns saved successfully")
}

func SettingsSocialsPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		return c.Redirect().To("/login")
	}

	if service.User == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Database offline"})
		}
		return c.Redirect().To("/settings?error=Database offline")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Redirect().To("/login")
	}

	socialsMap := make(map[string]string)
	if user.Socials != "" {
		_ = json.Unmarshal([]byte(user.Socials), &socialsMap)
	}

	reqPlatform := strings.ToLower(strings.TrimSpace(c.FormValue("platform")))
	reqValue := strings.TrimSpace(c.FormValue("value"))

	if reqPlatform != "" {
		if reqValue == "" {
			delete(socialsMap, reqPlatform)
		} else {
			socialsMap[reqPlatform] = reqValue
		}
	} else {
		platforms := []string{"discord", "twitter", "youtube", "twitch", "github", "instagram", "tiktok", "steam"}
		for _, p := range platforms {
			val := strings.TrimSpace(c.FormValue(p))
			if val != "" {
				socialsMap[p] = val
			} else if c.FormValue("batch") == "true" {
				delete(socialsMap, p)
			}
		}
	}

	if err := service.User.UpdateSocials(user.ID, socialsMap); err != nil {
		if isAJAXRequest(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect().To("/settings?error=" + err.Error())
	}

	if isAJAXRequest(c) {
		return c.JSON(fiber.Map{
			"success": "Social links saved successfully",
		})
	}

	return c.Redirect().To("/settings?success=Social links saved successfully")
}