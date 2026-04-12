package atomicpointer

import (
	"encoding/json"
	"sync/atomic"
)

type AtomicPointer[T any] struct {
	atomic.Pointer[T]
}

func New[T any](val *T) *AtomicPointer[T] {
	ap := &AtomicPointer[T]{}
	ap.Pointer.Store(val)
	return ap
}

func (p *AtomicPointer[T]) Clear() {
	var zero T
	p.Pointer.Store(&zero)
}

func (p *AtomicPointer[T]) Update(fn func(*T)) {
	for {
		old := p.Pointer.Load()
		if old == nil {
			panic("atomicpointer: Update called on nil pointer")
		}
		copied := *old
		fn(&copied)
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
	return json.Marshal(val)
}

func (p *AtomicPointer[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		p.Pointer.Store(nil)
		return nil
	}
	var val T
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	p.Pointer.Store(&val)
	return nil
}
