package cache

import (
	"time"

	"github.com/vuphan121/godis/config"
)

type TypedCache[V any] struct {
	inner *Cache
}

type typedValue[V any] struct {
	value V
}

func NewTypedCache[V any](options ...config.Option) (*TypedCache[V], error) {
	inner, err := NewCache(options...)
	if err != nil {
		return nil, err
	}
	return &TypedCache[V]{inner: inner}, nil
}

func (c *TypedCache[V]) Set(key string, value V, ttl ...time.Duration) error {
	return c.inner.Set(key, typedValue[V]{value: value}, ttl...)
}

func (c *TypedCache[V]) Get(key string) (V, bool) {
	stored, ok := c.inner.Get(key)
	if !ok {
		var zero V
		return zero, false
	}
	wrapped, ok := stored.(typedValue[V])
	if !ok {
		var zero V
		return zero, false
	}
	return wrapped.value, true
}

func (c *TypedCache[V]) Delete(key string) bool {
	return c.inner.Delete(key)
}

func (c *TypedCache[V]) IsHotKey(key string) bool {
	return c.inner.IsHotKey(key)
}

func (c *TypedCache[V]) Size() int {
	return c.inner.Size()
}

func (c *TypedCache[V]) Stats() Stats {
	return c.inner.Stats()
}

func (c *TypedCache[V]) Close() {
	c.inner.Close()
}
