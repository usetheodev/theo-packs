package theoyaml

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse_SingleAppServer(t *testing.T) {
	t.Parallel()
	in := []byte(`
version: 1
project: demo
apps:
  api:
    path: .
    framework: express
    type: server
    port: 3000
`)
	cfg, err := Parse(in)
	require.NoError(t, err)
	require.Equal(t, 1, cfg.Version)
	app, ok := cfg.App("api")
	require.True(t, ok)
	require.Equal(t, TypeServer, app.Type)
	require.Equal(t, 3000, app.Port)
}

func TestParse_MonorepoTwoApps(t *testing.T) {
	t.Parallel()
	in := []byte(`
version: 1
project: demo
apps:
  api:
    path: apps/api
    framework: express
    type: server
    port: 3001
  web:
    path: apps/web
    framework: nextjs
    type: frontend
`)
	cfg, err := Parse(in)
	require.NoError(t, err)
	require.Len(t, cfg.Apps, 2)
	require.Equal(t, 3001, cfg.Apps["api"].Port)
	require.Equal(t, TypeFrontend, cfg.Apps["web"].Type)
}

func TestParse_DefaultsTypeToServer(t *testing.T) {
	t.Parallel()
	in := []byte(`
version: 1
apps:
  api:
    port: 8080
`)
	cfg, err := Parse(in)
	require.NoError(t, err)
	require.Equal(t, TypeServer, cfg.Apps["api"].Type)
}

func TestParse_RejectsMalformed(t *testing.T) {
	t.Parallel()
	_, err := Parse([]byte("not: valid: yaml: ::"))
	require.Error(t, err)
}

func TestParse_RejectsEmpty(t *testing.T) {
	t.Parallel()
	_, err := Parse(nil)
	require.Error(t, err)
}

func TestApp_SingleAppEmptyName(t *testing.T) {
	t.Parallel()
	in := []byte(`
version: 1
apps:
  api:
    port: 8080
`)
	cfg, _ := Parse(in)
	app, ok := cfg.App("")
	require.True(t, ok, "single-app config should return the lone app on empty name")
	require.Equal(t, 8080, app.Port)
}

func TestApp_MultiAppEmptyName(t *testing.T) {
	t.Parallel()
	in := []byte(`
version: 1
apps:
  a:
    port: 1
  b:
    port: 2
`)
	cfg, _ := Parse(in)
	_, ok := cfg.App("")
	require.False(t, ok, "multi-app config must require explicit name")
}
