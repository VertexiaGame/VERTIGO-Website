package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                int
	Username          string
	DisplayName       string
	Mail              string
	Password          string
	Description       string
	Unikey            string
	Power             int
	PrimaryClan       int
	NameColor         string
	CustomCSS         sql.NullString
	Vermail           string
	Vermc             string
	Casom             string
	Bits              int
	Bucks             int
	LastOnline        time.Time
	CreationDate      time.Time
	Views             int
	FeedCaptchas      int
	EmailVerifyToken  sql.NullString
	EmailVerifyExpiry sql.NullInt64
}

type ReplayCache struct {
	mu     sync.RWMutex
	nonces map[string]time.Time
}

func NewReplayCache() *ReplayCache {
	rc := &ReplayCache{
		nonces: make(map[string]time.Time),
	}
	go rc.cleanupLoop()
	return rc
}

func (rc *ReplayCache) Add(nonce string, expiry time.Time) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if _, exists := rc.nonces[nonce]; exists {
		return false
	}
	rc.nonces[nonce] = expiry
	return true
}

func (rc *ReplayCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		rc.mu.Lock()
		now := time.Now()
		for nonce, expiry := range rc.nonces {
			if now.After(expiry) {
				delete(rc.nonces, nonce)
			}
		}
		rc.mu.Unlock()
	}
}

func GenerateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}

func CreateUser(db *sql.DB, username, displayname, email, password string) (*User, error) {
	if db == nil {
		mockUser := &User{
			Username:    username,
			DisplayName: displayname,
			Mail:        email,
		}
		return mockUser, nil
	}

	existingUser, err := GetUserByUsername(db, username)
	if err != nil {
		return nil, err
	}
	if existingUser != nil {
		return nil, errors.New("username is already taken")
	}

	existingEmail, err := GetUserByEmail(db, email)
	if err != nil {
		return nil, err
	}
	if existingEmail != nil {
		return nil, errors.New("email is already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	unikey, err := GenerateRandomString(25)
	if err != nil {
		return nil, err
	}

	var id int
	idBytes := make([]byte, 4)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}
	id = int(binary.BigEndian.Uint32(idBytes) & 0x7FFFFFFF)

	if displayname == "" {
		displayname = username
	}

	user := &User{
		ID:           id,
		Username:     username,
		DisplayName:  displayname,
		Mail:         email,
		Password:     string(hashedPassword),
		Description:  "I am new to VERTEXIA!",
		Unikey:       unikey,
		Power:        0,
		PrimaryClan:  0,
		NameColor:    "7423CB",
		Vermail:      "false",
		Vermc:        "false",
		Casom:        "false",
		Bits:         250,
		Bucks:        100,
		LastOnline:   time.Now(),
		CreationDate: time.Now(),
		Views:        0,
		FeedCaptchas: 0,
	}

	query := "INSERT INTO users (id, username, displayname, mail, password, description, unikey, power, primary_clan, namecolor, custom_css, vermail, vermc, casom, bits, bucks, last_online, creation_date, views, feed_captchas) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err = db.Exec(query,
		user.ID,
		user.Username,
		user.DisplayName,
		user.Mail,
		user.Password,
		user.Description,
		user.Unikey,
		user.Power,
		user.PrimaryClan,
		user.NameColor,
		user.CustomCSS,
		user.Vermail,
		user.Vermc,
		user.Casom,
		user.Bits,
		user.Bucks,
		user.LastOnline,
		user.CreationDate,
		user.Views,
		user.FeedCaptchas,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	if db == nil {
		return nil, errors.New("database connection is offline")
	}

	query := "SELECT id, username, displayname, mail, password, description, unikey, power, primary_clan, namecolor, custom_css, vermail, vermc, casom, bits, bucks, last_online, creation_date, views, feed_captchas, email_verify_token, email_verify_expiry FROM users WHERE username = ?"
	row := db.QueryRow(query, username)

	var u User
	err := row.Scan(
		&u.ID,
		&u.Username,
		&u.DisplayName,
		&u.Mail,
		&u.Password,
		&u.Description,
		&u.Unikey,
		&u.Power,
		&u.PrimaryClan,
		&u.NameColor,
		&u.CustomCSS,
		&u.Vermail,
		&u.Vermc,
		&u.Casom,
		&u.Bits,
		&u.Bucks,
		&u.LastOnline,
		&u.CreationDate,
		&u.Views,
		&u.FeedCaptchas,
		&u.EmailVerifyToken,
		&u.EmailVerifyExpiry,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &u, nil
}

func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	if db == nil {
		return nil, errors.New("database connection is offline")
	}

	query := "SELECT id, username, displayname, mail, password, description, unikey, power, primary_clan, namecolor, custom_css, vermail, vermc, casom, bits, bucks, last_online, creation_date, views, feed_captchas, email_verify_token, email_verify_expiry FROM users WHERE mail = ?"
	row := db.QueryRow(query, email)

	var u User
	err := row.Scan(
		&u.ID,
		&u.Username,
		&u.DisplayName,
		&u.Mail,
		&u.Password,
		&u.Description,
		&u.Unikey,
		&u.Power,
		&u.PrimaryClan,
		&u.NameColor,
		&u.CustomCSS,
		&u.Vermail,
		&u.Vermc,
		&u.Casom,
		&u.Bits,
		&u.Bucks,
		&u.LastOnline,
		&u.CreationDate,
		&u.Views,
		&u.FeedCaptchas,
		&u.EmailVerifyToken,
		&u.EmailVerifyExpiry,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &u, nil
}

func AuthenticateUser(db *sql.DB, identifier, password string) (*User, error) {
	var user *User
	var err error

	if strings.Contains(identifier, "@") {
		user, err = GetUserByEmail(db, identifier)
	} else {
		user, err = GetUserByUsername(db, identifier)
	}

	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("invalid username/email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, errors.New("invalid username/email or password")
	}

	return user, nil
}

func UpdateUserOnline(db *sql.DB, userID int) error {
	if db == nil {
		return nil
	}
	query := "UPDATE users SET last_online = ? WHERE id = ?"
	_, err := db.Exec(query, time.Now(), userID)
	return err
}