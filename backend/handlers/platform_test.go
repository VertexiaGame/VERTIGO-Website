package handlers

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/service/gsm"
)

func TestJoinFailureMapsContractStatuses(t *testing.T) {
	cases := []struct {
		err    error
		status int
		body   string
	}{
		{gsm.ErrGameMissing, fiber.StatusNotFound, "not_found"},
		{gsm.ErrGameClosed, fiber.StatusNotFound, "not_found"},
		{gsm.ErrNoCapacity, fiber.StatusConflict, "full"},
		{gsm.ErrFull, fiber.StatusConflict, "full"},
		{fmt.Errorf("%w: agent said no", gsm.ErrNoCapacity), fiber.StatusConflict, "full"},
		{gsm.ErrLaunch, fiber.StatusServiceUnavailable, "unavailable"},
		{fmt.Errorf("%w: not ready in time", gsm.ErrLaunch), fiber.StatusServiceUnavailable, "unavailable"},
		{gsm.ErrUnavailable, fiber.StatusServiceUnavailable, "unavailable"},
		{errors.New("redis down"), fiber.StatusServiceUnavailable, "unavailable"},
	}
	for _, tc := range cases {
		status, body := joinFailure(tc.err)
		if status != tc.status || body["error"] != tc.body {
			t.Errorf("joinFailure(%v) = %d %v; want %d %s", tc.err, status, body, tc.status, tc.body)
		}
	}
}

