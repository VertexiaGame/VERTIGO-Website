package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vertexia-frontend/backend/gsmagent"
)

func TestPortAllocator(t *testing.T) {
	p := newPortAllocator(7000, 7002)
	if p.Free() != 3 {
		t.Fatalf("expected 3 free, got %d", p.Free())
	}
	a, ok := p.Claim("a")
	b, ok2 := p.Claim("b")
	c, ok3 := p.Claim("c")
	if !ok || !ok2 || !ok3 || a != 7000 || b != 7001 || c != 7002 {
		t.Fatalf("unexpected claims %d %d %d", a, b, c)
	}
	if _, ok := p.Claim("d"); ok {
		t.Fatal("exhausted range must refuse")
	}
	p.Release(7001, "someone-else")
	if p.Free() != 0 {
		t.Fatal("release by a non-owner must be ignored")
	}
	p.Release(7001, "b")
	if got, ok := p.Claim("d"); !ok || got != 7001 {
		t.Fatalf("expected reuse of 7001, got %d %v", got, ok)
	}
	if p.ClaimSpecific(6999, "x") || p.ClaimSpecific(7003, "x") {
		t.Fatal("out-of-range ports must be refused")
	}
	if p.ClaimSpecific(7000, "x") {
		t.Fatal("port held by another id must be refused")
	}
	if !p.ClaimSpecific(7000, "a") {
		t.Fatal("re-claiming own port must succeed")
	}

	q := newPortAllocator(8000, 8002)
	q.probe = func(port int) bool { return port != 8000 }
	if got, ok := q.Claim("a"); !ok || got != 8001 {
		t.Fatalf("probe must skip busy port, got %d", got)
	}
}

func TestValidateCreate(t *testing.T) {
	mapDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mapDir, "arena"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mapDir, "arena", "one.vrtx"), []byte("map"), 0644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.vrtx")
	if err := os.WriteFile(outside, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(mapDir, "escape.vrtx")); err != nil {
		t.Fatal(err)
	}
	ok := gsmagent.CreateServerRequest{ServerID: "abcd1234", GameID: 3, MaxPlayers: 20}
	if got, err := validateCreate(ok, "default.vrtx", mapDir); err != nil || got != "default.vrtx" {
		t.Fatalf("default map: %q %v", got, err)
	}
	withMap := ok
	withMap.Map = "arena/one.vrtx"
	got, err := validateCreate(withMap, "default.vrtx", mapDir)
	if err != nil || !strings.HasSuffix(got, filepath.Join("arena", "one.vrtx")) {
		t.Fatalf("custom map: %q %v", got, err)
	}
	if _, err := validateCreate(withMap, "default.vrtx", ""); err == nil {
		t.Fatal("custom map without GSM_MAP_DIR must be rejected")
	}
	cases := map[string]gsmagent.CreateServerRequest{
		"short id":      {ServerID: "abc", GameID: 1, MaxPlayers: 1},
		"bad id chars":  {ServerID: "abcd-123", GameID: 1, MaxPlayers: 1},
		"zero game":     {ServerID: "abcd1234", GameID: 0, MaxPlayers: 1},
		"negative game": {ServerID: "abcd1234", GameID: -4, MaxPlayers: 1},
		"zero players":  {ServerID: "abcd1234", GameID: 1, MaxPlayers: 0},
		"too many":      {ServerID: "abcd1234", GameID: 1, MaxPlayers: 1001},
		"dotdot":        {ServerID: "abcd1234", GameID: 1, MaxPlayers: 1, Map: "../etc/passwd"},
		"inner dotdot":  {ServerID: "abcd1234", GameID: 1, MaxPlayers: 1, Map: "arena/../arena/one.vrtx"},
		"absolute":      {ServerID: "abcd1234", GameID: 1, MaxPlayers: 1, Map: "/etc/passwd"},
		"unclean":       {ServerID: "abcd1234", GameID: 1, MaxPlayers: 1, Map: "arena//one.vrtx"},
		"dot prefix":    {ServerID: "abcd1234", GameID: 1, MaxPlayers: 1, Map: "./arena/one.vrtx"},
		"bad chars":     {ServerID: "abcd1234", GameID: 1, MaxPlayers: 1, Map: "arena/one vrtx"},
		"missing":       {ServerID: "abcd1234", GameID: 1, MaxPlayers: 1, Map: "arena/none.vrtx"},
		"directory":     {ServerID: "abcd1234", GameID: 1, MaxPlayers: 1, Map: "arena"},
		"symlink out":   {ServerID: "abcd1234", GameID: 1, MaxPlayers: 1, Map: "escape.vrtx"},
	}
	for name, req := range cases {
		if _, err := validateCreate(req, "default.vrtx", mapDir); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
	edge := ok
	edge.MaxPlayers = 1000
	if _, err := validateCreate(edge, "default.vrtx", mapDir); err != nil {
		t.Fatalf("max_players 1000 must be accepted: %v", err)
	}
}

