package gsm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyGS        = "gsm:gs:"
	keyGame      = "gsm:game:"
	keyAll       = "gsm:ids"
	keyPorts     = "gsm:ports"
	keyMembers   = "gsm:members:"
	keyConfirmed = "gsm:confirmed:"
	storeTimeout = 3 * time.Second
)

type Store interface {
	Exists(id string) (bool, error)
	Save(gs GameServer) error
	Get(id string) (GameServer, bool, error)
	Delete(id string) error
	ListByGame(gameID int) ([]GameServer, error)
	ListAll() ([]GameServer, error)
	TryJoin(id string, member string, max int) (int, bool, bool, error)
	Leave(id string, member string) (int, error)
	Members(id string) ([]string, error)
	Confirm(id string, member string) (int, error)
	Unconfirm(id string, member string) (int, error)
	SetConfirmed(id string, members []string) (int, error)
	ConfirmedMembers(id string) ([]string, error)
	ClaimPort(port int) (bool, error)
	ReleasePort(port int) error
	ClaimedPorts() ([]int, error)
	Ping() error
	Close() error
}

func gsKey(id string) string {
	return keyGS + id
}

func gameKey(gameID int) string {
	return fmt.Sprintf("%s%d", keyGame, gameID)
}

func membersKey(id string) string {
	return keyMembers + id
}

func confirmedKey(id string) string {
	return keyConfirmed + id
}

type redisStore struct {
	client *redis.Client
}

func newRedisStore(addr, password string, db int) (*redisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &redisStore{client: client}, nil
}

func NewStore(addr, password string, db int) Store {
	if rs, err := newRedisStore(addr, password, db); err == nil {
		return rs
	}
	return NewMemoryStore()
}

func NewMemoryStore() Store {
	return &memoryStore{
		servers:   make(map[string]GameServer),
		byGame:    make(map[int]map[string]struct{}),
		ports:     make(map[int]struct{}),
		members:   make(map[string]map[string]struct{}),
		confirmed: make(map[string]map[string]struct{}),
	}
}

func (s *redisStore) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	return s.client.Ping(ctx).Err()
}

func (s *redisStore) Close() error {
	return s.client.Close()
}

func (s *redisStore) Exists(id string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	n, err := s.client.Exists(ctx, gsKey(id)).Result()
	return n > 0, err
}

func (s *redisStore) Save(gs GameServer) error {
	raw, err := json.Marshal(gs)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	pipe := s.client.Pipeline()
	pipe.Set(ctx, gsKey(gs.ID), raw, 0)
	pipe.SAdd(ctx, keyAll, gs.ID)
	pipe.SAdd(ctx, gameKey(gs.GameID), gs.ID)
	pipe.SAdd(ctx, keyPorts, gs.Port)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *redisStore) hydrate(ctx context.Context, id string, raw []byte) (GameServer, error) {
	var gs GameServer
	if err := json.Unmarshal(raw, &gs); err != nil {
		return gs, err
	}
	if n, err := s.client.SCard(ctx, membersKey(id)).Result(); err == nil {
		gs.PlayerCount = int(n)
	}
	return gs, nil
}

func (s *redisStore) Get(id string) (GameServer, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	raw, err := s.client.Get(ctx, gsKey(id)).Bytes()
	if err == redis.Nil {
		return GameServer{}, false, nil
	}
	if err != nil {
		return GameServer{}, false, err
	}
	gs, err := s.hydrate(ctx, id, raw)
	if err != nil {
		return GameServer{}, false, err
	}
	return gs, true, nil
}

func (s *redisStore) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	raw, err := s.client.Get(ctx, gsKey(id)).Bytes()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	var gs GameServer
	if err := json.Unmarshal(raw, &gs); err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Del(ctx, gsKey(id))
	pipe.Del(ctx, membersKey(id))
	pipe.Del(ctx, confirmedKey(id))
	pipe.SRem(ctx, keyAll, id)
	pipe.SRem(ctx, gameKey(gs.GameID), id)
	pipe.SRem(ctx, keyPorts, gs.Port)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *redisStore) listIDs(ctx context.Context, key string) ([]GameServer, error) {
	ids, err := s.client.SMembers(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []GameServer{}, nil
	}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, gsKey(id))
	}
	raws, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	pipe := s.client.Pipeline()
	countCmds := make([]*redis.IntCmd, 0, len(ids))
	for _, id := range ids {
		countCmds = append(countCmds, pipe.SCard(ctx, membersKey(id)))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	out := make([]GameServer, 0, len(ids))
	for i, raw := range raws {
		str, ok := raw.(string)
		if !ok || str == "" {
			continue
		}
		var gs GameServer
		if err := json.Unmarshal([]byte(str), &gs); err != nil {
			continue
		}
		if i < len(countCmds) {
			if n, err := countCmds[i].Result(); err == nil {
				gs.PlayerCount = int(n)
			}
		}
		out = append(out, gs)
	}
	return out, nil
}

