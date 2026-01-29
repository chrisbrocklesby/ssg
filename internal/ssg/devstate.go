package ssg

import (
	"sync"
)

type devState struct {
	mu  sync.RWMutex
	err error
}

func (s *devState) setErr(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

func (s *devState) getErr() error {
	s.mu.RLock()
	err := s.err
	s.mu.RUnlock()
	return err
}
