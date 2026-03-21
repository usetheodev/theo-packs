package dockerfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func goldenDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(thisFile), "testdata")
}

func assertGolden(t *testing.T, name string, got string) {
	t.Helper()

	path := filepath.Join(goldenDir(t), name)

	if os.Getenv("UPDATE_GOLDEN") == "true" {
		err := os.WriteFile(path, []byte(got), 0644)
		require.NoError(t, err, "failed to update golden file %s", name)
		return
	}

	expected, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf("golden file %s does not exist — run with UPDATE_GOLDEN=true to create it", name)
	}
	require.NoError(t, err)
	require.Equal(t, string(expected), got, "output does not match golden file %s", name)
}
