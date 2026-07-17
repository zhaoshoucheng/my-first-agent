package task

import (
	"context"
	"fmt"
	"sync"

	"github.com/shoucheng/my-first-agent/internal/event"
)

// InMemoryStore Store 的内存实现：进程退出即丢，先把接口跑起来。
type InMemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*Task
	seq   int64
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{tasks: make(map[string]*Task)}
}

func (s *InMemoryStore) Create(ctx context.Context) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	t := &Task{ID: fmt.Sprintf("task_%d", s.seq)}
	s.tasks[t.ID] = t
	return t, nil
}

func (s *InMemoryStore) Get(ctx context.Context, taskID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task.Get: task %s not found", taskID)
	}
	return t, nil
}

func (s *InMemoryStore) Append(ctx context.Context, taskID string, events ...event.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task.Append: task %s not found", taskID)
	}
	t.Events = append(t.Events, events...)
	return nil
}
