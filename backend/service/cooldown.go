package service

import (
	"fmt"
	"log"
	"math"
	"time"

	"vertexia-frontend/backend/kv"
)

// CooldownService enforces per-key action cooldowns. State lives in Redis
// (SET NX PX) so every platform instance shares it; kv only falls back to a
// process-local map in development.
type CooldownService struct {
	defaultDuration time.Duration
}

const cooldownPrefix = "cooldown:"

func NewCooldownService(defaultDuration time.Duration) *CooldownService {
	if defaultDuration <= 0 {
		defaultDuration = 5 * time.Second
	}
	return &CooldownService{defaultDuration: defaultDuration}
}

func (s *CooldownService) Allow(key string, customDuration ...time.Duration) (bool, time.Duration) {
	duration := s.defaultDuration
	if len(customDuration) > 0 && customDuration[0] > 0 {
		duration = customDuration[0]
	}
	ctx, cancel := kv.Ctx()
	defer cancel()
	ok, err := kv.SetNX(ctx, cooldownPrefix+key, []byte{1}, duration)
	if err != nil {
		// Cooldowns are anti-spam, not access control: a Redis outage must
		// not block every write on the site.
		log.Printf("cooldown check for %q failed open: %v", key, err)
		return true, 0
	}
	if ok {
		return true, 0
	}
	remaining, err := kv.PTTL(ctx, cooldownPrefix+key)
	if err != nil || remaining <= 0 {
		remaining = time.Second
	}
	if remaining > duration {
		remaining = duration
	}
	return false, remaining
}

func (s *CooldownService) FormatRemaining(remaining time.Duration) string {
	secs := int(math.Ceil(remaining.Seconds()))
	if secs <= 1 {
		return "1 second"
	}
	return fmt.Sprintf("%d seconds", secs)
}
