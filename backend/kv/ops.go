package kv

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// The helpers below run against Redis when connected. Without Redis they use
// one process-local map, which is only acceptable in development; when
// Required() is set they return ErrUnavailable instead.

var (
	fallbackMu sync.Mutex
	fallback   = newMemory()
)

func useFallback() error {
	if Required() {
		return ErrUnavailable
	}
	return nil
}

// SetNX stores val under key only if it does not exist yet. It reports
// whether the key was set.
func SetNX(ctx context.Context, key string, val []byte, ttl time.Duration) (bool, error) {
	if c := Client(); c != nil {
		return c.SetNX(ctx, key, val, ttl).Result()
	}
	if err := useFallback(); err != nil {
		return false, err
	}
	fallbackMu.Lock()
	defer fallbackMu.Unlock()
	if fallback.get(key) != nil {
		return false, nil
	}
	fallback.set(key, val, ttl)
	return true, nil
}

// Set stores val under key with an optional ttl (0 = no expiry).
func Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if c := Client(); c != nil {
		return c.Set(ctx, key, val, ttl).Err()
	}
	if err := useFallback(); err != nil {
		return err
	}
	fallback.set(key, val, ttl)
	return nil
}

// Get returns the value or (nil, nil) when the key does not exist.
func Get(ctx context.Context, key string) ([]byte, error) {
	if c := Client(); c != nil {
		val, err := c.Get(ctx, key).Bytes()
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return val, err
	}
	if err := useFallback(); err != nil {
		return nil, err
	}
	return fallback.get(key), nil
}

// GetDel atomically reads and deletes key; (nil, nil) when missing.
func GetDel(ctx context.Context, key string) ([]byte, error) {
	if c := Client(); c != nil {
		val, err := c.GetDel(ctx, key).Bytes()
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return val, err
	}
	if err := useFallback(); err != nil {
		return nil, err
	}
	fallbackMu.Lock()
	defer fallbackMu.Unlock()
	val := fallback.get(key)
	fallback.del(key)
	return val, nil
}

// Del removes keys.
func Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if c := Client(); c != nil {
		return c.Del(ctx, keys...).Err()
	}
	if err := useFallback(); err != nil {
		return err
	}
	for _, k := range keys {
		fallback.del(k)
	}
	return nil
}

// PTTL returns the remaining time to live of key, or 0 when it has none.
func PTTL(ctx context.Context, key string) (time.Duration, error) {
	if c := Client(); c != nil {
		d, err := c.PTTL(ctx, key).Result()
		if err != nil || d < 0 {
			return 0, err
		}
		return d, nil
	}
	if err := useFallback(); err != nil {
		return 0, err
	}
	fallback.mu.Lock()
	defer fallback.mu.Unlock()
	if e, ok := fallback.data[key]; ok && !e.exp.IsZero() {
		if d := time.Until(e.exp); d > 0 {
			return d, nil
		}
	}
	return 0, nil
}

// SAdd / SRem / SMembers manage small index sets (for example a user's tokens).
func SAdd(ctx context.Context, key string, members ...string) error {
	if c := Client(); c != nil {
		args := make([]any, len(members))
		for i, m := range members {
			args[i] = m
		}
		return c.SAdd(ctx, key, args...).Err()
	}
	if err := useFallback(); err != nil {
		return err
	}
	fallbackMu.Lock()
	defer fallbackMu.Unlock()
	set := decodeSet(fallback.get(key))
	for _, m := range members {
		set[m] = struct{}{}
	}
	fallback.set(key, encodeSet(set), 0)
	return nil
}

func SRem(ctx context.Context, key string, members ...string) error {
	if c := Client(); c != nil {
		args := make([]any, len(members))
		for i, m := range members {
			args[i] = m
		}
		return c.SRem(ctx, key, args...).Err()
	}
	if err := useFallback(); err != nil {
		return err
	}
	fallbackMu.Lock()
	defer fallbackMu.Unlock()
	set := decodeSet(fallback.get(key))
	for _, m := range members {
		delete(set, m)
	}
	fallback.set(key, encodeSet(set), 0)
	return nil
}

func SMembers(ctx context.Context, key string) ([]string, error) {
	if c := Client(); c != nil {
		return c.SMembers(ctx, key).Result()
	}
	if err := useFallback(); err != nil {
		return nil, err
	}
	fallbackMu.Lock()
	defer fallbackMu.Unlock()
	set := decodeSet(fallback.get(key))
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	return out, nil
}

func decodeSet(raw []byte) map[string]struct{} {
	set := make(map[string]struct{})
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i == len(raw) || raw[i] == 0 {
			if i > start {
				set[string(raw[start:i])] = struct{}{}
			}
			start = i + 1
		}
	}
	return set
}

func encodeSet(set map[string]struct{}) []byte {
	var out []byte
	for m := range set {
		if len(out) > 0 {
			out = append(out, 0)
		}
		out = append(out, m...)
	}
	if out == nil {
		out = []byte{}
	}
	return out
}

// Publish sends a message on a Redis pub/sub channel. Without Redis it is a
// no-op in development (single instance) and an error when Redis is required.
func Publish(ctx context.Context, channel string, payload []byte) error {
	if c := Client(); c != nil {
		return c.Publish(ctx, channel, payload).Err()
	}
	return useFallback()
}

// Subscribe delivers every message on channel to fn until ctx is done. It
// reconnects after errors. It returns immediately when Redis is not connected.
func Subscribe(ctx context.Context, channel string, fn func([]byte)) bool {
	c := Client()
	if c == nil {
		return false
	}
	go func() {
		backoff := time.Second
		for ctx.Err() == nil {
			sub := c.Subscribe(ctx, channel)
			if _, err := sub.Receive(ctx); err != nil {
				_ = sub.Close()
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				if backoff < 30*time.Second {
					backoff *= 2
				}
				continue
			}
			backoff = time.Second
			ch := sub.Channel()
		recv:
			for {
				select {
				case <-ctx.Done():
					break recv
				case msg, ok := <-ch:
					if !ok {
						break recv
					}
					fn([]byte(msg.Payload))
				}
			}
			_ = sub.Close()
		}
	}()
	return true
}
