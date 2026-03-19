package generate

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/plan"
)

func TestNewCacheContext(t *testing.T) {
	ctx := NewCacheContext()
	require.NotNil(t, ctx)
	require.Empty(t, ctx.Caches)
}

func TestAddCache(t *testing.T) {
	ctx := NewCacheContext()
	name := ctx.AddCache("npm", "/root/.npm")

	require.Equal(t, "npm", name)
	require.NotNil(t, ctx.Caches["npm"])
	require.Equal(t, "/root/.npm", ctx.Caches["npm"].Directory)
	require.Equal(t, plan.CacheTypeShared, ctx.Caches["npm"].Type)
}

func TestAddCacheWithType(t *testing.T) {
	ctx := NewCacheContext()
	name := ctx.AddCacheWithType("pip", "/root/.cache/pip", plan.CacheTypeLocked)

	require.Equal(t, "pip", name)
	require.Equal(t, plan.CacheTypeLocked, ctx.Caches["pip"].Type)
}

func TestSanitizeCacheName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/root/.npm", "root-.npm"},
		{"/path/to/cache", "path-to-cache"},
		{"simple", "simple"},
		{"/leading/slash/", "leading-slash"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeCacheName(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAptCaches(t *testing.T) {
	ctx := NewCacheContext()
	caches := ctx.GetAptCaches()

	require.Len(t, caches, 2)
	require.Contains(t, caches, "apt")
	require.Contains(t, caches, "apt-lists")

	require.Equal(t, "/var/cache/apt", ctx.Caches["apt"].Directory)
	require.Equal(t, plan.CacheTypeLocked, ctx.Caches["apt"].Type)
	require.Equal(t, "/var/lib/apt/lists", ctx.Caches["apt-lists"].Directory)
	require.Equal(t, plan.CacheTypeLocked, ctx.Caches["apt-lists"].Type)
}

func TestGetAptCachesIdempotent(t *testing.T) {
	ctx := NewCacheContext()
	caches1 := ctx.GetAptCaches()
	caches2 := ctx.GetAptCaches()

	require.Equal(t, caches1, caches2)
	require.Len(t, ctx.Caches, 2)
}

func TestSetAndGetCache(t *testing.T) {
	ctx := NewCacheContext()

	require.Nil(t, ctx.GetCache("test"))

	cache := plan.NewCache("/tmp/test")
	ctx.SetCache("test", cache)

	require.Equal(t, cache, ctx.GetCache("test"))
}
