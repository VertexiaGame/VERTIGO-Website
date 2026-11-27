package main

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"vertexia-frontend/backend/gsmagent"
)

// record is the agent's persisted view of one gameserver.
type record struct {
	ID           string      `json:"id"`
	GameID       int         `json:"game_id"`
	Map          string      `json:"map,omitempty"`
	MapPath      string      `json:"map_path"`
	MaxPlayers   int         `json:"max_players"`
	Port         int         `json:"port"`
	PID          int         `json:"pid"`
	Status       string      `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
	StartedAt    time.Time   `json:"started_at"`
	Restarts     int         `json:"restarts"`
	RestartTimes []time.Time `json:"restart_times,omitempty"`
	FailedAt     time.Time   `json:"failed_at,omitempty"`
	LastError    string      `json:"last_error,omitempty"`
}

func (r record) view() gsmagent.Server {
	return gsmagent.Server{
		ID:         r.ID,
		Port:       r.Port,
		PID:        r.PID,
		Status:     r.Status,
		GameID:     r.GameID,
		MaxPlayers: r.MaxPlayers,
		StartedAt:  r.StartedAt,
		Restarts:   r.Restarts,
	}
}

// recordStore persists records so an agent restart can reconcile.
type recordStore interface {
	Save(ctx context.Context, r record) error
	Delete(ctx context.Context, id string) error
	LoadAll(ctx context.Context) ([]record, error)
}

// redisRecordStore keeps records under gsmagent:<agent id>:server:<id> plus
// the index set gsmagent:<agent id>:servers.
type redisRecordStore struct {
	client *redis.Client
	prefix string
}

func newRedisRecordStore(c *redis.Client, agentID string) *redisRecordStore {
	return &redisRecordStore{client: c, prefix: "gsmagent:" + agentID + ":"}
}

func (s *redisRecordStore) key(id string) string { return s.prefix + "server:" + id }
func (s *redisRecordStore) index() string        { return s.prefix + "servers" }

func (s *redisRecordStore) Save(ctx context.Context, r record) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.key(r.ID), raw, 0)
	pipe.SAdd(ctx, s.index(), r.ID)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *redisRecordStore) Delete(ctx context.Context, id string) error {
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, s.key(id))
	pipe.SRem(ctx, s.index(), id)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *redisRecordStore) LoadAll(ctx context.Context) ([]record, error) {
	ids, err := s.client.SMembers(ctx, s.index()).Result()
	if err != nil {
		return nil, err
	}
	out := make([]record, 0, len(ids))
	for _, id := range ids {
		raw, err := s.client.Get(ctx, s.key(id)).Bytes()
		if err == redis.Nil {
			_ = s.client.SRem(ctx, s.index(), id).Err()
			continue
		}
		if err != nil {
			return nil, err
		}
		var r record
		if err := json.Unmarshal(raw, &r); err != nil || r.ID != id {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// memRecordStore is used by tests.
type memRecordStore struct {
	mu   sync.Mutex
	recs map[string]record
}

func newMemRecordStore() *memRecordStore {
	return &memRecordStore{recs: make(map[string]record)}
}

func (s *memRecordStore) Save(_ context.Context, r record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.RestartTimes = append([]time.Time(nil), r.RestartTimes...)
	s.recs[r.ID] = r
	return nil
}

func (s *memRecordStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.recs, id)
	return nil
}

func (s *memRecordStore) LoadAll(_ context.Context) ([]record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]record, 0, len(s.recs))
	for _, r := range s.recs {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *memRecordStore) get(id string) (record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[id]
	return r, ok
}
