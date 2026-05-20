//go:build e2e
// +build e2e

package e2e

import (
	"hash/fnv"
	"os"
	"strconv"
	"strings"
)

// Sharding helpers (T5.2, robust-test-suite-plan).
//
// When TEST_SHARD is set to "N/M" (e.g., "3/8"), the table-driven E2E
// runner only executes cases whose example name hashes into shard N.
// Distribution is deterministic (fnv32a) so the same example always
// lands in the same shard — reruns are reproducible.

// shouldRunInShard returns true when the given example name belongs to
// the active shard. When TEST_SHARD is unset, all examples run (no
// sharding).
func shouldRunInShard(name string) bool {
	spec := os.Getenv("TEST_SHARD")
	if spec == "" {
		return true
	}
	n, m, ok := parseShard(spec)
	if !ok {
		// Malformed value — fail open (run everything) rather than
		// silently dropping tests. Better than 0 tests on a CI typo.
		return true
	}
	return shardFor(name, m) == n
}

func shardFor(name string, m int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int(h.Sum32() % uint32(m))
}

func parseShard(spec string) (n, m int, ok bool) {
	parts := strings.Split(spec, "/")
	if len(parts) != 2 {
		return 0, 0, false
	}
	var err error
	n, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	m, err = strconv.Atoi(parts[1])
	if err != nil || m <= 0 || n < 0 || n >= m {
		return 0, 0, false
	}
	return n, m, true
}
