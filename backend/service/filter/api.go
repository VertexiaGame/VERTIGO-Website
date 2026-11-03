package filter

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL  = "https://api.typesafe.ai"
	defaultModel    = "jev-latest"
	defaultTimeout  = 20 * time.Second
	maxRequestBody  = 64 << 10
	loopbackAddress = "127.0.0.1:0"
)

type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
	Cache   Cache
}

type Service struct {
	endpoint string
	apiKey   string
	client   *http.Client
	server   *http.Server
}

type apiRequest struct {
	Content string `json:"content"`
}

type apiHandler struct {
	classifier *classifier
	key        string
	timeout    time.Duration
}

func Start(cfg Config) (*Service, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("TYPESAFE_API_KEY is not configured")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	key, err := randomKey()
	if err != nil {
		return nil, fmt.Errorf("generate filter API key: %w", err)
	}

	classifier := &classifier{
		endpoint: baseURL + "/v1/systemone",
		apiKey:   apiKey,
		model:    model,
		client:   &http.Client{Timeout: timeout},
		cache:    cfg.Cache,
	}

	listener, err := net.Listen("tcp", loopbackAddress)
	if err != nil {
		return nil, fmt.Errorf("listen on loopback: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/filter", apiHandler{
		classifier: classifier,
		key:        key,
		timeout:    timeout + 5*time.Second,
	})
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      timeout + 5*time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 14,
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Filter API stopped: %v", err)
		}
	}()

	return &Service{
		endpoint: "http://" + listener.Addr().String() + "/v1/filter",
		apiKey:   key,
		client:   &http.Client{Timeout: timeout + 5*time.Second},
		server:   server,
	}, nil
}

func (s *Service) Check(ctx context.Context, content string) (*Result, error) {
	if s == nil {
		return nil, ErrUnavailable
	}
	content, err := validateContent(content)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(apiRequest{Content: content})
	if err != nil {
		return nil, fmt.Errorf("encode filter request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build filter request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+s.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call filter API: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("filter API returned %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}

	var result Result
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode filter response: %w", err)
	}
	return &result, nil
}

func (s *Service) Close() error {
	if s == nil || s.server == nil {
		return nil
	}
	return s.server.Close()
}

func (h apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.authorized(r) {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var body apiRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	content, err := validateContent(body.Content)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	result, err := h.classifier.classify(ctx, content)
	if err != nil {
		log.Printf("Filter API classification failed: %v", err)
		writeAPIError(w, http.StatusBadGateway, "classification failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h apiHandler) authorized(r *http.Request) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(h.key)) == 1
}

func randomKey() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
