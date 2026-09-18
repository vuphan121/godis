package cache

import (
	"math/rand"
	"sync"
)

type cacheShard struct {
	mu    sync.RWMutex
	items map[string]*entry
}

func newCacheShard() *cacheShard {
	return &cacheShard{items: make(map[string]*entry)}
}

func (s *cacheShard) get(key string) (*entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.items[key]
	return value, ok
}

func (s *cacheShard) set(key string, value *entry) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, replaced := s.items[key]
	s.items[key] = value
	return replaced
}

func (s *cacheShard) delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[key]; !ok {
		return false
	}
	delete(s.items, key)
	return true
}

func (s *cacheShard) deleteIf(key string, expected *entry) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.items[key]
	if !ok || current != expected {
		return false
	}
	delete(s.items, key)
	return true
}

func (s *cacheShard) len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

func (s *cacheShard) keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.items))
	for key := range s.items {
		keys = append(keys, key)
	}
	return keys
}

func (s *cacheShard) sample(limit int) []sampledEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || len(s.items) == 0 {
		return nil
	}
	if limit >= len(s.items) {
		entries := make([]sampledEntry, 0, len(s.items))
		for key, value := range s.items {
			entries = append(entries, sampledEntry{key: key, entry: value})
		}
		return entries
	}

	entries := make([]sampledEntry, 0, limit)
	seen := 0
	for key, value := range s.items {
		candidate := sampledEntry{key: key, entry: value}
		if seen < limit {
			entries = append(entries, candidate)
		} else if index := rand.Intn(seen + 1); index < limit {
			entries[index] = candidate
		}
		seen++
	}
	return entries
}
