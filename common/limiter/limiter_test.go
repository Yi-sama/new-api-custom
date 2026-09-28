package limiter

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestAllowRecoversAfterScriptFlush(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx := context.Background()
	limiter := &RedisLimiter{client: client}

	allowed, err := limiter.Allow(ctx, "rate-limit:before-flush")
	require.NoError(t, err)
	require.True(t, allowed)

	require.NoError(t, client.ScriptFlush(ctx).Err())

	allowed, err = limiter.Allow(ctx, "rate-limit:after-flush")
	require.NoError(t, err)
	require.True(t, allowed)
}
