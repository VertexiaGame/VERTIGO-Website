package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"vertexia-frontend/backend/gsmagent"
	"vertexia-frontend/backend/service/gsm"
)

const maxMapLen = 255

// decodeCreate parses a CreateServerRequest strictly: unknown fields and
// trailing data are rejected.
func decodeCreate(body []byte) (gsmagent.CreateServerRequest, error) {
	var req gsmagent.CreateServerRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, fmt.Errorf("invalid JSON body: %v", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return req, errors.New("invalid JSON body: trailing data")
	}
	return req, nil
}

// validateCreate checks a create request and returns the map path to pass
// to the gameserver.
func validateCreate(req gsmagent.CreateServerRequest, defaultMap, mapDir string) (string, error) {
	if !gsm.ValidID(req.ServerID) {
		return "", errors.New("server_id must be 8 alphanumeric characters")
	}
	if req.GameID <= 0 {
		return "", errors.New("game_id must be a positive integer")
	}
	if req.MaxPlayers < gsmagent.MinMaxPlayers || req.MaxPlayers > gsmagent.MaxMaxPlayers {
		return "", fmt.Errorf("max_players must be between %d and %d", gsmagent.MinMaxPlayers, gsmagent.MaxMaxPlayers)
	}
	if req.Map == "" {
		if defaultMap == "" {
			return "", errors.New("no map requested and no default map configured")
		}
		return defaultMap, nil
	}
	return resolveMap(req.Map, mapDir)
}

func validMapChars(m string) bool {
	for i := 0; i < len(m); i++ {
		c := m[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' || c == '/') {
			return false
		}
	}
	return true
}

// resolveMap accepts only a clean relative path under mapDir that names an
// existing regular file, also after resolving symlinks.
func resolveMap(m, mapDir string) (string, error) {
	if mapDir == "" {
		return "", errors.New("custom maps are disabled (GSM_MAP_DIR is not set)")
	}
	if len(m) > maxMapLen || !validMapChars(m) {
		return "", errors.New("map contains invalid characters")
	}
	if strings.Contains(m, "..") || strings.HasPrefix(m, "/") || filepath.Clean(m) != m || !filepath.IsLocal(m) {
		return "", errors.New("map must be a clean relative path inside the map directory")
	}
	root, err := filepath.EvalSymlinks(mapDir)
	if err != nil {
		return "", errors.New("map directory is unavailable")
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, m))
	if err != nil {
		return "", errors.New("map not found")
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || !filepath.IsLocal(rel) {
		return "", errors.New("map must be inside the map directory")
	}
	st, err := os.Stat(full)
	if err != nil || !st.Mode().IsRegular() {
		return "", errors.New("map not found")
	}
	return full, nil
}
