package atomicslice

import (
	"encoding/json"

	"github.com/hayageek/threadsafe"
	"github.com/pkg/errors"
)

type AtomicSlice[T any] struct {
	*threadsafe.Slice[T]
}

func New[T any]() *AtomicSlice[T] {
	return &AtomicSlice[T]{
		Slice: threadsafe.NewSlice[T](),
	}
}

func NewFrom[T any](items []T) *AtomicSlice[T] {
	s := New[T]()
	for _, item := range items {
		s.Append(item)
	}

	return s
}

func (s *AtomicSlice[T]) Last() (T, bool) {
	return s.Get(s.Length() - 1)
}

func (s *AtomicSlice[T]) Copy() *AtomicSlice[T] {
	return &AtomicSlice[T]{
		Slice: s.Slice.Copy(),
	}
}

func (s *AtomicSlice[T]) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(s.Values())

	return b, errors.Wrap(err, "marshal atomic slice")
}

func (s *AtomicSlice[T]) UnmarshalJSON(data []byte) error {
	var items []T

	err := json.Unmarshal(data, &items)
	if err != nil {
		return errors.Wrap(err, "unmarshal atomic slice")
	}

	if s.Slice == nil {
		s.Slice = threadsafe.NewSlice[T]()
	} else {
		s.Clear()
	}

	for _, item := range items {
		s.Append(item)
	}

	return nil
}
