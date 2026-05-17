package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// IdempotencyStore 内存幂等缓存（进程内；多副本需外置）。
type IdempotencyStore struct {
	mu  sync.Mutex
	m   map[string]*idempoEntry
	ttl time.Duration
}

type idempoEntry struct {
	StatusCode int
	Body       []byte
	Expires    time.Time
	Hash       string
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	if ttl < time.Minute {
		ttl = 10 * time.Minute
	}
	return &IdempotencyStore{m: make(map[string]*idempoEntry), ttl: ttl}
}

func hashBody(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Lookup 若存在相同 key 且 body hash 一致，返回 true 及缓存响应。
func (s *IdempotencyStore) Lookup(key string, body []byte) (status int, bodyOut []byte, ok bool) {
	if s == nil || key == "" {
		return 0, nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	e, ok := s.m[key]
	if !ok || e == nil {
		return 0, nil, false
	}
	if time.Now().After(e.Expires) {
		delete(s.m, key)
		return 0, nil, false
	}
	if e.Hash != hashBody(body) {
		return 0, nil, false
	}
	return e.StatusCode, append([]byte(nil), e.Body...), true
}

func (s *IdempotencyStore) Store(key string, body []byte, status int, respBody []byte) {
	if s == nil || key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = &idempoEntry{
		StatusCode: status,
		Body:       append([]byte(nil), respBody...),
		Expires:    time.Now().Add(s.ttl),
		Hash:       hashBody(body),
	}
}

func (s *IdempotencyStore) pruneLocked() {
	now := time.Now()
	for k, e := range s.m {
		if e == nil || now.After(e.Expires) {
			delete(s.m, k)
		}
	}
}

func idempotencyKey(r *http.Request) string {
	return r.Header.Get("Idempotency-Key")
}

func replayJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Idempotency-Replayed", "true")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
