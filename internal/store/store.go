package store

import (
	"sync"

	"github.com/sanchey92/go-cube/internal/services/task"
	"github.com/sanchey92/go-cube/pkg/errors"
)

type InMemoryStore struct {
	mu    sync.RWMutex
	data  map[string]*task.Task
	count int
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]*task.Task),
	}
}

func (s *InMemoryStore) Put(key string, value interface{}) error {
	t, ok := value.(*task.Task)
	if !ok {
		return errors.ErrValueExists
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = t
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
	var tasks []*task.Task
	for _, t := range s.data {
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *InMemoryStore) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count, nil
}
