package cache

import (
	"context"
	"reflect"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

type Loader[T any] func(ctx context.Context) (T, error)

type Cache[T any] struct {
	store *gocache.Cache
	group singleflight.Group
	ttl   time.Duration
}

func NewCache[T any](defaultExpiration, cleanupInterval time.Duration) *Cache[T] {
	return &Cache[T]{
		store: gocache.New(defaultExpiration, cleanupInterval),
		ttl:   defaultExpiration,
	}
}

func (c *Cache[T]) GetWithLoad(ctx context.Context, key string, load Loader[T]) (T, error) {
	var zero T
	if val, ok := c.store.Get(key); ok {
		return val.(T), nil
	}

	result, err, _ := c.group.Do(key, func() (interface{}, error) {
		if val, ok := c.store.Get(key); ok {
			return val.(T), nil
		}

		val, loadErr := load(ctx)
		if loadErr != nil {
			return zero, loadErr
		}
		if !isNilValue(val) {
			c.store.Set(key, val, c.ttl)
		}
		return val, nil
	})
	if err != nil {
		return zero, err
	}
	return result.(T), nil
}

func (c *Cache[T]) Set(key string, value T) {
	c.store.Set(key, value, c.ttl)
}

func (c *Cache[T]) Delete(key string) {
	c.store.Delete(key)
}

func isNilValue[T any](v T) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}
