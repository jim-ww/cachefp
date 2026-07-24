package cachefp

import (
	"sync"
	"time"
)

// writeTokenSet is a self-expiring set used to authorize one-shot /write/:mid requests.
type writeTokenSet struct {
	mu     sync.Mutex
	tokens map[string]struct{}
}

func newWriteTokenSet() *writeTokenSet {
	return &writeTokenSet{tokens: map[string]struct{}{}}
}

func (s *writeTokenSet) Generate() string {
	token := newUUID()
	s.mu.Lock()
	s.tokens[token] = struct{}{}
	s.mu.Unlock()
	time.AfterFunc(time.Minute, func() { s.Delete(token) })
	return token
}

func (s *writeTokenSet) Has(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tokens[token]
	return ok
}

func (s *writeTokenSet) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}