func TestDecodeCreateStrict(t *testing.T) {
	if _, err := decodeCreate([]byte(`{"server_id":"abcd1234","game_id":1,"max_players":2}`)); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"server_id":"abcd1234","game_id":1,"max_players":2,"bin":"/bin/sh"}`,
		`{"server_id":"abcd1234","game_id":1,"max_players":2}{}`,
		`{"server_id":"abcd1234","game_id":"1","max_players":2}`,
		``,
	} {
		if _, err := decodeCreate([]byte(body)); err == nil {
			t.Errorf("body %q must be rejected", body)
		}
	}
}

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"GSM_AGENT_SECRET":     strings.Repeat("s", 40),
		"GSM_PORT_RANGE":       "7000-7099",
		"GSM_SERVER_BIN":       "/opt/vertexia/RaveEngineServer",
		"GSM_AGENT_TLS_CERT":   "/etc/vertexia/agent.crt",
		"GSM_AGENT_TLS_KEY":    "/etc/vertexia/agent.key",
		"NETCODE_PRIVATE_KEY":  strings.Repeat("0f", 32),
		"NETCODE_PROTOCOL_ID":  "123",
		"GAMESERVER_API_KEY":   strings.Repeat("k", 40),
		"REDIS_URL":            "rediss://default:pw@redis.example:6380/0",
		"GSM_AGENT_ID":         "games-1",
		"GSM_SERVER_LOGS_DIR":  "/var/log/gsm-agent",
		"GSM_MAX_RESTARTS":     "3",
		"GSM_READY_TIMEOUT":    "30s",
		"GSM_SHUTDOWN_GRACE":   "10s",
		"GSM_AGENT_ALLOW_HTTP": "",
	}
}

func TestLoadConfig(t *testing.T) {
	cfg, err := loadConfig(envMap(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":8443" || cfg.PortMin != 7000 || cfg.PortMax != 7099 || cfg.BindHost != "0.0.0.0" || cfg.APIDomain != "https://vertexia.wempos-forum.com" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	env := strings.Join(cfg.childEnv(), "\n")
	if strings.Contains(env, "GSM_AGENT_SECRET") || strings.Contains(env, "REDIS") || strings.Contains(env, strings.Repeat("s", 40)) {
		t.Fatalf("child env leaks secrets: %s", env)
	}

	mutate := func(k, v string) map[string]string {
		m := validEnv()
		m[k] = v
		return m
	}
	bad := map[string]map[string]string{
		"missing secret":   mutate("GSM_AGENT_SECRET", ""),
		"short secret":     mutate("GSM_AGENT_SECRET", "short"),
		"no port range":    mutate("GSM_PORT_RANGE", ""),
		"bad port range":   mutate("GSM_PORT_RANGE", "7099-7000"),
		"no netcode key":   mutate("NETCODE_PRIVATE_KEY", ""),
		"bad netcode key":  mutate("NETCODE_PRIVATE_KEY", "zz"),
		"no protocol id":   mutate("NETCODE_PROTOCOL_ID", ""),
		"no api key":       mutate("GAMESERVER_API_KEY", ""),
		"no redis":         mutate("REDIS_URL", ""),
		"relative bin":     mutate("GSM_SERVER_BIN", "server"),
		"bad agent id":     mutate("GSM_AGENT_ID", "bad id!"),
		"cert without key": mutate("GSM_AGENT_TLS_KEY", ""),
		"bad duration":     mutate("GSM_READY_TIMEOUT", "soon"),
	}
	for name, m := range bad {
		if _, err := loadConfig(envMap(m)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}

	plain := validEnv()
	delete(plain, "GSM_AGENT_TLS_CERT")
	delete(plain, "GSM_AGENT_TLS_KEY")
	if _, err := loadConfig(envMap(plain)); err == nil {
		t.Fatal("plain HTTP without opt-in must be rejected")
	}
	plain["GSM_AGENT_ALLOW_HTTP"] = "true"
	if _, err := loadConfig(envMap(plain)); err == nil {
		t.Fatal("plain HTTP on a public listen address must be rejected")
	}
	plain["GSM_AGENT_LISTEN"] = "0.0.0.0:8080"
	if _, err := loadConfig(envMap(plain)); err == nil {
		t.Fatal("plain HTTP on 0.0.0.0 must be rejected")
	}
	plain["GSM_AGENT_LISTEN"] = "127.0.0.1:8080"
	if _, err := loadConfig(envMap(plain)); err != nil {
		t.Fatalf("loopback plain HTTP behind a tunnel must be allowed: %v", err)
	}
	plain["GSM_AGENT_LISTEN"] = "[::1]:8080"
	if _, err := loadConfig(envMap(plain)); err != nil {
		t.Fatalf("IPv6 loopback must be allowed: %v", err)
	}
}

func TestPidMatches(t *testing.T) {
	if pidMatches(0, "abcd1234") || pidMatches(-1, "abcd1234") {
		t.Fatal("non-positive pids never match")
	}
	if _, err := os.Stat("/proc/self/cmdline"); err != nil {
		t.Skip("no /proc")
	}
	if pidMatches(os.Getpid(), "definitely-not-an-arg") {
		t.Fatal("unrelated id must not match")
	}
}
