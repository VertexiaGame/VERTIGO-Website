package gsmagent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef-test")

func signedRequest(t *testing.T, method, target string, body []byte, secret []byte, now time.Time) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(string(body)))
	Sign(req, body, secret, now)
	return req
}

func TestSignVerifyRoundTrip(t *testing.T) {
	now := time.Now()
	body := []byte(`{"server_id":"abcdEFGH","game_id":1,"max_players":10}`)
	req := signedRequest(t, http.MethodPost, "/v1/servers?x=1&y=2", body, testSecret, now)
	if got := req.Header.Get(HeaderNonce); len(got) != 32 {
		t.Fatalf("nonce must be 32 hex chars, got %q", got)
	}
	if sig := req.Header.Get(HeaderSignature); sig != strings.ToLower(sig) || len(sig) != 64 {
		t.Fatalf("signature must be lowercase hex sha256, got %q", sig)
	}
	if err := Verify(req, body, testSecret, NewMemoryNonceStore(), now.Add(5*time.Second)); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	now := time.Now()
	req := signedRequest(t, http.MethodGet, "/v1/servers", nil, testSecret, now)
	other := []byte("ffffffffffffffffffffffffffffffff-other")
	if err := Verify(req, nil, other, NewMemoryNonceStore(), now); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("expected ErrBadSignature, got %v", err)
	}
}

func TestVerifyTampered(t *testing.T) {
	now := time.Now()
	body := []byte(`{"game_id":1}`)
	t.Run("body", func(t *testing.T) {
		req := signedRequest(t, http.MethodPost, "/v1/servers", body, testSecret, now)
		if err := Verify(req, []byte(`{"game_id":2}`), testSecret, NewMemoryNonceStore(), now); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("expected ErrBadSignature, got %v", err)
		}
	})
	t.Run("path", func(t *testing.T) {
		req := signedRequest(t, http.MethodDelete, "/v1/servers/aaaaaaaa", nil, testSecret, now)
		forged := httptest.NewRequest(http.MethodDelete, "/v1/servers/bbbbbbbb", nil)
		forged.Header = req.Header.Clone()
		if err := Verify(forged, nil, testSecret, NewMemoryNonceStore(), now); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("expected ErrBadSignature, got %v", err)
		}
	})
	t.Run("query", func(t *testing.T) {
		req := signedRequest(t, http.MethodGet, "/v1/servers?a=1", nil, testSecret, now)
		forged := httptest.NewRequest(http.MethodGet, "/v1/servers?a=2", nil)
		forged.Header = req.Header.Clone()
		if err := Verify(forged, nil, testSecret, NewMemoryNonceStore(), now); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("expected ErrBadSignature, got %v", err)
		}
	})
	t.Run("method", func(t *testing.T) {
		req := signedRequest(t, http.MethodGet, "/v1/servers/aaaaaaaa", nil, testSecret, now)
		forged := httptest.NewRequest(http.MethodDelete, "/v1/servers/aaaaaaaa", nil)
		forged.Header = req.Header.Clone()
		if err := Verify(forged, nil, testSecret, NewMemoryNonceStore(), now); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("expected ErrBadSignature, got %v", err)
		}
	})
}

func TestVerifyClockSkew(t *testing.T) {
	now := time.Now()
	for _, offset := range []time.Duration{31 * time.Second, -31 * time.Second, 10 * time.Minute, -10 * time.Minute} {
		req := signedRequest(t, http.MethodGet, "/v1/health", nil, testSecret, now.Add(offset))
		if err := Verify(req, nil, testSecret, NewMemoryNonceStore(), now); !errors.Is(err, ErrClockSkew) {
			t.Fatalf("offset %v: expected ErrClockSkew, got %v", offset, err)
		}
	}
	for _, offset := range []time.Duration{29 * time.Second, -29 * time.Second} {
		req := signedRequest(t, http.MethodGet, "/v1/health", nil, testSecret, now.Add(offset))
		if err := Verify(req, nil, testSecret, NewMemoryNonceStore(), now); err != nil {
			t.Fatalf("offset %v within window rejected: %v", offset, err)
		}
	}
}

func TestVerifyReplay(t *testing.T) {
	now := time.Now()
	nonces := NewMemoryNonceStore()
	req := signedRequest(t, http.MethodGet, "/v1/servers", nil, testSecret, now)
	if err := Verify(req, nil, testSecret, nonces, now); err != nil {
		t.Fatal(err)
	}
	replay := httptest.NewRequest(http.MethodGet, "/v1/servers", nil)
	replay.Header = req.Header.Clone()
	if err := Verify(replay, nil, testSecret, nonces, now.Add(time.Second)); !errors.Is(err, ErrReplay) {
		t.Fatalf("expected ErrReplay, got %v", err)
	}
}

