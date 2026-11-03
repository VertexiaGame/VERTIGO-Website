package filter

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type capturedRequest struct {
	authorization string
	request       systemOneRequest
}

func float64Ptr(value float64) *float64 {
	return &value
}

func answersFor(content string) map[string]answer {
	lower := strings.ToLower(content)
	values := map[string]float64{
		"profanity":      0.05,
		"sexual":         0.05,
		"discrimination": 0.05,
		"harassment":     0.05,
	}
	politics := 0.1

	switch {
	case strings.Contains(lower, "cunt"):
		values["profanity"] = 0.98
	case strings.Contains(lower, "nudes"):
		values["sexual"] = 0.95
	case strings.Contains(lower, "hate those people"):
		values["discrimination"] = 0.97
	case strings.Contains(lower, "kill yourself"):
		values["harassment"] = 0.96
	case strings.Contains(lower, "election"):
		politics = 2.4
	}

	answers := make(map[string]answer, 5)
	for name, value := range values {
		answers[name] = answer{Type: "noul", Noul: float64Ptr(value)}
	}
	answers["politics"] = answer{
		Type:       "score",
		Score:      float64Ptr(politics),
		Confidence: float64Ptr(0.9),
	}
	return answers
}

func newFakeTypeSafe(t *testing.T, captured chan<- capturedRequest) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		var request systemOneRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		captured <- capturedRequest{
			authorization: r.Header.Get("Authorization"),
			request:       request,
		}
		_ = json.NewEncoder(w).Encode(systemOneResponse{
			Model:   "jev-test",
			Answers: answersFor(request.State.Content),
			Usage:   Usage{InputTokens: 120, OutputTokens: 12},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func startTestService(t *testing.T, typeSafeURL string) *Service {
	t.Helper()
	service, err := Start(Config{
		APIKey:  "test-key",
		BaseURL: typeSafeURL,
		Model:   "jev-latest",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})
	return service
}

func TestStartBindsLoopbackAndSendsExpectedRequest(t *testing.T) {
	captured := make(chan capturedRequest, 1)
	typeSafe := newFakeTypeSafe(t, captured)
	service := startTestService(t, typeSafe.URL)

	parsed, err := url.Parse(service.endpoint)
	if err != nil {
		t.Fatalf("parse endpoint: %v", err)
	}
	host, _, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("split endpoint host: %v", err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		t.Fatalf("filter API is not bound to loopback: %s", host)
	}

	result, err := service.Check(context.Background(), "hello there")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.Blocked {
		t.Fatalf("expected content to be allowed, reasons: %v", result.Reasons)
	}
	if result.Model != "jev-test" {
		t.Fatalf("unexpected model: %s", result.Model)
	}

	request := <-captured
	if request.authorization != "Bearer test-key" {
		t.Fatalf("unexpected TypeSafe authorization header: %q", request.authorization)
	}
	if request.request.Model != "jev-latest" {
		t.Fatalf("unexpected model: %s", request.request.Model)
	}
	if request.request.State.Content != "hello there" {
		t.Fatalf("unexpected state content: %q", request.request.State.Content)
	}
	if request.request.State.Platform == "" {
		t.Fatal("state is missing platform context")
	}

	for _, name := range []string{"profanity", "sexual", "discrimination", "harassment", "politics"} {
		question, ok := request.request.Questions[name]
		if !ok {
			t.Fatalf("missing question %q", name)
		}
		if question.Instructions == "" {
			t.Fatalf("question %q has no instructions", name)
		}
	}
	if request.request.Questions["politics"].Type != "score" {
		t.Fatalf("politics question should be a score, got %q", request.request.Questions["politics"].Type)
	}
	if request.request.Questions["harassment"].Type != "noul" {
		t.Fatalf("harassment question should be a noul, got %q", request.request.Questions["harassment"].Type)
	}
}

func TestCheckDecisions(t *testing.T) {
	tests := []struct {
		name    string
		content string
		blocked bool
		reason  string
	}{
		{name: "mild trash talk", content: "you are so bad", blocked: false},
		{name: "casual profanity", content: "damn, that was a good match", blocked: false},
		{name: "excessive profanity", content: "you absolute cunt", blocked: true, reason: "Excessive profanity"},
		{name: "sexual content", content: "send nudes", blocked: true, reason: "Sexual content"},
		{name: "discrimination", content: "I hate those people", blocked: true, reason: "Discrimination or hate"},
		{name: "severe harassment", content: "kill yourself", blocked: true, reason: "Severe harassment"},
		{name: "politics", content: "the election is rigged", blocked: true, reason: "Politics"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			captured := make(chan capturedRequest, 1)
			typeSafe := newFakeTypeSafe(t, captured)
			service := startTestService(t, typeSafe.URL)

			result, err := service.Check(context.Background(), test.content)
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			<-captured

			if result.Blocked != test.blocked {
				t.Fatalf("blocked = %t, want %t (reasons: %v)", result.Blocked, test.blocked, result.Reasons)
			}
			if result.Action != ActionBlock && result.Blocked {
				t.Fatalf("action = %q for blocked content", result.Action)
			}
			if test.reason != "" {
				found := false
				for _, reason := range result.Reasons {
					if reason == test.reason {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("missing reason %q in %v", test.reason, result.Reasons)
				}
			}
			if len(result.Reasons) == 0 && result.Blocked {
				t.Fatal("blocked result has no reasons")
			}
		})
	}
}

func TestInternalAPIRejectsMissingOrWrongKey(t *testing.T) {
	captured := make(chan capturedRequest, 1)
	typeSafe := newFakeTypeSafe(t, captured)
	service := startTestService(t, typeSafe.URL)

	for name, header := range map[string]string{
		"missing": "",
		"wrong":   "Bearer wrong-key",
	} {
		t.Run(name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, service.endpoint, strings.NewReader(`{"content":"hello"}`))
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			request.Header.Set("Content-Type", "application/json")
			if header != "" {
				request.Header.Set("Authorization", header)
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
			}
		})
	}

	request, err := http.NewRequest(http.MethodPost, service.endpoint, strings.NewReader(`{"content":"hello"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+service.apiKey)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do authorized request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("authorized status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	<-captured
}

func TestCheckValidatesContent(t *testing.T) {
	service := &Service{}

	if _, err := service.Check(context.Background(), "   "); !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("empty content error = %v, want %v", err, ErrEmptyContent)
	}
	if _, err := service.Check(context.Background(), strings.Repeat("a", maxContentRunes+1)); !errors.Is(err, ErrContentTooLong) {
		t.Fatalf("long content error = %v, want %v", err, ErrContentTooLong)
	}
}

func TestStartRequiresAPIKey(t *testing.T) {
	if _, err := Start(Config{}); err == nil {
		t.Fatal("expected Start to fail without an API key")
	}
}

type memoryResultCache struct {
	mu      sync.Mutex
	entries map[string][]byte
	puts    int
}

func newMemoryResultCache() *memoryResultCache {
	return &memoryResultCache{entries: make(map[string][]byte)}
}

func (m *memoryResultCache) Get(ctx context.Context, key []byte) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.entries[string(key)]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), value...), true, nil
}

func (m *memoryResultCache) Put(ctx context.Context, key, result []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts++
	m.entries[string(key)] = append([]byte(nil), result...)
	return nil
}

func TestCheckCachesResults(t *testing.T) {
	var calls atomic.Int64
	typeSafe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request systemOneRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(systemOneResponse{
			Model:   "jev-test",
			Answers: answersFor(request.State.Content),
			Usage:   Usage{InputTokens: 120, OutputTokens: 12},
		})
	}))
	t.Cleanup(typeSafe.Close)

	cache := newMemoryResultCache()
	service, err := Start(Config{
		APIKey:  "test-key",
		BaseURL: typeSafe.URL,
		Model:   "jev-latest",
		Timeout: 5 * time.Second,
		Cache:   cache,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	first, err := service.Check(context.Background(), "send nudes")
	if err != nil {
		t.Fatalf("first Check: %v", err)
	}
	second, err := service.Check(context.Background(), "send nudes")
	if err != nil {
		t.Fatalf("second Check: %v", err)
	}

	if calls.Load() != 1 {
		t.Fatalf("TypeSafe calls = %d, want 1", calls.Load())
	}
	cache.mu.Lock()
	puts := cache.puts
	cache.mu.Unlock()
	if puts != 1 {
		t.Fatalf("cache puts = %d, want 1", puts)
	}
	if second.Blocked != first.Blocked || second.Action != first.Action || second.Model != first.Model {
		t.Fatalf("cached result decision mismatch: first=%+v second=%+v", first, second)
	}
	if strings.Join(second.Reasons, ",") != strings.Join(first.Reasons, ",") {
		t.Fatalf("cached reasons = %v, want %v", second.Reasons, first.Reasons)
	}
	for _, spec := range categorySpecs {
		firstCategory := first.Categories[spec.name]
		secondCategory := second.Categories[spec.name]
		if secondCategory.Value != firstCategory.Value || secondCategory.Triggered != firstCategory.Triggered {
			t.Fatalf("category %s mismatch: first=%+v second=%+v", spec.name, firstCategory, secondCategory)
		}
		if (firstCategory.Confidence == nil) != (secondCategory.Confidence == nil) {
			t.Fatalf("category %s confidence mismatch", spec.name)
		}
		if firstCategory.Confidence != nil && *firstCategory.Confidence != *secondCategory.Confidence {
			t.Fatalf("category %s confidence = %v, want %v", spec.name, *secondCategory.Confidence, *firstCategory.Confidence)
		}
	}

	if _, err := service.Check(context.Background(), "hello there"); err != nil {
		t.Fatalf("third Check: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("TypeSafe calls after new content = %d, want 2", calls.Load())
	}
}
