package generate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	m := NewMetadata()
	require.NotNil(t, m)
	require.Empty(t, m.Properties)

	m.Set("key", "value")
	require.Equal(t, "value", m.Get("key"))

	m.Set("empty", "")
	require.Empty(t, m.Get("empty"), "empty values should not be set")

	require.Empty(t, m.Get("nonexistent"))
}

func TestMetadataSetBool(t *testing.T) {
	m := NewMetadata()

	m.SetBool("enabled", true)
	require.Equal(t, "true", m.Get("enabled"))

	m.SetBool("disabled", false)
	require.Empty(t, m.Get("disabled"), "false values should not be set")
}
