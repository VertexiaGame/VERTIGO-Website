package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/database"
	"vertexia-frontend/backend/models"
)

var replayCache = models.NewReplayCache()

func verifyAltcha(payloadStr string) error {
	if payloadStr == "" {
		return errors.New("CAPTCHA is required")
	}

	decoded, err := base64.StdEncoding.DecodeString(payloadStr)
	if err != nil {
		return errors.New("invalid CAPTCHA encoding")
	}

	var payload struct {
		Algorithm string `json:"algorithm"`
		Challenge string `json:"challenge"`
		Number    int    `json:"number"`
		Salt      string `json:"salt"`
		Signature string `json:"signature"`
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return errors.New("invalid CAPTCHA format")
	}

	parts := strings.Split(payload.Salt, "?expires=")
	if len(parts) != 2 {
		return errors.New("invalid CAPTCHA salt format")
	}

	expiresUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return errors.New("invalid CAPTCHA expiration format")
	}

	if time.Now().Unix() > expiresUnix {
		return errors.New("CAPTCHA challenge has expired")
	}

	secret := config.Global.AltchaHMACKey

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload.Challenge))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(payload.Signature), []byte(expectedSignature)) {
		return errors.New("CAPTCHA signature verification failed")
	}

	h := sha256.New()
	h.Write([]byte(payload.Salt + strconv.Itoa(payload.Number)))
	expectchlng := hex.EncodeToString(h.Sum(nil))
	if payload.Challenge != expectchlng {
		return errors.New("CAPTCHA challenge verification failed")
	}

	if !replayCache.Add(payload.Signature, time.Unix(expiresUnix, 0)) {
		return errors.New("CAPTCHA challenge has already been used")
	}

	return nil
}

func AltchaGet(c fiber.Ctx) error {
	secret := config.Global.AltchaHMACKey

	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")

	expat := time.Now().Add(15 * time.Minute).Unix()

	saltBytes := make([]byte, 12)
	if _, err := rand.Read(saltBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate salt",
		})
	}
	salt := hex.EncodeToString(saltBytes) + "?expires=" + strconv.FormatInt(expat, 10)

	maxNumber := 50000
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate number",
		})
	}
	number := (int(binary.BigEndian.Uint64(b)) % maxNumber) + 1

	h := sha256.New()
	h.Write([]byte(salt + strconv.Itoa(number)))
	challenge := hex.EncodeToString(h.Sum(nil))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(challenge))
	signature := hex.EncodeToString(mac.Sum(nil))

	return c.JSON(fiber.Map{
		"algorithm": "SHA-256",
		"challenge": challenge,
		"salt":      salt,
		"signature": signature,
		"maxnumber": maxNumber,
	})
}

func setHashedCookie(c fiber.Ctx, username, unikey string) {
	secret := config.Global.AltchaHMACKey
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(username + ":" + unikey))
	signature := hex.EncodeToString(mac.Sum(nil))

	payload := username + ":" + signature
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))

	cookie := &fiber.Cookie{
		Name:     "vertexia_remember",
		Value:    encoded,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   config.Global.SessionSecure,
		SameSite: config.Global.SessionSameSite,
	}
	c.Cookie(cookie)
}

func LoginGet(c fiber.Ctx) error {
	if username := GetActiveUser(c); username != "" {
		return c.Redirect().To("/")
	}
	return Render(c, "pages/login", fiber.Map{
		"Title": "Log In - VERTEXIA",
	}, "layouts/main")
}

func LoginPost(c fiber.Ctx) error {
	if username := GetActiveUser(c); username != "" {
		return c.Redirect().To("/")
	}

	altchaPayload := c.FormValue("altcha")
	if err := verifyAltcha(altchaPayload); err != nil {
		return Render(c, "pages/login", fiber.Map{
			"Title": "Log In - VERTEXIA",
			"Error": err.Error(),
		}, "layouts/main")
	}

	identifier := c.FormValue("identifier")
	password := c.FormValue("password")

	if identifier == "" || password == "" {
		return Render(c, "pages/login", fiber.Map{
			"Title": "Log In - VERTEXIA",
			"Error": "Username/Email and Password are required",
		}, "layouts/main")
	}

	user, err := models.AuthenticateUser(database.DB, identifier, password)
	if err != nil {
		return Render(c, "pages/login", fiber.Map{
			"Title": "Log In - VERTEXIA",
			"Error": err.Error(),
		}, "layouts/main")
	}

	_ = models.UpdateUserOnline(database.DB, user.ID)

	sess := session.FromContext(c)
	if sess != nil {
		sess.Set("username", user.Username)
	}

	setHashedCookie(c, user.Username, user.Unikey)

	return c.Redirect().To("/")
}

func RegisterGet(c fiber.Ctx) error {
	if username := GetActiveUser(c); username != "" {
		return c.Redirect().To("/")
	}
	return Render(c, "pages/register", fiber.Map{
		"Title": "Register - VERTEXIA",
	}, "layouts/main")
}

func RegisterPost(c fiber.Ctx) error {
	if username := GetActiveUser(c); username != "" {
		return c.Redirect().To("/")
	}

	altchaPayload := c.FormValue("altcha")
	if err := verifyAltcha(altchaPayload); err != nil {
		return Render(c, "pages/register", fiber.Map{
			"Title": "Register - VERTEXIA",
			"Error": err.Error(),
		}, "layouts/main")
	}

	username := c.FormValue("username")
	displayname := c.FormValue("displayname")
	email := c.FormValue("email")
	password := c.FormValue("password")
	passwordConfirm := c.FormValue("password_confirm")

	if username == "" || email == "" || password == "" {
		return Render(c, "pages/register", fiber.Map{
			"Title": "Register - VERTEXIA",
			"Error": "All required fields must be filled!",
		}, "layouts/main")
	}

	if password != passwordConfirm {
		return Render(c, "pages/register", fiber.Map{
			"Title": "Register - VERTEXIA",
			"Error": "Passwords do not match",
		}, "layouts/main")
	}

	if len(password) < 8 {
		return Render(c, "pages/register", fiber.Map{
			"Title": "Register - VERTEXIA",
			"Error": "Password must be at least 8 characters long",
		}, "layouts/main")
	}

	user, err := models.CreateUser(database.DB, username, displayname, email, password)
	if err != nil {
		return Render(c, "pages/register", fiber.Map{
			"Title": "Register - VERTEXIA",
			"Error": err.Error(),
		}, "layouts/main")
	}

	sess := session.FromContext(c)
	if sess != nil {
		sess.Set("username", user.Username)
	}

	setHashedCookie(c, user.Username, user.Unikey)

	return c.Redirect().To("/")
}

func Logout(c fiber.Ctx) error {
	sess := session.FromContext(c)
	if sess != nil {
		_ = sess.Destroy()
	}

	cookie := &fiber.Cookie{
		Name:     "vertexia_remember",
		Value:    "",
		Expires:  time.Now().Add(-24 * time.Hour),
		HTTPOnly: true,
		Secure:   config.Global.SessionSecure,
		SameSite: config.Global.SessionSameSite,
	}
	c.Cookie(cookie)

	return c.Redirect().To("/")
}