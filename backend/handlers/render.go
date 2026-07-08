package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/database"
	"vertexia-frontend/backend/models"
)

func GetActiveUser(c fiber.Ctx) string {
	sess := session.FromContext(c)
	if sess == nil {
		return ""
	}

	usernameVal := sess.Get("username")
	if usernameStr, ok := usernameVal.(string); ok && usernameStr != "" {
		return usernameStr
	}

	cookieVal := c.Cookies("vertexia_remember")
	if cookieVal == "" {
		return ""
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(cookieVal)
	if err != nil {
		return ""
	}

	parts := strings.SplitN(string(decodedBytes), ":", 2)
	if len(parts) != 2 {
		return ""
	}

	cookieUser := parts[0]
	cookieSig := parts[1]

	user, err := models.GetUserByUsername(database.DB, cookieUser)
	if err != nil || user == nil {
		return ""
	}

	secret := config.Global.AltchaHMACKey
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(user.Username + ":" + user.Unikey))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(cookieSig), []byte(expectedSig)) {
		return ""
	}

	_ = models.UpdateUserOnline(database.DB, user.ID)
	sess.Set("username", user.Username)
	return user.Username
}

func Render(c fiber.Ctx, view string, data fiber.Map, layouts ...string) error {
	layout := "layouts/main"
	if len(layouts) > 0 {
		layout = layouts[0]
	}

	username := GetActiveUser(c)
	if username != "" {
		data["Username"] = username
		data["IsLoggedIn"] = true

		bucks := 0
		bits := 0
		if database.DB != nil {
			u, err := models.GetUserByUsername(database.DB, username)
			if err == nil && u != nil {
				bucks = u.Bucks
				bits = u.Bits
			}
		}
		data["Bucks"] = bucks
		data["Bits"] = bits
	} else {
		data["Username"] = ""
		data["IsLoggedIn"] = false
		data["Bucks"] = 0
		data["Bits"] = 0
	}

	return c.Render(view, data, layout)
}