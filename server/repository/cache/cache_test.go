package cache

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetWithLoadCachesValue(t *testing.T) {
	c := NewCache[string](time.Minute, 2*time.Minute)
	var loads int32

	load := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&loads, 1)
		return "value", nil
	}

	v1, err := c.GetWithLoad(context.Background(), "k1", load)
	require.NoError(t, err)
	assert.Equal(t, "value", v1)

	v2, err := c.GetWithLoad(context.Background(), "k1", load)
	require.NoError(t, err)
	assert.Equal(t, "value", v2)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loads))
}

func TestGetWithLoadConcurrentSingleLoad(t *testing.T) {
	c := NewCache[int](time.Minute, 2*time.Minute)
	var loads int32

	load := func(ctx context.Context) (int, error) {
		atomic.AddInt32(&loads, 1)
		time.Sleep(50 * time.Millisecond)
		return 42, nil
	}

	const workers = 20
	results := make(chan int, workers)
	for i := 0; i < workers; i++ {
		go func() {
			val, err := c.GetWithLoad(context.Background(), "k1", load)
			require.NoError(t, err)
			results <- val
		}()
	}

	for i := 0; i < workers; i++ {
		assert.Equal(t, 42, <-results)
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&loads))
}

func TestGetWithLoadDoesNotCacheLoadError(t *testing.T) {
	c := NewCache[string](time.Minute, 2*time.Minute)
	var loads int32

	load := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&loads, 1)
		return "", assert.AnError
	}

	_, err := c.GetWithLoad(context.Background(), "k1", load)
	require.Error(t, err)

	_, err = c.GetWithLoad(context.Background(), "k1", load)
	require.Error(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&loads))
}

func TestDeleteRemovesCachedValue(t *testing.T) {
	c := NewCache[string](time.Minute, 2*time.Minute)
	var loads int32

	load := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&loads, 1)
		return "value", nil
	}

	_, err := c.GetWithLoad(context.Background(), "k1", load)
	require.NoError(t, err)
	c.Delete("k1")

	_, err = c.GetWithLoad(context.Background(), "k1", load)
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&loads))
}