func (s *redisStore) ListByGame(gameID int) ([]GameServer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	return s.listIDs(ctx, gameKey(gameID))
}

func (s *redisStore) ListAll() ([]GameServer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	return s.listIDs(ctx, keyAll)
}

func (s *redisStore) TryJoin(id string, member string, max int) (int, bool, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	if exists, err := s.client.Exists(ctx, gsKey(id)).Result(); err != nil {
		return 0, false, false, err
	} else if exists == 0 {
		return 0, false, false, nil
	}
	added, err := s.client.SAdd(ctx, membersKey(id), member).Result()
	if err != nil {
		return 0, false, false, err
	}
	count, err := s.client.SCard(ctx, membersKey(id)).Result()
	if err != nil {
		return 0, false, false, err
	}
	if int(count) > max {
		_ = s.client.SRem(ctx, membersKey(id), member).Err()
		count, _ = s.client.SCard(ctx, membersKey(id)).Result()
		return int(count), false, false, nil
	}
	return int(count), true, added > 0, nil
}

func (s *redisStore) Leave(id string, member string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	if exists, err := s.client.Exists(ctx, gsKey(id)).Result(); err != nil {
		return 0, err
	} else if exists == 0 {
		return 0, nil
	}
	_ = s.client.SRem(ctx, membersKey(id), member).Err()
	n, err := s.client.SCard(ctx, membersKey(id)).Result()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *redisStore) Members(id string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	return s.client.SMembers(ctx, membersKey(id)).Result()
}

func (s *redisStore) Confirm(id string, member string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	if exists, err := s.client.Exists(ctx, gsKey(id)).Result(); err != nil {
		return 0, err
	} else if exists == 0 {
		return 0, nil
	}
	if err := s.client.SAdd(ctx, confirmedKey(id), member).Err(); err != nil {
		return 0, err
	}
	n, err := s.client.SCard(ctx, confirmedKey(id)).Result()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *redisStore) Unconfirm(id string, member string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	if exists, err := s.client.Exists(ctx, gsKey(id)).Result(); err != nil {
		return 0, err
	} else if exists == 0 {
		return 0, nil
	}
	_ = s.client.SRem(ctx, confirmedKey(id), member).Err()
	n, err := s.client.SCard(ctx, confirmedKey(id)).Result()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *redisStore) SetConfirmed(id string, members []string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	if exists, err := s.client.Exists(ctx, gsKey(id)).Result(); err != nil {
		return 0, err
	} else if exists == 0 {
		return 0, nil
	}
	pipe := s.client.Pipeline()
	pipe.Del(ctx, confirmedKey(id))
	if len(members) > 0 {
		args := make([]any, 0, len(members))
		for _, member := range members {
			args = append(args, member)
		}
		pipe.SAdd(ctx, confirmedKey(id), args...)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	n, err := s.client.SCard(ctx, confirmedKey(id)).Result()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *redisStore) ConfirmedMembers(id string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	return s.client.SMembers(ctx, confirmedKey(id)).Result()
}

func (s *redisStore) ClaimPort(port int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	n, err := s.client.SAdd(ctx, keyPorts, port).Result()
	return n > 0, err
}

func (s *redisStore) ReleasePort(port int) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	return s.client.SRem(ctx, keyPorts, port).Err()
}

