package config

import (
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

//handle and provide configurations

type Config struct {
	DBUser                 string
	DBPass                 string
	DBHost                 string
	DBPort                 string
	DBName                 string
	RunWithDatabase        bool
	DBMaxOpenConns         int
	DBMaxIdleConns         int
	DBConnMaxLifetime      time.Duration
	DBConnMaxIdleTime      time.Duration
	ServerReadTimeout      time.Duration
	ServerWriteTimeout     time.Duration
	ServerIdleTimeout      time.Duration
	LimiterMax             int
	LimiterExpiration      time.Duration
	SessionSecure          bool
	SessionSameSite        string
	SessionIdleTimeout     time.Duration
	SessionAbsoluteTimeout time.Duration
	AltchaHMACKey          string
	GameserverAPIKey       string
	LiveTimerEnabled       bool
	LiveTimerDuration      time.Duration
	LiveTimerEnd           time.Time
	SiteLockEnabled        bool
	SiteLockPassword       string
	RedisAddr              string
	RedisPassword          string
	RedisDB                int
	GSMEnabled             bool
	GSMServerBin           string
	GSMServerWorkdir       string
	GSMServerMap           string
	GSMBindHost            string
	GSMPublicHost          string
	GSMPortMin             int
	GSMPortMax             int
	GSMMaxPlayersDefault   int
	GSMMinPlayersDefault   int
	GSMIdleTimeout         time.Duration
	GSMReapInterval        time.Duration
	GSMShutdownGrace       time.Duration
	GSMServerLogsDir       string
}

var Global *Config

func parseLiveTimerDuration(val string) time.Duration {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	if d, err := time.ParseDuration(val); err == nil && d > 0 {
		return d
	}
	if sec, err := strconv.Atoi(val); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	if strings.Contains(val, ":") {
		parts := strings.Split(val, ":")
		if len(parts) == 2 {
			m, err1 := strconv.Atoi(parts[0])
			s, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil && m >= 0 && s >= 0 {
				return time.Duration(m)*time.Minute + time.Duration(s)*time.Second
			}
		}
	}
	return 0
}

func (c *Config) IsSiteLockActive() bool {
	return c != nil && c.SiteLockEnabled && c.SiteLockPassword != ""
}

func (c *Config) IsLiveTimerActive() bool {
	return c != nil && c.LiveTimerEnabled && c.LiveTimerDuration > 0 && time.Now().Before(c.LiveTimerEnd)
}

func (c *Config) LiveTimerRemaining() time.Duration {
	if !c.IsLiveTimerActive() {
		return 0
	}
	rem := time.Until(c.LiveTimerEnd)
	if rem < 0 {
		return 0
	}
	return rem
}

func detectServerHost() string {
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		addr, _ := conn.LocalAddr().(*net.UDPAddr)
		_ = conn.Close()
		if addr != nil && !addr.IP.IsLoopback() && !addr.IP.IsUnspecified() {
			return addr.IP.String()
		}
	}
	if interfaces, err := net.Interfaces(); err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}
				ip := ipNet.IP.To4()
				if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
					continue
				}
				return ip.String()
			}
		}
	}
	return ""
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	runWithDB, _ := strconv.ParseBool(os.Getenv("RUN_WITH_DATABASE"))

	dbMaxOpenConns, _ := strconv.Atoi(os.Getenv("DB_MAX_OPEN_CONNS"))
	dbMaxIdleConns, _ := strconv.Atoi(os.Getenv("DB_MAX_IDLE_CONNS"))
	dbConnMaxLifetime, _ := time.ParseDuration(os.Getenv("DB_CONN_MAX_LIFETIME"))
	dbConnMaxIdleTime, _ := time.ParseDuration(os.Getenv("DB_CONN_MAX_IDLE_TIME"))
	serverReadTimeout, _ := time.ParseDuration(os.Getenv("SERVER_READ_TIMEOUT"))
	serverWriteTimeout, _ := time.ParseDuration(os.Getenv("SERVER_WRITE_TIMEOUT"))
	serverIdleTimeout, _ := time.ParseDuration(os.Getenv("SERVER_IDLE_TIMEOUT"))
	if serverReadTimeout <= 0 {
		serverReadTimeout = 120 * time.Second
	}
	if serverWriteTimeout <= 0 {
		serverWriteTimeout = 120 * time.Second
	}
	if serverIdleTimeout <= 0 {
		serverIdleTimeout = 60 * time.Second
	}
	limiterMax, _ := strconv.Atoi(os.Getenv("LIMITER_MAX"))
	limiterExpiration, _ := time.ParseDuration(os.Getenv("LIMITER_EXPIRATION"))
	sessionSecure, _ := strconv.ParseBool(os.Getenv("SESSION_SECURE"))
	sessionSameSite := os.Getenv("SESSION_SAMESITE")
	sessionIdleTimeout, _ := time.ParseDuration(os.Getenv("SESSION_IDLE_TIMEOUT"))
	sessionAbsoluteTimeout, _ := time.ParseDuration(os.Getenv("SESSION_ABSOLUTE_TIMEOUT"))
	altchaHMACKey := os.Getenv("ALTCHA_HMAC_KEY")
	redisAddr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	gsmEnabled := true
	if raw := strings.TrimSpace(os.Getenv("GSM_ENABLED")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			gsmEnabled = parsed
		}
	}
	gsmPortMin, _ := strconv.Atoi(os.Getenv("GSM_PORT_MIN"))
	if gsmPortMin <= 0 {
		gsmPortMin = 5100
	}
	gsmPortMax, _ := strconv.Atoi(os.Getenv("GSM_PORT_MAX"))
	if gsmPortMax <= 0 {
		gsmPortMax = 5250
	}
	if gsmPortMax < gsmPortMin {
		gsmPortMax = gsmPortMin
	}
	gsmMaxPlayers, _ := strconv.Atoi(os.Getenv("GSM_MAX_PLAYERS_DEFAULT"))
	if gsmMaxPlayers <= 0 {
		gsmMaxPlayers = 100
	}
	gsmMinPlayers, _ := strconv.Atoi(os.Getenv("GSM_MIN_PLAYERS_DEFAULT"))
	if gsmMinPlayers <= 0 {
		gsmMinPlayers = 2
	}
	gsmIdleTimeout, _ := time.ParseDuration(os.Getenv("GSM_IDLE_TIMEOUT"))
	if gsmIdleTimeout <= 0 {
		gsmIdleTimeout = 5 * time.Minute
	}
	gsmReapInterval, _ := time.ParseDuration(os.Getenv("GSM_REAP_INTERVAL"))
	if gsmReapInterval <= 0 {
		gsmReapInterval = 15 * time.Second
	}
	gsmShutdownGrace, _ := time.ParseDuration(os.Getenv("GSM_SHUTDOWN_GRACE"))
	if gsmShutdownGrace <= 0 {
		gsmShutdownGrace = 10 * time.Second
	}
	gsmServerBin := strings.TrimSpace(os.Getenv("GSM_SERVER_BIN"))
	if gsmServerBin == "" {
		gsmServerBin = `C:\Users\xampp\Documents\VERTIGO-Full\A\target\debug\RaveEngineServer.exe`
	}
	gsmServerWorkdir := strings.TrimSpace(os.Getenv("GSM_SERVER_WORKDIR"))
	if gsmServerWorkdir == "" {
		gsmServerWorkdir = `C:\Users\xampp\Documents\VERTIGO-Full\A\target\debug`
	}
	gsmServerMap := strings.TrimSpace(os.Getenv("GSM_SERVER_MAP"))
	if gsmServerMap == "" {
		gsmServerMap = "assets/maps/default.vrtx"
	}
	gsmBindHost := strings.TrimSpace(os.Getenv("GSM_BIND_HOST"))
	if gsmBindHost == "" {
		gsmBindHost = "127.0.0.1"
	}
	gsmPublicHost := strings.TrimSpace(os.Getenv("GSM_PUBLIC_HOST"))
	if gsmPublicHost == "" {
		if detected := detectServerHost(); detected != "" {
			gsmPublicHost = detected
		} else {
			gsmPublicHost = gsmBindHost
		}
	}
	gsmLogsDir := strings.TrimSpace(os.Getenv("GSM_SERVER_LOGS_DIR"))
	if gsmLogsDir == "" {
		gsmLogsDir = "./logs/gsm"
	}

	liveTimerEnabled, _ := strconv.ParseBool(os.Getenv("LIVETIMER_ENABLED"))
	liveTimerDuration := parseLiveTimerDuration(os.Getenv("LIVETIMER_DURATION"))
	if liveTimerDuration > time.Hour {
		liveTimerDuration = time.Hour
	}
	liveTimerEnd := time.Now().Add(liveTimerDuration)

	siteLockEnabled, _ := strconv.ParseBool(os.Getenv("SITELOCK_ENABLED"))
	siteLockPassword := os.Getenv("SITELOCK_PASSWORD")

	cfg := &Config{
		DBUser:                 os.Getenv("DB_USER"),
		DBPass:                 os.Getenv("DB_PASS"),
		DBHost:                 os.Getenv("DB_HOST"),
		DBPort:                 os.Getenv("DB_PORT"),
		DBName:                 os.Getenv("DB_NAME"),
		RunWithDatabase:        runWithDB, //testing purposes
		DBMaxOpenConns:         dbMaxOpenConns,
		DBMaxIdleConns:         dbMaxIdleConns,
		DBConnMaxLifetime:      dbConnMaxLifetime,
		DBConnMaxIdleTime:      dbConnMaxIdleTime,
		ServerReadTimeout:      serverReadTimeout,
		ServerWriteTimeout:     serverWriteTimeout,
		ServerIdleTimeout:      serverIdleTimeout,
		LimiterMax:             limiterMax,
		LimiterExpiration:      limiterExpiration,
		SessionSecure:          sessionSecure,
		SessionSameSite:        sessionSameSite,
		SessionIdleTimeout:     sessionIdleTimeout,
		SessionAbsoluteTimeout: sessionAbsoluteTimeout,
		AltchaHMACKey:          altchaHMACKey,
		GameserverAPIKey:       os.Getenv("GAMESERVER_API_KEY"),
		LiveTimerEnabled:       liveTimerEnabled,
		LiveTimerDuration:      liveTimerDuration,
		LiveTimerEnd:           liveTimerEnd,
		SiteLockEnabled:        siteLockEnabled,
		SiteLockPassword:       siteLockPassword,
		RedisAddr:              redisAddr,
		RedisPassword:          os.Getenv("REDIS_PASSWORD"),
		RedisDB:                redisDB,
		GSMEnabled:             gsmEnabled,
		GSMServerBin:           gsmServerBin,
		GSMServerWorkdir:       gsmServerWorkdir,
		GSMServerMap:           gsmServerMap,
		GSMBindHost:            gsmBindHost,
		GSMPublicHost:          gsmPublicHost,
		GSMPortMin:             gsmPortMin,
		GSMPortMax:             gsmPortMax,
		GSMMaxPlayersDefault:   gsmMaxPlayers,
		GSMMinPlayersDefault:   gsmMinPlayers,
		GSMIdleTimeout:         gsmIdleTimeout,
		GSMReapInterval:        gsmReapInterval,
		GSMShutdownGrace:       gsmShutdownGrace,
		GSMServerLogsDir:       gsmLogsDir,
	}

	Global = cfg

	//return to database.go the config
	return cfg, nil
}