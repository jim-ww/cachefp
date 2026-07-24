package cachefp

import (
	"encoding/json"
	"os"
	"sync"
)

// Store persists the middleware's small amount of durable state: the
// per-deployment cache ID and the next write index.
// Both must survive process restarts, or previously-cached probe URLs in
// visitors' browsers stop matching anything the server expects.
//
// Get/Set values are limited to what encoding/json's default decoding
// produces (so implementations backed by JSON, like JSONStore, round-trip
// cleanly): strings and float64 for the numeric index. A custom Store may
// use a real integer type internally as long as GetUint64 converts it.
type Store interface {
	Get(key string) (value any, ok bool)
	Set(key string, value any)
}

func getString(s Store, key, fallback string) string {
	v, ok := s.Get(key)
	if !ok {
		return fallback
	}
	if str, ok := v.(string); ok {
		return str
	}
	return fallback
}

func getUint64(s Store, key string, fallback uint64) uint64 {
	v, ok := s.Get(key)
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return uint64(n)
	case uint64:
		return n
	case int:
		return uint64(n)
	}
	return fallback
}

// JSONStore is the default Store: it persists state as a small JSON file.
type JSONStore struct {
	mu   sync.Mutex
	path string
	data map[string]any
}

// NewJSONStore opens (or creates) the JSON file at path as a Store.
func NewJSONStore(path string) *JSONStore {
	s := &JSONStore{path: path, data: map[string]any{}}
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &s.data)
	} else {
		s.save()
	}
	return s
}

func (s *JSONStore) save() {
	b, _ := json.MarshalIndent(s.data, "", "\t")
	_ = os.WriteFile(s.path, b, 0o644)
}

func (s *JSONStore) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *JSONStore) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	s.save()
}

type InMemoryStore sync.Map

func (s *InMemoryStore) Get(key string) (any, bool) {
	return (*sync.Map)(s).Load(key)
}

func (s *InMemoryStore) Set(key string, value any) {
	(*sync.Map)(s).Store(key, value)
}
