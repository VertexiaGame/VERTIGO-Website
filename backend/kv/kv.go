// Package kv owns the shared Redis connection used for state that must
// survive restarts and be visible to every platform instance: HTTP sessions,
// rate-limit counters, Studio/client auth codes, access tokens, gameserver
// records and the feed WebSocket fan-out.
package kv

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const opTimeout = 3 * time.Second

// Options selects the Redis endpoint. URL takes precedence and accepts
// redis:// or rediss:// (TLS) URLs; Addr/Password/DB are the legacy fields.
type Options struct {
	URL      string
	Addr     string
	Password string
	DB       int
	// TLS forces TLS for Addr-style configuration (rediss:// URLs always use TLS).
	TLS bool
}

var (
	mu     sync.RWMutex
	client *redis.Client
)

// Connect dials Redis and verifies it with PING. It never falls back to an
// in-memory store; callers decide whether a failure is fatal.
func Connect(opts Options) (*redis.Client, error) {
	var ro *redis.Options
	if u := strings.TrimSpace(opts.URL); u != "" {
		parsed, err := redis.ParseURL(u)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
		}
		ro = parsed
	} else {
		if strings.TrimSpace(opts.Addr) == "" {
			return nil, errors.New("redis address is not configured")
		}
		ro = &redis.Options{Addr: opts.Addr, Password: opts.Password, DB: opts.DB}
		if opts.TLS {
			host := opts.Addr
			if i := strings.LastIndex(host, ":"); i > 0 {
				host = host[:i]
			}
			ro.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
		}
	}
	if ro.TLSConfig != nil && ro.TLSConfig.MinVersion == 0 {
		ro.TLSConfig.MinVersion = tls.VersionTLS12
	}
	ro.DialTimeout = opTimeout
	ro.ReadTimeout = opTimeout
	ro.WriteTimeout = opTimeout
	c := redis.NewClient(ro)
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// SetClient installs the process-wide client.
func SetClient(c *redis.Client) {
	mu.Lock()
	client = c
	mu.Unlock()
}

// Client returns the process-wide client or nil when Redis is not configured.
func Client() *redis.Client {
	mu.RLock()
	defer mu.RUnlock()
	return client
}

// Ctx returns a context bounded by the standard operation timeout.
func Ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), opTimeout)
}