func TestVerifyMissingHeaders(t *testing.T) {
	now := time.Now()
	for _, h := range []string{HeaderTimestamp, HeaderNonce, HeaderSignature} {
		req := signedRequest(t, http.MethodGet, "/v1/servers", nil, testSecret, now)
		req.Header.Del(h)
		if err := Verify(req, nil, testSecret, NewMemoryNonceStore(), now); !errors.Is(err, ErrMissingAuth) {
			t.Fatalf("missing %s: expected ErrMissingAuth, got %v", h, err)
		}
	}
	bare := httptest.NewRequest(http.MethodGet, "/v1/servers", nil)
	if err := Verify(bare, nil, testSecret, NewMemoryNonceStore(), now); !errors.Is(err, ErrMissingAuth) {
		t.Fatalf("expected ErrMissingAuth, got %v", err)
	}
}

func TestVerifyMalformed(t *testing.T) {
	now := time.Now()
	req := signedRequest(t, http.MethodGet, "/v1/servers", nil, testSecret, now)
	req.Header.Set(HeaderNonce, "not-hex")
	if err := Verify(req, nil, testSecret, NewMemoryNonceStore(), now); !errors.Is(err, ErrBadNonce) {
		t.Fatalf("expected ErrBadNonce, got %v", err)
	}
	req = signedRequest(t, http.MethodGet, "/v1/servers", nil, testSecret, now)
	req.Header.Set(HeaderTimestamp, "12abc")
	if err := Verify(req, nil, testSecret, NewMemoryNonceStore(), now); !errors.Is(err, ErrBadTimestamp) {
		t.Fatalf("expected ErrBadTimestamp, got %v", err)
	}
	req = signedRequest(t, http.MethodGet, "/v1/servers", nil, testSecret, now)
	if err := Verify(req, nil, nil, NewMemoryNonceStore(), now); !errors.Is(err, ErrNoSecret) {
		t.Fatalf("expected ErrNoSecret, got %v", err)
	}
}

func TestMemoryNonceExpiry(t *testing.T) {
	s := NewMemoryNonceStore()
	base := time.Now()
	s.now = func() time.Time { return base }
	ctx := context.Background()
	if ok, _ := s.Remember(ctx, "n1", time.Minute); !ok {
		t.Fatal("first use must be fresh")
	}
	if ok, _ := s.Remember(ctx, "n1", time.Minute); ok {
		t.Fatal("second use must be rejected")
	}
	s.now = func() time.Time { return base.Add(2 * time.Minute) }
	if ok, _ := s.Remember(ctx, "n1", time.Minute); !ok {
		t.Fatal("expired nonce should be accepted again")
	}
}

func TestMiddlewareAndClient(t *testing.T) {
	var gotBody CreateServerRequest
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/servers", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Server{ID: gotBody.ServerID, Port: 7001, Status: StatusRunning, GameID: gotBody.GameID})
	})
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Health{OK: true, Capacity: 3, Version: "test"})
	})
	srv := httptest.NewServer(Middleware(testSecret, NewMemoryNonceStore(), nil, mux))
	defer srv.Close()

	c, err := NewClient(srv.URL, testSecret, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	out, err := c.CreateServer(ctx, CreateServerRequest{ServerID: "abcdEFGH", GameID: 4, MaxPlayers: 8})
	if err != nil {
		t.Fatal(err)
	}
	if out.Port != 7001 || gotBody.GameID != 4 {
		t.Fatalf("unexpected round trip: %+v %+v", out, gotBody)
	}
	h, err := c.Health(ctx)
	if err != nil || !h.OK || h.Capacity != 3 {
		t.Fatalf("health: %+v %v", h, err)
	}

	bad, err := NewClient(srv.URL, []byte("ffffffffffffffffffffffffffffffff-wrong"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bad.Health(ctx); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	resp, err := http.Get(srv.URL + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unsigned request: expected 401, got %d", resp.StatusCode)
	}
}

func TestNewClientValidation(t *testing.T) {
	if _, err := NewClient("ftp://x", testSecret, time.Second); err == nil {
		t.Fatal("non-http scheme must be rejected")
	}
	if _, err := NewClient("https://agent.example", []byte("short"), time.Second); err == nil {
		t.Fatal("short secret must be rejected")
	}
}
