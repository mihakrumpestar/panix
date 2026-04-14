package cache

import (
	"fmt"
	"hash/fnv"
	"sync"
)

type Cache[T any] struct {
	mu       sync.Mutex
	cached   bool
	hash     uint64
	contents T
}

func New[T any]() *Cache[T] {
	return &Cache[T]{}
}

// Get has function that returns true if the new contents are valid and false if not
// (in that case it returns cached contents)
func (c *Cache[T]) Get(fn func() (T, bool), cacheValidationElements ...any) T {
	h := computeHash(cacheValidationElements...)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached && c.hash == h {
		return c.contents
	}

	contents, ok := fn()
	if !ok {
		return c.contents
	}

	c.cached = true
	c.hash = h
	c.contents = contents
	return contents
}

func (c *Cache[T]) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cached = false
	c.hash = 0
	var zero T
	c.contents = zero
}

func computeHash(args ...any) uint64 {
	h := fnv.New64a()
	for _, arg := range args {
		fmt.Fprint(h, arg)
	}
	return h.Sum64()
}
