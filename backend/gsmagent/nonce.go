package gsmagent

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// NonceKeyPrefix prefixes nonce keys in Redis.
const NonceKeyPrefix = "gsmagent:nonce:"

// NonceStore remembers nonces for replay protection.
type NonceStore interface {
	// Remember records nonce for ttl and reports whether it was unseen.
	Remember(ctx context.Context, nonce string, ttl time.Duration) (bool, error)
}

// RedisNonceStore keeps nonces in Redis with SET NX EX so every agent
// process sharing the Redis database rejects the same replays.
type RedisNonceStore struct {
	client *redis.Client
}

func NewRedisNonceStore(c *redis.Client) *RedisNonceStore {
	return &RedisNonceStore{client: c}
}

func (s *RedisNonceStore) Remember(ctx context.Context, nonce string, ttl time.Duration) (bool, error) {
	if ttl < time.Second {
		ttl = time.Second
	}
	// SetArgs with Mode NX and TTL issues SET key val NX EX seconds.
	res, err := s.client.SetArgs(ctx, NonceKeyPrefix+nonce, 1, redis.SetArgs{Mode: "NX", TTL: ttl.Truncate(time.Second)}).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return res == "OK", nil
}

// MemoryNonceStore is a process-local NonceStore for tests and development.
type MemoryNonceStore struct {
	mu      sync.Mutex
	seen    map[string]time.Time
	now     func() time.Time
	lastGC  time.Time
	gcEvery time.Duration
}

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{seen: make(map[string]time.Time), now: time.Now, gcEvery: 10 * time.Second}
}

func (s *MemoryNonceStore) Remember(_ context.Context, nonce string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if now.Sub(s.lastGC) >= s.gcEvery {
		for k, exp := range s.seen {
			if !now.Before(exp) {
				delete(s.seen, k)
			}
		}
		s.lastGC = now
	}
	if exp, ok := s.seen[nonce]; ok && now.Before(exp) {
		return false, nil
	}
	s.seen[nonce] = now.Add(ttl)
	return true, nil
}
