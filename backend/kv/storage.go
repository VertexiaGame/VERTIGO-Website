package kv

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

// Storage implements fiber.Storage. Every call resolves the process-wide
// Redis client, so middleware built at package init (rate limiters) picks up
// Redis once main has connected. Without a client it uses a process-local map,
// which is only acceptable in development: production startup refuses to run
// without Redis (see config.RedisRequired) and SetRequired turns any fallback
// into an error.
type Storage struct {
	prefix string
	local  *memory
}

var _ fiber.Storage = (*Storage)(nil)

// NewStorage returns a storage that namespaces keys under prefix.
func NewStorage(prefix string) *Storage {
	return &Storage{prefix: prefix, local: newMemory()}
}

func (s *Storage) key(k string) string { return s.prefix + k }

func (s *Storage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if key == "" {
		return nil, nil
	}
	if c := Client(); c != nil {
		val, err := c.Get(ctx, s.key(key)).Bytes()
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return val, err
	}
	if Required() {
		return nil, ErrUnavailable
	}
	return s.local.get(key), nil
}

func (s *Storage) Get(key string) ([]byte, error) {
	ctx, cancel := Ctx()
	defer cancel()
	return s.GetWithContext(ctx, key)
}

func (s *Storage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if key == "" || len(val) == 0 {
		return nil
	}
	if c := Client(); c != nil {
		return c.Set(ctx, s.key(key), val, exp).Err()
	}
	if Required() {
		return ErrUnavailable
	}
	s.local.set(key, val, exp)
	return nil
}

func (s *Storage) Set(key string, val []byte, exp time.Duration) error {
	ctx, cancel := Ctx()
	defer cancel()
	return s.SetWithContext(ctx, key, val, exp)
}

func (s *Storage) DeleteWithContext(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	if c := Client(); c != nil {
		return c.Del(ctx, s.key(key)).Err()
	}
	if Required() {
		return ErrUnavailable
	}
	s.local.del(key)
	return nil
}

func (s *Storage) Delete(key string) error {
	ctx, cancel := Ctx()
	defer cancel()
	return s.DeleteWithContext(ctx, key)
}

// ResetWithContext deletes every key under this storage's prefix.
func (s *Storage) ResetWithContext(ctx context.Context) error {
	if c := Client(); c != nil {
		iter := c.Scan(ctx, 0, s.prefix+"*", 500).Iterator()
		for iter.Next(ctx) {
			if err := c.Del(ctx, iter.Val()).Err(); err != nil {
				return err
			}
		}
		return iter.Err()
	}
	if Required() {
		return ErrUnavailable
	}
	s.local.reset()
	return nil
}

func (s *Storage) Reset() error {
	ctx, cancel := Ctx()
	defer cancel()
	return s.ResetWithContext(ctx)
}

// Close is a no-op: the shared client is closed by main on shutdown.
func (s *Storage) Close() error { return nil }

type memEntry struct {
	val []byte
	exp time.Time
}

type memory struct {
	mu   sync.Mutex
	data map[string]memEntry
}

func newMemory() *memory { return &memory{data: make(map[string]memEntry)} }

func (m *memory) get(k string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[k]
	if !ok {
		return nil
	}
	if !e.exp.IsZero() && time.Now().After(e.exp) {
		delete(m.data, k)
		return nil
	}
	return append([]byte(nil), e.val...)
}

func (m *memory) set(k string, v []byte, exp time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := memEntry{val: append([]byte(nil), v...)}
	if exp > 0 {
		e.exp = time.Now().Add(exp)
	}
	m.data[k] = e
	if len(m.data) > 100000 {
		now := time.Now()
		for key, entry := range m.data {
			if !entry.exp.IsZero() && now.After(entry.exp) {
				delete(m.data, key)
			}
		}
	}
}

func (m *memory) del(k string) {
	m.mu.Lock()
	delete(m.data, k)
	m.mu.Unlock()
}

func (m *memory) reset() {
	m.mu.Lock()
	m.data = make(map[string]memEntry)
	m.mu.Unlock()
}
