package cache

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"
	"math"
)

type Cache[T any] struct {
	hasher   hash.Hash64
	cached   bool
	hash     uint64
	contents T
}

// Get has function that returns true if the new contents are valid and false if not
// (in that case it returns cached contents)
func (c *Cache[T]) Get(fn func() (T, bool), cacheValidationElements ...any) T {
	h := c.computeHash(cacheValidationElements...)

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
	c.cached = false
	c.hash = 0
	var zero T
	c.contents = zero
}

func (c *Cache[T]) computeHash(args ...any) uint64 {
	if c.hasher == nil {
		c.hasher = fnv.New64a()
	}
	c.hasher.Reset()
	for _, arg := range args {
		switch v := arg.(type) {
		case int:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], uint64(v))
			c.hasher.Write(buf[:])
		case string:
			c.hasher.Write([]byte(v))
		case uint64:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], v)
			c.hasher.Write(buf[:])
		case float64:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
			c.hasher.Write(buf[:])
		case bool:
			var b [1]byte
			if v {
				b[0] = 1
			}
			c.hasher.Write(b[:])
		default:
			fmt.Fprint(c.hasher, v)
		}
	}
	return c.hasher.Sum64()
}
