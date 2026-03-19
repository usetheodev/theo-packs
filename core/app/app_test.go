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

func TestFindDirectories(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "src"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "dist"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "index.js"), []byte(""), 0644))

	testApp, err := NewApp(tempDir)
	require.NoError(t, err)

	dirs, err := testApp.FindDirectories("*")
	require.NoError(t, err)
	require.Len(t, dirs, 2)
	require.Contains(t, dirs, "src")
	require.Contains(t, dirs, "dist")
}

func TestHasMatch(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "public"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "public", "index.html"), []byte(""), 0644))

	testApp, err := NewApp(tempDir)
	require.NoError(t, err)

	require.True(t, testApp.HasMatch("public"))
	require.False(t, testApp.HasMatch("nonexistent"))
}

func TestReadYAML(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := "root: public\nport: 8080\n"
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(yamlContent), 0644))

	testApp, err := NewApp(tempDir)
	require.NoError(t, err)

	var config struct {
		Root string `yaml:"root"`
		Port int    `yaml:"port"`
	}
	err = testApp.ReadYAML("config.yaml", &config)
	require.NoError(t, err)
	require.Equal(t, "public", config.Root)
	require.Equal(t, 8080, config.Port)
}

func TestReadYAML_Error(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := NewApp(tempDir)
	require.NoError(t, err)

	var config struct{}
	err = testApp.ReadYAML("nonexistent.yaml", &config)
	require.Error(t, err)
}

func TestReadTOML(t *testing.T) {
	tempDir := t.TempDir()

	tomlContent := `[project]
name = "myapp"
version = "1.0.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "config.toml"), []byte(tomlContent), 0644))

	testApp, err := NewApp(tempDir)
	require.NoError(t, err)

	var config struct {
		Project struct {
			Name    string `toml:"name"`
			Version string `toml:"version"`
		} `toml:"project"`
	}
	err = testApp.ReadTOML("config.toml", &config)
	require.NoError(t, err)
	require.Equal(t, "myapp", config.Project.Name)
	require.Equal(t, "1.0.0", config.Project.Version)
}

func TestIsFileExecutable(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "script.sh"), []byte("#!/bin/bash\necho hi"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "data.txt"), []byte("hello"), 0644))

	testApp, err := NewApp(tempDir)
	require.NoError(t, err)

	require.True(t, testApp.IsFileExecutable("script.sh"))
	require.False(t, testApp.IsFileExecutable("data.txt"))
	require.False(t, testApp.IsFileExecutable("nonexistent"))
}

func TestReadFile_NonExistent(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := NewApp(tempDir)
	require.NoError(t, err)

	_, err = testApp.ReadFile("nonexistent.txt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "error reading")
}

func TestReadJSON_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "bad.json"), []byte("{invalid"), 0644))

	testApp, err := NewApp(tempDir)
	require.NoError(t, err)

	var result map[string]interface{}
	err = testApp.ReadJSON("bad.json", &result)
	require.Error(t, err)
}
