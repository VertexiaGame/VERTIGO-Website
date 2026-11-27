package database

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"vertexia-frontend/backend/config"
)

var DB *sql.DB

// tlsConfigName is the name registered with the MySQL driver when DB_TLS=true
// carries a custom CA or server name.
const tlsConfigName = "vertexia"

// BuildDSN returns the driver DSN for cfg. It registers a custom TLS config
// with the driver when a CA bundle or server name override is configured.
func BuildDSN(cfg *config.Config) (string, error) {
	mc := mysql.NewConfig()
	mc.User = cfg.DBUser
	mc.Passwd = cfg.DBPass
	mc.Net = "tcp"
	port := strings.TrimSpace(cfg.DBPort)
	if port == "" {
		port = "3306"
	}
	mc.Addr = net.JoinHostPort(strings.TrimSpace(cfg.DBHost), port)
	mc.DBName = cfg.DBName
	mc.ParseTime = true
	mc.Loc = time.Local
	timeout := cfg.DBTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	mc.Timeout = timeout
	mc.ReadTimeout = 3 * timeout
	mc.WriteTimeout = 3 * timeout

	tlsName, err := tlsMode(cfg)
	if err != nil {
		return "", err
	}
	mc.TLSConfig = tlsName
	return mc.FormatDSN(), nil
}

func tlsMode(cfg *config.Config) (string, error) {
	switch cfg.DBTLS {
	case "", "false":
		return "", nil
	case "preferred", "skip-verify":
		return cfg.DBTLS, nil
	case "true":
		if cfg.DBTLSCAPEM == "" && cfg.DBTLSServerName == "" {
			// Driver default: verify against the system roots and DB_HOST.
			return "true", nil
		}
		tc := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.DBTLSServerName}
		if tc.ServerName == "" {
			tc.ServerName = strings.TrimSpace(cfg.DBHost)
		}
		if cfg.DBTLSCAPEM != "" {
			pool := x509.NewCertPool()
			// Secrets stores often flatten newlines; accept literal "\n".
			pem := strings.ReplaceAll(cfg.DBTLSCAPEM, `\n`, "\n")
			if !pool.AppendCertsFromPEM([]byte(pem)) {
				return "", errors.New("DB_TLS_CA_PEM does not contain a valid PEM certificate")
			}
			tc.RootCAs = pool
		}
		if err := mysql.RegisterTLSConfig(tlsConfigName, tc); err != nil {
			return "", err
		}
		return tlsConfigName, nil
	default:
		return "", errors.New("unsupported DB_TLS value")
	}
}

func Connect(cfg *config.Config) error {
	if !cfg.RunWithDatabase {
		return nil
	}

	dsn, err := BuildDSN(cfg)
	if err != nil {
		return err
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	maxOpen := cfg.DBMaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 25
	}
	maxIdle := cfg.DBMaxIdleConns
	if maxIdle <= 0 || maxIdle > maxOpen {
		maxIdle = min(10, maxOpen)
	}
	lifetime := cfg.DBConnMaxLifetime
	if lifetime <= 0 {
		lifetime = 30 * time.Minute
	}
	idleTime := cfg.DBConnMaxIdleTime
	if idleTime <= 0 {
		idleTime = 5 * time.Minute
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(lifetime)
	db.SetConnMaxIdleTime(idleTime)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout+5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return err
	}

	DB = db
	return nil
}

// Ping checks the shared pool; it returns nil when the app runs without a DB.
func Ping(ctx context.Context) error {
	if DB == nil {
		return nil
	}
	return DB.PingContext(ctx)
}
