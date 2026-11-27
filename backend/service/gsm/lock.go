package gsm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyLockGame     = "gsm:lock:game:"
	keyLockCapacity = "gsm:lock:capacity"
	lockPoll        = 50 * time.Millisecond
)

var errLockTimeout = errors.New("gsm: timed out waiting for lock")

// locker serializes spawn decisions. With a Redis store the lock is shared by
// every platform instance; the memory store uses process-local mutexes.
type locker interface {
	// Lock blocks until key is held or ctx ends. ttl bounds how long a
	// crashed holder can keep the lock.
	Lock(ctx context.Context, key string, ttl time.Duration) (func(), error)
}

type memLocker struct {
	mu    sync.Mutex
	locks map[string]chan struct{}
}

func newMemLocker() *memLocker {
	return &memLocker{locks: make(map[string]chan struct{})}
}

func (l *memLocker) Lock(ctx context.Context, key string, _ time.Duration) (func(), error) {
	l.mu.Lock()
	ch, ok := l.locks[key]
	if !ok {
		ch = make(chan struct{}, 1)
		l.locks[key] = ch
	}
	l.mu.Unlock()
	select {
	case ch <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-ch }) }, nil
	case <-ctx.Done():
		return nil, errLockTimeout
	}
}

// releaseScript deletes the lock only when it still holds our token, so a
// holder whose TTL expired never releases a lock taken over by someone else.
var releaseScript = redis.NewScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`)

type redisLocker struct {
	client *redis.Client
}

func lockToken() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func (l *redisLocker) Lock(ctx context.Context, key string, ttl time.Duration) (func(), error) {
	token := lockToken()
	for {
		opCtx, cancel := context.WithTimeout(ctx, storeTimeout)
		ok, err := l.client.SetNX(opCtx, key, token, ttl).Result()
		cancel()
		if err != nil && ctx.Err() == nil && !errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if ok {
			var once sync.Once
			return func() {
				once.Do(func() {
					rctx, rcancel := context.WithTimeout(context.Background(), storeTimeout)
					defer rcancel()
					_ = releaseScript.Run(rctx, l.client, []string{key}, token).Err()
				})
			}, nil
		}
		select {
		case <-ctx.Done():
			return nil, errLockTimeout
		case <-time.After(lockPoll):
		}
	}
}

func newLocker(store Store) locker {
	if rs, ok := store.(*redisStore); ok && rs.client != nil {
		return &redisLocker{client: rs.client}
	}
	return newMemLocker()
}
