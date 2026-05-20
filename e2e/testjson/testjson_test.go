package testjson

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse_JustBuild_Default(t *testing.T) {
	t.Parallel()
	cfg, err := Parse([]byte(`{}`))
	require.NoError(t, err)
	require.Equal(t, ModeJustBuild, cfg.Mode)
}

func TestParse_HTTPCheck_Valid(t *testing.T) {
	t.Parallel()
	in := `{
		// JSONC comment is allowed
		"mode": "httpCheck",
		"httpCheck": { "path": "/health", "port": 8080, "expectedStatus": 200 },
		"env": { "THEOPACKS_APP_NAME": "api" }
	}`
	cfg, err := Parse([]byte(in))
	require.NoError(t, err)
	require.Equal(t, ModeHTTPCheck, cfg.Mode)
	require.Equal(t, "/health", cfg.HTTPCheck.Path)
	require.Equal(t, 8080, cfg.HTTPCheck.Port)
	require.Equal(t, 200, cfg.HTTPCheck.ExpectedStatus)
	require.Equal(t, "api", cfg.Env["THEOPACKS_APP_NAME"])
}

func TestParse_ExpectedOutput_Valid(t *testing.T) {
	t.Parallel()
	in := `{
		"mode": "expectedOutput",
		"expectedOutput": { "args": ["--version"], "mustContain": "v1." }
	}`
	cfg, err := Parse([]byte(in))
	require.NoError(t, err)
	require.Equal(t, ModeExpectedOutput, cfg.Mode)
	require.Equal(t, []string{"--version"}, cfg.ExpectedOutput.Args)
}

func TestParse_RejectsUnknownMode(t *testing.T) {
	t.Parallel()
	_, err := Parse([]byte(`{"mode": "magic"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown mode")
}

func TestParse_RejectsHTTPCheckWithoutBlock(t *testing.T) {
	t.Parallel()
	_, err := Parse([]byte(`{"mode": "httpCheck"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "httpCheck block")
}

func TestParse_RejectsBadPort(t *testing.T) {
	t.Parallel()
	in := `{"mode":"httpCheck","httpCheck":{"path":"/","port":0,"expectedStatus":200}}`
	_, err := Parse([]byte(in))
	require.Error(t, err)
	require.Contains(t, err.Error(), "port")
}

func TestParse_RejectsBadStatus(t *testing.T) {
	t.Parallel()
	in := `{"mode":"httpCheck","httpCheck":{"path":"/","port":80,"expectedStatus":999}}`
	_, err := Parse([]byte(in))
	require.Error(t, err)
	require.Contains(t, err.Error(), "expectedStatus")
}

func TestParse_Skip(t *testing.T) {
	t.Parallel()
	in := `{"mode":"justBuild","skip":true,"skipReason":"flaky on darwin"}`
	cfg, err := Parse([]byte(in))
	require.NoError(t, err)
	require.True(t, cfg.Skip)
	require.Equal(t, "flaky on darwin", cfg.SkipReason)
}
