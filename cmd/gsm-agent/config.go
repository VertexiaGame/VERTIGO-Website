package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/gsmagent"
	"vertexia-frontend/backend/kv"
)

const (
	defaultListen    = ":8443"
	defaultAPIDomain = "https://vertexia.wempos-forum.com"
	defaultMap       = "assets/maps/default.vrtx"
	defaultLogsDir   = "/var/log/gsm-agent"
	defaultPath      = "/usr/local/bin:/usr/bin:/bin"
)

type agentConfig struct {
	Listen    string
	Secret    string
	TLSCert   string
	TLSKey    string
	AllowHTTP bool
	AgentID   string

	PortMin       int
	PortMax       int
	ServerBin     string
	ServerWorkdir string
	ServerMap     string
	MapDir        string
	LogsDir       string
	BindHost      string

	ReadyTimeout  time.Duration
	ShutdownGrace time.Duration
	MaxRestarts   int
	RestartWindow time.Duration
	MaxServers    int

	Redis kv.Options

	NetcodeKeyHex     string
	NetcodeProtocolID string
	APIKey            string
	APIDomain         string
	RustLog           string
	Path              string
	Home              string
}

func envDur(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration such as 30s", key)
	}
	return d, nil
}

func envNonNegInt(getenv func(string) string, key string, def int) (int, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return n, nil
}

func envBoolValue(getenv func(string) string, key string) (bool, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return b, nil
}

