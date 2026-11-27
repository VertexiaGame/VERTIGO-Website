package config

import (
	"strings"
	"testing"
)

func validProduction() *Config {
	secret := strings.Repeat("s", 40)
	return &Config{
		AppEnv:             "production",
		PublicBaseURL:      "https://vertexia.wempos-forum.com",
		AltchaHMACKey:      secret,
		RegistrationIPSalt: secret,
		SessionSecure:      true,
		RedisRequired:      true,
		RunWithDatabase:    true,
		DBHost:             "db.example.internal",
		DBTLS:              "true",
		GSMEnabled:         true,
		GSMAgentURL:        "https://agent.example.internal:8443",
		GSMAgentSecret:     strings.Repeat("a", 48),
		GSMPublicHost:      "games.vertexia.wempos-forum.com",
		NetcodePrivateKey:  make([]byte, 32),
		GameserverAPIKey:   strings.Repeat("g", 40),
		R2AccountID:        "acct",
		R2Bucket:           "bucket",
		R2AccessKeyID:      "id",
		R2SecretAccessKey:  "secret",
	}
}

func TestValidateAcceptsCompleteProductionConfig(t *testing.T) {
	if err := validProduction().Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}

func TestValidateAcceptsEmptyDevelopmentConfig(t *testing.T) {
	if err := (&Config{}).Validate(); err != nil {
		t.Fatalf("development defaults must validate, got %v", err)
	}
}

func TestValidateRejectsUnsafeProductionConfigs(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"weak altcha key", func(c *Config) { c.AltchaHMACKey = "short" }, "ALTCHA_HMAC_KEY"},
		{"placeholder salt", func(c *Config) { c.RegistrationIPSalt = "Please-Put-A-Real-Salt-Here-0123456789" }, "REGISTRATION_IP_SALT"},
		{"insecure session", func(c *Config) { c.SessionSecure = false }, "SESSION_SECURE"},
		{"redis optional", func(c *Config) { c.RedisRequired = false }, "REDIS_REQUIRED"},
		{"db without tls", func(c *Config) { c.DBTLS = "" }, "DB_TLS must be true"},
		{"db skip verify", func(c *Config) { c.DBTLS = "skip-verify" }, "skip-verify"},
		{"local gameservers", func(c *Config) { c.GSMAgentURL = "" }, "GSM_AGENT_URL is required"},
		{"plain http agent", func(c *Config) { c.GSMAgentURL = "http://agent:8080" }, "https"},
		{"short agent secret", func(c *Config) { c.GSMAgentSecret = "short" }, "GSM_AGENT_SECRET"},
		{"no public udp host", func(c *Config) { c.GSMPublicHost = "" }, "GSM_PUBLIC_HOST"},
		{"udp host is web host", func(c *Config) { c.GSMPublicHost = "vertexia.wempos-forum.com" }, "web hostname"},
		{"missing netcode key", func(c *Config) { c.NetcodePrivateKey = nil }, "NETCODE_PRIVATE_KEY"},
		{"bad netcode key", func(c *Config) { c.NetcodePrivateKey = make([]byte, 16) }, "64 hex"},
		{"weak gameserver key", func(c *Config) { c.GameserverAPIKey = "abc" }, "GAMESERVER_API_KEY"},
		{"no r2", func(c *Config) { c.R2Bucket = "" }, "R2_BUCKET is required"},
		{"r2 without credentials", func(c *Config) { c.R2SecretAccessKey = "" }, "R2_ACCESS_KEY_ID"},
		{"r2 public url not https", func(c *Config) { c.R2PublicBaseURL = "http://cdn.example" }, "R2_PUBLIC_BASE_URL"},
		{"unknown db tls", func(c *Config) { c.DBTLS = "maybe" }, "DB_TLS must be one of"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validProduction()
			tc.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestValidateAllowsLocalDatabaseWithoutTLS(t *testing.T) {
	cfg := validProduction()
	cfg.DBHost = "127.0.0.1"
	cfg.DBTLS = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("loopback MySQL should not require TLS: %v", err)
	}
}

func TestParsePortRange(t *testing.T) {
	cases := []struct {
		in     string
		lo, hi int
		ok     bool
	}{
		{"7000-7099", 7000, 7099, true},
		{" 7000 - 7000 ", 7000, 7000, true},
		{"7099-7000", 0, 0, false},
		{"80-90", 0, 0, false},
		{"7000-70000", 0, 0, false},
		{"7000", 0, 0, false},
		{"a-b", 0, 0, false},
	}
	for _, tc := range cases {
		lo, hi, ok := ParsePortRange(tc.in)
		if ok != tc.ok || lo != tc.lo || hi != tc.hi {
			t.Errorf("ParsePortRange(%q) = %d, %d, %v; want %d, %d, %v", tc.in, lo, hi, ok, tc.lo, tc.hi, tc.ok)
		}
	}
}

func TestLoadParsesNetcodeAndPortRange(t *testing.T) {
	t.Setenv("NETCODE_PRIVATE_KEY", strings.Repeat("ab", 32))
	t.Setenv("NETCODE_PROTOCOL_ID", "123456789")
	t.Setenv("GSM_PORT_RANGE", "7000-7010")
	t.Setenv("APP_ENV", "production")
	t.Setenv("REDIS_REQUIRED", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.NetcodePrivateKey) != 32 || cfg.NetcodePrivateKey[0] != 0xab {
		t.Fatalf("netcode key not decoded: %x", cfg.NetcodePrivateKey)
	}
	if cfg.NetcodeProtocolID != 123456789 {
		t.Fatalf("protocol id = %d", cfg.NetcodeProtocolID)
	}
	if cfg.GSMPortMin != 7000 || cfg.GSMPortMax != 7010 {
		t.Fatalf("port range = %d-%d", cfg.GSMPortMin, cfg.GSMPortMax)
	}
	if !cfg.RedisRequired {
		t.Fatal("production must default REDIS_REQUIRED to true")
	}
	if cfg.GSMPublicHost != "" {
		t.Fatalf("production must not auto-detect GSM_PUBLIC_HOST, got %q", cfg.GSMPublicHost)
	}
}

func TestLoadRejectsMalformedNetcodeKey(t *testing.T) {
	t.Setenv("NETCODE_PRIVATE_KEY", "not-hex")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a malformed NETCODE_PRIVATE_KEY")
	}
}
