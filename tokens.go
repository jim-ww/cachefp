package cachefp

import (
	"sync"
	"time"
)

// WriteTokenSet is a self-expiring set used to authorize one-shot /write/:mid requests.
type WriteTokenSet struct {
	mu     sync.Mutex
	tokens map[string]struct{}
}

func NewWriteTokenSet() *WriteTokenSet {
	return &WriteTokenSet{tokens: map[string]struct{}{}}
}

func (s *WriteTokenSet) Generate() string {
	token := newUUID()
	s.mu.Lock()
	s.tokens[token] = struct{}{}
	s.mu.Unlock()
	time.AfterFunc(time.Minute, func() { s.Delete(token) })
	return token
}

func (s *WriteTokenSet) Has(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tokens[token]
	return ok
}

func (s *WriteTokenSet) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}
