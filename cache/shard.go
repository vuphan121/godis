package cache

import "sync"

type CacheShard struct {
	items    map[string]*Entry
	lock     sync.RWMutex
	keyCount int
}

func NewCacheShard() *CacheShard {
	return &CacheShard{
		items: make(map[string]*Entry),
	}
}

func (s *CacheShard) Get(key string) (*Entry, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	e, ok := s.items[key]
	return e, ok
}

func (s *CacheShard) Set(key string, entry *Entry) {
	s.lock.Lock()

	if _, exists := s.items[key]; !exists {
		s.keyCount++
	}
	s.items[key] = entry

	s.lock.Unlock()
}

func (s *CacheShard) Delete(key string) {
	s.lock.Lock()

	if _, exists := s.items[key]; exists {
		delete(s.items, key)
		s.keyCount--
	}

	s.lock.Unlock()
}
