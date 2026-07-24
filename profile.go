package cachefp

import (
	"slices"
	"sync"
)

// Profile tracks one visitor across the write (encode) or read (decode) flow.
type Profile struct {
	mu          sync.Mutex
	uid         string
	vector      []string
	identifier  uint64
	hasID       bool // identifier was supplied at creation (write mode)
	visited     map[string]bool
	storageSize int
}

// ProfileStore is a concurrency-safe registry of in-flight profiles, keyed by uid.
type ProfileStore struct {
	mu    sync.Mutex
	byUID map[string]*Profile
}

func NewProfileStore() *ProfileStore {
	return &ProfileStore{byUID: map[string]*Profile{}}
}

func (s *ProfileStore) Get(uid string) *Profile {
	if uid == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byUID[uid]
}

func (s *ProfileStore) Has(uid string) bool {
	return s.Get(uid) != nil
}

// FromWrite creates a profile bound to a known identifier (write mode).
// Returns nil if uid is already registered.
func (s *ProfileStore) FromWrite(uid string, identifier uint64, routes *RouteTable) *Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byUID[uid]; exists {
		return nil
	}
	p := &Profile{
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
func (s *ProfileStore) FromRead(uid string) *Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byUID[uid]; exists {
		return nil
	}
	p := &Profile{uid: uid, visited: map[string]bool{}}
	s.byUID[uid] = p
	return p
}

func (p *Profile) IsReading() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.hasID
}

func (p *Profile) Vector() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.vector
}

func (p *Profile) Identifier() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.identifier
}

func (p *Profile) VisitRoute(route string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.visited[route] = true
}

func (p *Profile) HasVisited(route string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.visited[route]
}

func (p *Profile) VisitedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.visited)
}

func (p *Profile) SetStorageSize(size int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.storageSize = size
}

func (p *Profile) StorageSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.storageSize
}

// CalcIdentifier decodes the identifier from visited routes (read mode) and caches it.
func (p *Profile) CalcIdentifier(routes *RouteTable) uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.identifier = routes.Identifier(p.visited, p.storageSize)
	p.hasID = true
	return p.identifier
}

func (p *Profile) VectorContains(route string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Contains(p.vector, route)
}

func (s *ProfileStore) Delete(uid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byUID, uid)
}
