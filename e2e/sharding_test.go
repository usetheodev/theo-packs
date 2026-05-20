//go:build e2e
// +build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Distribution sanity: across ~48 examples + 8 shards, every shard
// must receive at least one example (no shard goes idle). fnv32a is
// not a cryptographic hash; small-sample distribution can be uneven
// — but we MUST avoid empty shards (those CI matrix slots would
// waste compute). A tighter "max ≤ Nx min" check is intentionally
// avoided for this corpus size; it surfaces noise more than signal.
func TestSharding_NoEmptyShards(t *testing.T) {
	t.Parallel()
	const M = 8
	counts := make(map[int]int)
	for _, c := range e2eCases() {
		counts[shardFor(c.example, M)]++
	}
	for i := 0; i < M; i++ {
		require.Greater(t, counts[i], 0,
			"shard %d/8 is empty (counts=%v) — would waste a CI matrix slot", i, counts)
	}
}

// Deterministic placement: hashing the same name twice must yield the
// same shard. Trivially true for fnv32a but guards against accidental
// rand.* introduction in the future.
func TestSharding_Deterministic(t *testing.T) {
	t.Parallel()
	for _, c := range e2eCases() {
		s1 := shardFor(c.example, 8)
		s2 := shardFor(c.example, 8)
		require.Equal(t, s1, s2, "non-deterministic shard for %q", c.example)
	}
}

func TestParseShard_Valid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     string
		n, m   int
		isOK   bool
	}{
		{"0/8", 0, 8, true},
		{"7/8", 7, 8, true},
		{"3/3", 0, 0, false}, // n must be < m
		{"-1/8", 0, 0, false},
		{"abc/8", 0, 0, false},
		{"1/2/3", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			n, m, ok := parseShard(c.in)
			require.Equal(t, c.isOK, ok)
			if c.isOK {
				require.Equal(t, c.n, n)
				require.Equal(t, c.m, m)
			}
		})
	}
}

func TestShouldRunInShard_UnsetEnv_RunsAll(t *testing.T) {
	t.Parallel()
	t.Setenv("TEST_SHARD", "")
	require.True(t, shouldRunInShard("any-example"))
}

func TestShouldRunInShard_MalformedFailsOpen(t *testing.T) {
	t.Parallel()
	t.Setenv("TEST_SHARD", "bogus")
	require.True(t, shouldRunInShard("any"))
}
