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

func (p *AtomicPointer[T]) Update(fun func(*T)) {
	for {
		old := p.Pointer.Load()
		if old == nil {
			panic("atomicpointer: Update called on nil pointer")
		}

		copied := *old
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
	if err := json.Unmarshal(data, &val); err != nil {
		return errors.Wrap(err, "unmarshal atomic pointer")
	}

	p.Pointer.Store(&val)

	return nil
}
