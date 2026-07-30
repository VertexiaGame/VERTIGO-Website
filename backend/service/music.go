package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"vertexia-frontend/backend/models"
)

type MusicService struct {
	client *http.Client
}

func NewMusicService() *MusicService {
	return &MusicService{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

var tagBracketRegex = regexp.MustCompile(`(?i)\b(artist|track|album|label):\s*\[([^\]]+)\]`)

func (s *MusicService) SearchTracks(query string) ([]models.DeezerTrack, error) {
	if query == "" {
		return []models.DeezerTrack{}, nil
	}

	formattedQuery := tagBracketRegex.ReplaceAllStringFunc(query, func(m string) string {
		match := tagBracketRegex.FindStringSubmatch(m)
		if len(match) == 3 {
			tagName := strings.ToLower(match[1])
			val := strings.TrimSpace(match[2])
			return fmt.Sprintf(`%s:"%s"`, tagName, val)
		}
		return m
	})

	apiURL := fmt.Sprintf("https://api.deezer.com/search?q=%s", url.QueryEscape(formattedQuery))
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer api returned status %d", resp.StatusCode)
	}

	var deezerResp models.DeezerSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&deezerResp); err != nil {
		return nil, err
	}

	for i := range deezerResp.Data {
		mins := deezerResp.Data[i].Duration / 60
		secs := deezerResp.Data[i].Duration % 60
		deezerResp.Data[i].Formatted = fmt.Sprintf("%d:%02d", mins, secs)
	}

	return deezerResp.Data, nil
}

func (s *MusicService) GetTrackByID(trackID int64) (*models.DeezerTrack, error) {
	if trackID <= 0 {
		return nil, errors.New("invalid track id")
	}

	apiURL := fmt.Sprintf("https://api.deezer.com/track/%d", trackID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer api returned status %d", resp.StatusCode)
	}

	var track models.DeezerTrack
	if err := json.NewDecoder(resp.Body).Decode(&track); err != nil {
		return nil, err
	}

	if track.ID == 0 || track.Title == "" {
		return nil, errors.New("track not found")
	}

	mins := track.Duration / 60
	secs := track.Duration % 60
	track.Formatted = fmt.Sprintf("%d:%02d", mins, secs)

	return &track, nil
}