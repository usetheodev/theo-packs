package app

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApp(t *testing.T) {
	// Create a temp dir with a package.json for testing
	tempDir, err := os.MkdirTemp("", "app-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	packageJSON := `{"name": "test-app", "version": "1.0.0"}`
	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "index.ts"), []byte("console.log('hello')"), 0644)
	require.NoError(t, err)

	app, err := NewApp(tempDir)
	require.NoError(t, err)

	content, err := app.ReadFile("package.json")
	require.NoError(t, err)
	require.Contains(t, content, "test-app")

	type PackageJSON struct {
		Name string `json:"name"`
	}
	var pkg PackageJSON
	err = app.ReadJSON("package.json", &pkg)
	require.NoError(t, err)
	require.Equal(t, "test-app", pkg.Name)

	files, err := app.FindFiles("*.ts")
	require.NoError(t, err)
	require.Equal(t, 1, len(files))
	require.Equal(t, "index.ts", files[0])
}

func TestAppAbsolutePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "app-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	absPath, err := filepath.Abs(tempDir)
	require.NoError(t, err)

	app, err := NewApp(absPath)
	require.NoError(t, err)

	require.Equal(t, app.Source, absPath)
}

func TestAppReadJsonWithComments(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "app-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// JSONC content with comments
	jsonc := `{
		// this is a comment
		"hello": "world"
	}`
	err = os.WriteFile(filepath.Join(tempDir, "hello.jsonc"), []byte(jsonc), 0644)
	require.NoError(t, err)

	app, err := NewApp(tempDir)
	require.NoError(t, err)

	var config map[string]interface{}
	err = app.ReadJSON("hello.jsonc", &config)
	require.NoError(t, err)
	require.Equal(t, config["hello"], "world")
}

func TestFindFilesWithContent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "app-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "index.ts"), []byte("process.stdout.write('hello')"), 0644)
	require.NoError(t, err)

	app, err := NewApp(tempDir)
	require.NoError(t, err)

	regex := regexp.MustCompile(`process\.stdout\.write`)
	matches := app.FindFilesWithContent("*.ts", regex)
	require.Equal(t, 1, len(matches))
	require.Equal(t, "index.ts", matches[0])

	regex = regexp.MustCompile(`nonexistent`)
	matches = app.FindFilesWithContent("*.ts", regex)
	require.Empty(t, matches)

	regex = regexp.MustCompile(`test`)
	matches = app.FindFilesWithContent("[invalid", regex)
	require.Empty(t, matches)
}

func TestHasFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "app-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "exists.txt"), []byte("hello"), 0644)
	require.NoError(t, err)

	app, err := NewApp(tempDir)
	require.NoError(t, err)

	require.True(t, app.HasFile("exists.txt"))
	require.False(t, app.HasFile("doesnt-exist.txt"))
}

func TestNewAppNonExistentDir(t *testing.T) {
	_, err := NewApp("/nonexistent/directory")
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not exist")
}
