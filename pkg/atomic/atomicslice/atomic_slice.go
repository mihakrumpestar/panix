package atomicslice

import (
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
)

var (
	ErrAtomicSliceNil = errors.New("AtomicSlice.UnmarshalJSON: nil receiver")
)

type AtomicSlice[T any] struct {
	mu   sync.RWMutex
	data []T
}

func New[T any]() *AtomicSlice[T] {
	return &AtomicSlice[T]{
		data: make([]T, 0),
	}
}

func NewFrom[T any](items []T) *AtomicSlice[T] {
	s := New[T]()
	for _, item := range items {
		s.Append(item)
	}

	return s
}

func (s *AtomicSlice[T]) Append(value T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = append(s.data, value)
}

func (s *AtomicSlice[T]) Get(index int) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if index < 0 || index >= len(s.data) {
		var zero T

		return zero, false
	}

	return s.data[index], true
}

func (s *AtomicSlice[T]) Set(index int, value T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.data) {
		return false
	}

	s.data[index] = value

	return true
}

func (s *AtomicSlice[T]) Insert(index int, value T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index > len(s.data) {
		return false
	}

	s.data = append(s.data[:index], append([]T{value}, s.data[index:]...)...)

	return true
}

func (s *AtomicSlice[T]) Remove(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.data) {
		return false
	}

	s.data = append(s.data[:index], s.data[index+1:]...)

	return true
}

func (s *AtomicSlice[T]) Contains(value T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, v := range s.data {
		if any(v) == any(value) {
			return true
		}
	}

	return false
}

func (s *AtomicSlice[T]) Length() int {
	if s == nil {
		return 0
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.data)
}

func (s *AtomicSlice[T]) Last() (T, bool) {
	return s.Get(s.Length() - 1)
}

func (s *AtomicSlice[T]) Values() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]T, len(s.data))
	copy(result, s.data)

	return result
}

func (s *AtomicSlice[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = s.data[:0]
}

func (s *AtomicSlice[T]) Copy() *AtomicSlice[T] {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copied := make([]T, len(s.data))
	copy(copied, s.data)

	return &AtomicSlice[T]{
		data: copied,
	}
}

func (s *AtomicSlice[T]) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(s.Values())

	return b, errors.Wrap(err, "marshal atomic slice")
}

func (s *AtomicSlice[T]) UnmarshalJSON(data []byte) error {
	if s == nil {
		return ErrAtomicSliceNil
	}

	var items []T

	err := json.Unmarshal(data, &items)
	if err != nil {
		return errors.Wrap(err, "unmarshal atomic slice")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = s.data[:0]

	s.data = append(s.data, items...)

	return nil
}
