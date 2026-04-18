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
// (in that case it returns cached contents).
func (c *Cache[T]) Get(fun func() (T, bool), cacheValidationElements ...any) T {
	hash := c.computeHash(cacheValidationElements...)

	if c.cached && c.hash == hash {
		return c.contents
	}

	contents, ok := fun()
	if !ok {
		return c.contents
	}

	c.cached = true
	c.hash = hash
	c.contents = contents

	return contents
}

func (c *Cache[T]) computeHash(args ...any) uint64 {
	if c.hasher == nil {
		c.hasher = fnv.New64a()
	}

	c.hasher.Reset()

	for _, arg := range args {
		switch argType := arg.(type) {
		case int:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], uint64(argType)) //nolint:gosec // intentional conversion for hashing
			_, _ = c.hasher.Write(buf[:])
		case string:
			_, _ = c.hasher.Write([]byte(argType))
		case uint64:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], argType)
			_, _ = c.hasher.Write(buf[:])
		case float64:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], math.Float64bits(argType))
			_, _ = c.hasher.Write(buf[:])
		case bool:
			var b [1]byte
			if argType {
				b[0] = 1
			}

			_, _ = c.hasher.Write(b[:])
		default:
			_, _ = fmt.Fprint(c.hasher, argType)
		}
	}

	return c.hasher.Sum64()
}
