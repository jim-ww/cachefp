package cachefp

import (
	"slices"
	"sync"
)

// profile tracks one visitor across the write (encode) or read (decode) flow.
type profile struct {
	mu          sync.Mutex
	uid         string
	vector      []string
	identifier  uint64
	hasID       bool // identifier was supplied at creation (write mode)
	visited     map[string]bool
	storageSize int
}

// profileStore is a concurrency-safe registry of in-flight profiles, keyed by uid.
type profileStore struct {
	mu    sync.Mutex
	byUID map[string]*profile
}

func newProfileStore() *profileStore {
	return &profileStore{byUID: map[string]*profile{}}
}

func (s *profileStore) Get(uid string) *profile {
	if uid == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byUID[uid]
}

func (s *profileStore) Has(uid string) bool {
	return s.Get(uid) != nil
}

// FromWrite creates a profile bound to a known identifier (write mode).
// Returns nil if uid is already registered.
func (s *profileStore) FromWrite(uid string, identifier uint64, routes *routeTable) *profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byUID[uid]; exists {
		return nil
	}
	p := &profile{
		uid:        uid,
		identifier: identifier,
		hasID:      true,
		vector:     routes.Vector(uint32(identifier)),
		visited:    map[string]bool{},
	}
	s.byUID[uid] = p
	return p
}

// FromRead creates a profile with no known identifier yet (read mode).
// Returns nil if uid is already registered.
func (s *profileStore) FromRead(uid string) *profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byUID[uid]; exists {
		return nil
	}
	p := &profile{uid: uid, visited: map[string]bool{}}
	s.byUID[uid] = p
	return p
}

func (p *profile) IsReading() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.hasID
}

func (p *profile) Vector() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.vector
}

func (p *profile) Identifier() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.identifier
}

func (p *profile) VisitRoute(route string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.visited[route] = true
}

func (p *profile) HasVisited(route string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.visited[route]
}

func (p *profile) VisitedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.visited)
}

func (p *profile) SetStorageSize(size int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.storageSize = size
}

func (p *profile) StorageSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.storageSize
}

// CalcIdentifier decodes the identifier from visited routes (read mode) and caches it.
func (p *profile) CalcIdentifier(routes *routeTable) uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.identifier = routes.Identifier(p.visited, p.storageSize)
	p.hasID = true
	return p.identifier
}

func (p *profile) VectorContains(route string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Contains(p.vector, route)
}

func (s *profileStore) Delete(uid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byUID, uid)
}
