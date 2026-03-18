package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
)

func TestCheckAndParseDockerignore(t *testing.T) {
	t.Run("nonexistent dockerignore", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dockerignore-test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)

		excludes, includes, err := CheckAndParseDockerignore(testApp)
		require.NoError(t, err)
		require.Nil(t, excludes)
		require.Nil(t, includes)
	})

	t.Run("valid dockerignore file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dockerignore-test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		// Create test files for negation patterns
		err = os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "src", "main.go"), []byte("package main"), 0644)
		require.NoError(t, err)

		dockerignoreContent := `node_modules
*.log
.env*
!src/main.go
`
		err = os.WriteFile(filepath.Join(tempDir, ".dockerignore"), []byte(dockerignoreContent), 0644)
		require.NoError(t, err)

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)

		excludes, includes, err := CheckAndParseDockerignore(testApp)
		require.NoError(t, err)
		require.NotNil(t, excludes)
		require.Contains(t, excludes, "node_modules")
		require.Contains(t, excludes, "*.log")
		require.Contains(t, excludes, ".env*")
		require.Contains(t, includes, "src/main.go")
	})

	t.Run("inaccessible dockerignore", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dockerignore-test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		dockerignorePath := filepath.Join(tempDir, ".dockerignore")
		err = os.WriteFile(dockerignorePath, []byte("*.log\nnode_modules\n"), 0644)
		require.NoError(t, err)

		err = os.Chmod(dockerignorePath, 0000)
		require.NoError(t, err)
		defer func() { _ = os.Chmod(dockerignorePath, 0644) }()

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)

		excludes, includes, err := CheckAndParseDockerignore(testApp)
		require.Error(t, err)
		require.Contains(t, err.Error(), "error reading .dockerignore")
		require.Nil(t, excludes)
		require.Nil(t, includes)
	})
}

func TestSeparatePatterns(t *testing.T) {
	t.Run("only exclude patterns", func(t *testing.T) {
		patterns := []string{"*.log", "node_modules", "/tmp"}
		excludes, includes := separatePatterns(patterns)

		require.Equal(t, patterns, excludes)
		require.Empty(t, includes)
	})

	t.Run("only include patterns", func(t *testing.T) {
		patterns := []string{"!important.log", "!keep/this"}
		excludes, includes := separatePatterns(patterns)

		require.Empty(t, excludes)
		require.Equal(t, []string{"important.log", "keep/this"}, includes)
	})

	t.Run("mixed patterns", func(t *testing.T) {
		patterns := []string{"*.log", "!important.log", "node_modules", "!node_modules/keep"}
		excludes, includes := separatePatterns(patterns)

		require.Equal(t, []string{"*.log", "node_modules"}, excludes)
		require.Equal(t, []string{"important.log", "node_modules/keep"}, includes)
	})

	t.Run("empty patterns", func(t *testing.T) {
		patterns := []string{}
		excludes, includes := separatePatterns(patterns)

		require.Empty(t, excludes)
		require.Empty(t, includes)
	})

	t.Run("empty string patterns", func(t *testing.T) {
		patterns := []string{"", "*.log", "", "!keep.log"}
		excludes, includes := separatePatterns(patterns)

		require.Equal(t, []string{"", "*.log", ""}, excludes)
		require.Equal(t, []string{"keep.log"}, includes)
	})
}

func TestDockerignoreContext(t *testing.T) {
	t.Run("new context without dockerignore", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dockerignore-test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)

		ctx, err := NewDockerignoreContext(testApp)
		require.NoError(t, err)
		require.NotNil(t, ctx)
		require.False(t, ctx.HasFile)
		require.Nil(t, ctx.Excludes)
		require.Nil(t, ctx.Includes)
	})

	t.Run("context with dockerignore file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dockerignore-test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		err = os.WriteFile(filepath.Join(tempDir, "keep.txt"), []byte("exists"), 0644)
		require.NoError(t, err)

		dockerignoreContent := `*.log
node_modules
!keep.txt
`
		err = os.WriteFile(filepath.Join(tempDir, ".dockerignore"), []byte(dockerignoreContent), 0644)
		require.NoError(t, err)

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)

		ctx, err := NewDockerignoreContext(testApp)
		require.NoError(t, err)
		require.True(t, ctx.HasFile)
		require.NotNil(t, ctx.Excludes)
		require.NotNil(t, ctx.Includes)
		require.Contains(t, ctx.Includes, "keep.txt")
	})

	t.Run("parse error handling", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dockerignore-test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		dockerignorePath := filepath.Join(tempDir, ".dockerignore")
		err = os.WriteFile(dockerignorePath, []byte("*.log\n"), 0644)
		require.NoError(t, err)

		err = os.Chmod(dockerignorePath, 0000)
		require.NoError(t, err)
		defer func() { _ = os.Chmod(dockerignorePath, 0644) }()

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)

		ctx, err := NewDockerignoreContext(testApp)
		require.Error(t, err)
		require.Nil(t, ctx)
	})
}

func TestDockerignoreDuplicatePatterns(t *testing.T) {
	t.Run("duplicate patterns removed", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dockerignore-test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		err = os.WriteFile(filepath.Join(tempDir, "keep.txt"), []byte("exists"), 0644)
		require.NoError(t, err)

		dockerignoreContent := `*.log
*.log
node_modules
!keep.txt
!keep.txt
`
		err = os.WriteFile(filepath.Join(tempDir, ".dockerignore"), []byte(dockerignoreContent), 0644)
		require.NoError(t, err)

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)

		ctx, err := NewDockerignoreContext(testApp)
		require.NoError(t, err)

		logCount := 0
		nodeModulesCount := 0
		for _, pattern := range ctx.Excludes {
			if pattern == "*.log" {
				logCount++
			}
			if pattern == "node_modules" {
				nodeModulesCount++
			}
		}

		keepCount := 0
		for _, pattern := range ctx.Includes {
			if pattern == "keep.txt" {
				keepCount++
			}
		}

		require.LessOrEqual(t, logCount, 1, "*.log pattern should appear at most once")
		require.LessOrEqual(t, nodeModulesCount, 1, "node_modules pattern should appear at most once")
		require.LessOrEqual(t, keepCount, 1, "keep.txt pattern should appear at most once")
	})
}

func TestCheckAndParseDockerignoreWithNegation(t *testing.T) {
	t.Run("negated patterns with existing and non-existing files and folders", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dockerignore-test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		err = os.MkdirAll(filepath.Join(tempDir, "negation_test", "existing_folder"), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "negation_test", "should_exist.txt"), []byte("exists"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "negation_test", "existing_folder", "file.txt"), []byte("exists"), 0644)
		require.NoError(t, err)

		dockerignoreContent := `
negation_test/*
!negation_test/should_exist.txt
!negation_test/should_not_exist.txt
!negation_test/folder_does_not_exist/
!negation_test/existing_folder/
`
		err = os.WriteFile(filepath.Join(tempDir, ".dockerignore"), []byte(dockerignoreContent), 0644)
		require.NoError(t, err)

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)

		excludes, includes, err := CheckAndParseDockerignore(testApp)
		require.NoError(t, err)

		require.Contains(t, excludes, "negation_test/*")
		require.Contains(t, includes, "negation_test/should_exist.txt")
		require.Contains(t, includes, "negation_test/existing_folder")
		require.NotContains(t, includes, "negation_test/should_not_exist.txt")
		require.NotContains(t, includes, "negation_test/folder_does_not_exist/")
		require.Len(t, includes, 2)
	})
}
