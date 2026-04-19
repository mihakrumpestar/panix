package cache

type Cache[T any, K comparable] struct {
	cached   bool
	key      K
	contents T
}

// Get returns the cached contents if key matches the previously stored key.
// Otherwise, it calls fun to produce new contents, caches them under key, and returns them.
// If fun returns ok == false, the previous cached contents are returned without updating.
func (c *Cache[T, K]) Get(fun func() (T, bool), key K) T {
	if c.cached && c.key == key {
		return c.contents
	}

	contents, ok := fun()
	if !ok {
		return c.contents
	}

	c.cached = true
	c.key = key
	c.contents = contents

	return contents
}