func validAgentID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func sanitizeAgentID(raw string) string {
	var b strings.Builder
	for i := 0; i < len(raw) && b.Len() < 64; i++ {
		c := raw[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' {
			b.WriteByte(c)
		} else {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "gsm-agent"
	}
	return b.String()
}

// isLoopbackListen reports whether addr only listens on a loopback address.
func isLoopbackListen(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// loadConfig reads and validates the agent environment. Every problem is
// reported at once.
func loadConfig(getenv func(string) string) (agentConfig, error) {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	str := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}
	cfg := agentConfig{
		Listen:            str("GSM_AGENT_LISTEN", defaultListen),
		Secret:            strings.TrimSpace(getenv("GSM_AGENT_SECRET")),
		TLSCert:           str("GSM_AGENT_TLS_CERT", ""),
		TLSKey:            str("GSM_AGENT_TLS_KEY", ""),
		ServerBin:         str("GSM_SERVER_BIN", ""),
		ServerWorkdir:     str("GSM_SERVER_WORKDIR", ""),
		ServerMap:         str("GSM_SERVER_MAP", defaultMap),
		MapDir:            str("GSM_MAP_DIR", ""),
		LogsDir:           str("GSM_SERVER_LOGS_DIR", defaultLogsDir),
		BindHost:          str("GSM_BIND_HOST", "0.0.0.0"),
		NetcodeKeyHex:     strings.ToLower(str("NETCODE_PRIVATE_KEY", "")),
		NetcodeProtocolID: str("NETCODE_PROTOCOL_ID", ""),
		APIKey:            strings.TrimSpace(getenv("GAMESERVER_API_KEY")),
		APIDomain:         strings.TrimSuffix(str("VERTIGO_API_DOMAIN", defaultAPIDomain), "/"),
		RustLog:           strings.TrimSpace(getenv("RUST_LOG")),
		Path:              str("PATH", defaultPath),
		Redis: kv.Options{
			URL:      str("REDIS_URL", ""),
			Addr:     str("REDIS_ADDR", ""),
			Password: getenv("REDIS_PASSWORD"),
		},
	}
	var err error
	if cfg.AllowHTTP, err = envBoolValue(getenv, "GSM_AGENT_ALLOW_HTTP"); err != nil {
		add("%v", err)
	}
	if cfg.Redis.TLS, err = envBoolValue(getenv, "REDIS_TLS"); err != nil {
		add("%v", err)
	}
	if raw := strings.TrimSpace(getenv("REDIS_DB")); raw != "" {
		if cfg.Redis.DB, err = strconv.Atoi(raw); err != nil || cfg.Redis.DB < 0 {
			add("REDIS_DB must be a non-negative integer")
		}
	}
	if cfg.ReadyTimeout, err = envDur(getenv, "GSM_READY_TIMEOUT", 30*time.Second); err != nil {
		add("%v", err)
	}
	if cfg.ShutdownGrace, err = envDur(getenv, "GSM_SHUTDOWN_GRACE", 10*time.Second); err != nil {
		add("%v", err)
	}
	if cfg.RestartWindow, err = envDur(getenv, "GSM_RESTART_WINDOW", 10*time.Minute); err != nil {
		add("%v", err)
	}
	if cfg.MaxRestarts, err = envNonNegInt(getenv, "GSM_MAX_RESTARTS", 3); err != nil {
		add("%v", err)
	}
	if cfg.MaxServers, err = envNonNegInt(getenv, "GSM_MAX_SERVERS", 0); err != nil {
		add("%v", err)
	}

	if len(cfg.Secret) < gsmagent.MinSecretLen {
		add("GSM_AGENT_SECRET is required and must be at least %d characters", gsmagent.MinSecretLen)
	}
	if lo, hi, ok := config.ParsePortRange(getenv("GSM_PORT_RANGE")); ok {
		cfg.PortMin, cfg.PortMax = lo, hi
	} else {
		add("GSM_PORT_RANGE is required, for example 7000-7099 (1024-65535)")
	}
	if cfg.ServerBin == "" {
		add("GSM_SERVER_BIN is required")
	} else if !filepath.IsAbs(cfg.ServerBin) {
		add("GSM_SERVER_BIN must be an absolute path")
	}
	if cfg.MapDir != "" && !filepath.IsAbs(cfg.MapDir) {
		add("GSM_MAP_DIR must be an absolute path")
	}
	if !filepath.IsAbs(cfg.LogsDir) {
		add("GSM_SERVER_LOGS_DIR must be an absolute path")
	}
	if cfg.ServerWorkdir != "" && !filepath.IsAbs(cfg.ServerWorkdir) {
		add("GSM_SERVER_WORKDIR must be an absolute path")
	}
	if net.ParseIP(cfg.BindHost) == nil {
		add("GSM_BIND_HOST must be an IP address")
	}
	if key, err := hex.DecodeString(cfg.NetcodeKeyHex); err != nil || len(key) != 32 {
		add("NETCODE_PRIVATE_KEY is required and must be 64 hex characters")
	}
	if _, err := strconv.ParseUint(cfg.NetcodeProtocolID, 10, 64); err != nil {
		add("NETCODE_PROTOCOL_ID is required and must be an unsigned integer")
	}
	if cfg.APIKey == "" || strings.HasPrefix(cfg.APIKey, "Please-Put") {
		add("GAMESERVER_API_KEY is required")
	}
	if u, err := url.Parse(cfg.APIDomain); err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		add("VERTIGO_API_DOMAIN must be an absolute http(s) URL")
	}
	if cfg.Redis.URL == "" && cfg.Redis.Addr == "" {
		add("REDIS_URL or REDIS_ADDR is required")
	}

	cfg.AgentID = strings.TrimSpace(getenv("GSM_AGENT_ID"))
	if cfg.AgentID == "" {
		host, _ := os.Hostname()
		cfg.AgentID = sanitizeAgentID(host)
	} else if !validAgentID(cfg.AgentID) {
		add("GSM_AGENT_ID may only contain letters, digits, '.', '_' and '-' (max 64)")
	}

	if (cfg.TLSCert == "") != (cfg.TLSKey == "") {
		add("GSM_AGENT_TLS_CERT and GSM_AGENT_TLS_KEY must be set together")
	}
	if cfg.TLSCert == "" && cfg.TLSKey == "" {
		switch {
		case !cfg.AllowHTTP:
			add("HTTPS is required: set GSM_AGENT_TLS_CERT and GSM_AGENT_TLS_KEY (or GSM_AGENT_ALLOW_HTTP=true with a loopback GSM_AGENT_LISTEN behind a tunnel)")
		case !isLoopbackListen(cfg.Listen):
			add("GSM_AGENT_ALLOW_HTTP=true requires a loopback GSM_AGENT_LISTEN such as 127.0.0.1:8443")
		}
	}

	cfg.Home = strings.TrimSpace(getenv("HOME"))
	if cfg.Home == "" {
		cfg.Home = cfg.ServerWorkdir
	}
	if cfg.Home == "" {
		cfg.Home = "/"
	}
	if len(problems) > 0 {
		return cfg, errors.New("invalid gsm-agent configuration:\n  - " + strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

// childEnv is the complete environment of a gameserver. The agent secret
// and Redis credentials are deliberately absent.
func (c agentConfig) childEnv() []string {
	env := []string{
		"PATH=" + c.Path,
		"HOME=" + c.Home,
		"NETCODE_PRIVATE_KEY=" + c.NetcodeKeyHex,
		"NETCODE_PROTOCOL_ID=" + c.NetcodeProtocolID,
	}
	if c.RustLog != "" {
		env = append(env, "RUST_LOG="+c.RustLog)
	}
	return env
}
