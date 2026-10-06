package atomicpointer

import (
	"encoding/json"
	"sync/atomic"

	"github.com/pkg/errors"
)

type AtomicPointer[T any] struct {
	atomic.Pointer[T]
}

func New[T any]() *AtomicPointer[T] {
	ap := &AtomicPointer[T]{}

	var zero T
	ap.Pointer.Store(&zero)

	return ap
}

func (p *AtomicPointer[T]) Clear() {
	var zero T
	p.Pointer.Store(&zero)
}

// Update applies fun to a copy of the stored value and swaps the copy back,
// retrying while concurrent writers win the compare-and-swap. A nil stored
// pointer is legitimate (the JSON "null" round-trip state) and is handled as
// the zero value.
func (p *AtomicPointer[T]) Update(fun func(*T)) {
	for {
		old := p.Pointer.Load()

		var copied T
		if old != nil {
			copied = *old
		}

		fun(&copied)

		if p.Pointer.CompareAndSwap(old, &copied) {
			return
		}
	}
}

func (p *AtomicPointer[T]) MarshalJSON() ([]byte, error) {
	val := p.Pointer.Load()
	if val == nil {
		return []byte("null"), nil
	}

	b, err := json.Marshal(val)

	return b, errors.Wrap(err, "marshal atomic pointer")
}

func (p *AtomicPointer[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		p.Pointer.Store(nil)

		return nil
	}

	var val T

	err := json.Unmarshal(data, &val)
	if err != nil {
		return errors.Wrap(err, "unmarshal atomic pointer")
	}

	p.Pointer.Store(&val)

	return nil
}