func TestWriteJoinFailureSetsRetryAfter(t *testing.T) {
	app := fiber.New()
	app.Post("/x", func(c fiber.Ctx) error { return writeJoinFailure(c, gsm.ErrLaunch) })
	app.Post("/y", func(c fiber.Ctx) error { return writeJoinFailure(c, gsm.ErrNoCapacity) })
	res, err := app.Test(httptest.NewRequest("POST", "/x", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusServiceUnavailable || res.Header.Get("Retry-After") == "" {
		t.Fatalf("503 must carry Retry-After: %d %q", res.StatusCode, res.Header.Get("Retry-After"))
	}
	res, err = app.Test(httptest.NewRequest("POST", "/y", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusConflict {
		t.Fatalf("capacity exhaustion must be 409, got %d", res.StatusCode)
	}
}

// fakeLauncher simulates local gameserver processes so the join capacity
// logic can be exercised without binaries.
type fakeLauncher struct{ nextPID int }

func (f *fakeLauncher) Launch(id string, gameID int, port int) (int, error) {
	f.nextPID++
	return f.nextPID, nil
}
func (f *fakeLauncher) WaitHealthy(string, int, int, time.Duration) error { return nil }
func (f *fakeLauncher) Alive(int, int) bool                               { return true }
func (f *fakeLauncher) Kill(int, int) error                               { return nil }

func TestJoinCapacityFlowsToConflict(t *testing.T) {
	cfg := gsm.DefaultConfig()
	cfg.PortMin, cfg.PortMax = 7000, 7000 // room for exactly one server
	cfg.MinPlayersDefault, cfg.MaxPlayersDefault = 2, 2
	m := gsm.NewManager(cfg, gsm.NewMemoryStore(), &fakeLauncher{}, func(id int) (gsm.GameInfo, error) {
		return gsm.GameInfo{ID: id, MaxPlayers: 2}, nil
	})
	first, _, err := m.Join(1, "alice")
	if err != nil {
		t.Fatalf("first join: %v", err)
	}
	second, redirected, err := m.Join(1, "bob")
	if err != nil || !redirected || second.ID != first.ID {
		t.Fatalf("second player should share the server: %+v %v %v", second, redirected, err)
	}
	_, _, err = m.Join(1, "carol")
	if status, body := joinFailure(err); status != fiber.StatusConflict || body["error"] != "full" {
		t.Fatalf("third player with no ports left: err=%v status=%d body=%v", err, status, body)
	}
}

func TestGameserverAddrsPrefersOverrideAndRejectsBadPorts(t *testing.T) {
	cfg := &config.Config{GSMPublicHost: "games.vertexia.wempos-forum.com", GSMPublicIP: "203.0.113.7, 2001:db8::7"}
	addrs, err := gameserverAddrs(cfg, 7001)
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.AddrPort{netip.MustParseAddrPort("203.0.113.7:7001"), netip.MustParseAddrPort("[2001:db8::7]:7001")}
	if len(addrs) != 2 || addrs[0] != want[0] || addrs[1] != want[1] {
		t.Fatalf("addrs = %v", addrs)
	}
	if _, err := gameserverAddrs(cfg, 0); err == nil {
		t.Fatal("port 0 must be rejected")
	}
	if _, err := gameserverAddrs(&config.Config{GSMPublicIP: "not-an-ip"}, 7001); err == nil {
		t.Fatal("invalid GSM_PUBLIC_IP must be rejected")
	}
}

func TestCSRFProtect(t *testing.T) {
	prev := config.Global
	config.Global = &config.Config{PublicBaseURL: "https://vertexia.wempos-forum.com"}
	defer func() { config.Global = prev }()

	app := fiber.New()
	app.Use(CSRFProtect())
	app.Post("/settings/bio", func(c fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/settings", func(c fiber.Ctx) error { return c.SendString("ok") })

	cases := []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{"safe method", "GET", map[string]string{"Origin": "https://evil.example"}, 200},
		{"same origin fetch metadata", "POST", map[string]string{"Sec-Fetch-Site": "same-origin"}, 200},
		{"cross site fetch metadata", "POST", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"sibling subdomain", "POST", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
		{"public origin", "POST", map[string]string{"Origin": "https://vertexia.wempos-forum.com"}, 200},
		{"foreign origin", "POST", map[string]string{"Origin": "https://evil.example"}, 403},
		{"null origin", "POST", map[string]string{"Origin": "null"}, 403},
		{"foreign referer", "POST", map[string]string{"Referer": "https://evil.example/page"}, 403},
		{"own referer", "POST", map[string]string{"Referer": "https://vertexia.wempos-forum.com/settings"}, 200},
		{"bearer client", "POST", map[string]string{"Authorization": "Bearer vxa_x", "Origin": "https://evil.example"}, 200},
		{"native client without browser headers", "POST", nil, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := "/settings/bio"
			if tc.method == "GET" {
				path = "/settings"
			}
			req := httptest.NewRequest(tc.method, path, nil)
			req.Host = "vertexia.wempos-forum.com"
			req.Header.Set("Cookie", "session_id=abc")
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", res.StatusCode, tc.want)
			}
		})
	}
}

func TestStudioRedirectURIValidation(t *testing.T) {
	good := []string{"http://127.0.0.1:53111/callback", "http://localhost:8080/cb", "http://[::1]:9000/x", "vertexia-studio://auth", "vertexia://auth"}
	bad := []string{"", "https://evil.example/cb", "http://localhost:1@evil.example/", "http://localhost/cb", "http://127.0.0.1.evil.example:80/", "javascript:alert(1)", "http://127.0.0.1:80/#frag", "http://127.0.0.1:80/\r\nx"}
	for _, u := range good {
		if !isValidStudioRedirectURI(u) {
			t.Errorf("rejected valid redirect %q", u)
		}
	}
	for _, u := range bad {
		if isValidStudioRedirectURI(u) {
			t.Errorf("accepted unsafe redirect %q", u)
		}
	}
}

func TestGameserverKeyHeadersOnly(t *testing.T) {
	prev := config.Global
	config.Global = &config.Config{GameserverAPIKey: "gs-secret"}
	defer func() { config.Global = prev }()

	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error {
		if gameserverKeyValid(c) {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusUnauthorized)
	})
	cases := []struct {
		name    string
		path    string
		headers map[string]string
		want    int
	}{
		{"header", "/x", map[string]string{"X-Gameserver-Key": "gs-secret"}, 200},
		{"bearer", "/x", map[string]string{"Authorization": "Bearer gs-secret"}, 200},
		{"wrong header", "/x", map[string]string{"X-Gameserver-Key": "nope"}, 401},
		{"query gameserver_key", "/x?gameserver_key=gs-secret", nil, 401},
		{"query api_key", "/x?api_key=gs-secret", nil, 401},
		{"access token is not a server key", "/x", map[string]string{"Authorization": "Bearer vxa_gs-secret"}, 401},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", res.StatusCode, tc.want)
			}
		})
	}
}

func TestWebSocketOriginAllowed(t *testing.T) {
	prev := config.Global
	config.Global = &config.Config{PublicBaseURL: "https://vertexia.wempos-forum.com"}
	defer func() { config.Global = prev }()

	app := fiber.New()
	app.Get("/ws", func(c fiber.Ctx) error {
		if WebSocketOriginAllowed(c) {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusForbidden)
	})
	for origin, want := range map[string]int{
		"":                                      200,
		"https://vertexia.wempos-forum.com":     200,
		"https://evil.example":                  403,
		"https://cdn.vertexia.wempos-forum.com": 403,
	} {
		req := httptest.NewRequest("GET", "/ws", nil)
		req.Host = "internal:8080"
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Errorf("origin %q: status = %d, want %d", origin, res.StatusCode, want)
		}
	}
}