func (s *redisStore) ClaimedPorts() ([]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	members, err := s.client.SMembers(ctx, keyPorts).Result()
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(members))
	for _, m := range members {
		var p int
		if _, err := fmt.Sscanf(m, "%d", &p); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

type memoryStore struct {
	mu        sync.Mutex
	servers   map[string]GameServer
	byGame    map[int]map[string]struct{}
	ports     map[int]struct{}
	members   map[string]map[string]struct{}
	confirmed map[string]map[string]struct{}
}

func (s *memoryStore) Ping() error {
	return nil
}

func (s *memoryStore) Close() error {
	return nil
}

func (s *memoryStore) Exists(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.servers[id]
	return ok, nil
}

func (s *memoryStore) Save(gs GameServer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	gs.PlayerCount = len(s.members[gs.ID])
	s.servers[gs.ID] = gs
	if s.byGame[gs.GameID] == nil {
		s.byGame[gs.GameID] = make(map[string]struct{})
	}
	s.byGame[gs.GameID][gs.ID] = struct{}{}
	s.ports[gs.Port] = struct{}{}
	return nil
}

func (s *memoryStore) Get(id string) (GameServer, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	gs, ok := s.servers[id]
	if ok {
		gs.PlayerCount = len(s.members[id])
	}
	return gs, ok, nil
}

func (s *memoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	gs, ok := s.servers[id]
	if !ok {
		return nil
	}
	delete(s.servers, id)
	delete(s.members, id)
	delete(s.confirmed, id)
	if set := s.byGame[gs.GameID]; set != nil {
		delete(set, id)
	}
	stillUsed := false
	for _, other := range s.servers {
		if other.Port == gs.Port {
			stillUsed = true
			break
		}
	}
	if !stillUsed {
		delete(s.ports, gs.Port)
	}
	return nil
}

func (s *memoryStore) ListByGame(gameID int) ([]GameServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := s.byGame[gameID]
	out := make([]GameServer, 0, len(set))
	for id := range set {
		if gs, ok := s.servers[id]; ok {
			gs.PlayerCount = len(s.members[id])
			out = append(out, gs)
		}
	}
	return out, nil
}

func (s *memoryStore) ListAll() ([]GameServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]GameServer, 0, len(s.servers))
	for id, gs := range s.servers {
		gs.PlayerCount = len(s.members[id])
		out = append(out, gs)
	}
	return out, nil
}

func (s *memoryStore) TryJoin(id string, member string, max int) (int, bool, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	gs, ok := s.servers[id]
	if !ok {
		return 0, false, false, nil
	}
	set := s.members[id]
	if set == nil {
		set = make(map[string]struct{})
		s.members[id] = set
	}
	_, isNew := set[member]
	if !isNew && len(set) >= max {
		return len(set), false, false, nil
	}
	set[member] = struct{}{}
	if len(set) > max {
		delete(set, member)
		return len(set), false, false, nil
	}
	gs.PlayerCount = len(set)
	gs.LastBeat = time.Now()
	if len(set) > 0 {
		gs.EmptySince = time.Time{}
	}
	s.servers[id] = gs
	return len(set), true, isNew, nil
}

func (s *memoryStore) Leave(id string, member string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	gs, ok := s.servers[id]
	if !ok {
		return 0, nil
	}
	if set := s.members[id]; set != nil {
		delete(set, member)
	}
	count := len(s.members[id])
	gs.PlayerCount = count
	if count == 0 && gs.EmptySince.IsZero() {
		gs.EmptySince = time.Now()
	}
	gs.LastBeat = time.Now()
	s.servers[id] = gs
	return count, nil
}

func (s *memoryStore) Members(id string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := s.members[id]
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	return out, nil
}

func (s *memoryStore) Confirm(id string, member string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.servers[id]; !ok {
		return 0, nil
	}
	set := s.confirmed[id]
	if set == nil {
		set = make(map[string]struct{})
		s.confirmed[id] = set
	}
	set[member] = struct{}{}
	return len(set), nil
}

func (s *memoryStore) Unconfirm(id string, member string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.servers[id]; !ok {
		return 0, nil
	}
	if set := s.confirmed[id]; set != nil {
		delete(set, member)
	}
	return len(s.confirmed[id]), nil
}

func (s *memoryStore) SetConfirmed(id string, members []string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.servers[id]; !ok {
		return 0, nil
	}
	set := make(map[string]struct{}, len(members))
	for _, member := range members {
		if member == "" {
			continue
		}
		set[member] = struct{}{}
	}
	s.confirmed[id] = set
	return len(set), nil
}

func (s *memoryStore) ConfirmedMembers(id string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := s.confirmed[id]
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	return out, nil
}

func (s *memoryStore) ClaimPort(port int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.ports[port]; ok {
		return false, nil
	}
	s.ports[port] = struct{}{}
	return true, nil
}

func (s *memoryStore) ReleasePort(port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stillUsed := false
	for _, gs := range s.servers {
		if gs.Port == port {
			stillUsed = true
			break
		}
	}
	if !stillUsed {
		delete(s.ports, port)
	}
	return nil
}

func (s *memoryStore) ClaimedPorts() ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]int, 0, len(s.ports))
	for p := range s.ports {
		out = append(out, p)
	}
	return out, nil
}
