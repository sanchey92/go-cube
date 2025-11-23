package store

import (
	"sync"

	"github.com/sanchey92/go-cube/pkg/errors"
)

type InMemoryStore struct {
	mu    sync.RWMutex
	data  map[string]interface{}
	count int
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]interface{}),
	}
}

func (s *InMemoryStore) Put(key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	s.count++
	return nil
}

func (s *InMemoryStore) Get(key string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.data[key]
	if !ok {
		return nil, errors.ErrValueNotExists
	}

	return t, nil
}

func (s *InMemoryStore) List() (interface{}, error) {
	s.mu.Lock()
	defer s.mu.RLock()
	var data []interface{}
	for _, t := range s.data {
		data = append(data, t)
	}
	return data, nil
}

func (s *InMemoryStore) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count, nil
}
