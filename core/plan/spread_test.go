package plan

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpreadStrings(t *testing.T) {
	t.Run("nil left returns right", func(t *testing.T) {
		result := SpreadStrings(nil, []string{"a", "b"})
		require.Equal(t, []string{"a", "b"}, result)
	})

	t.Run("no spread marker", func(t *testing.T) {
		result := SpreadStrings([]string{"x", "y"}, []string{"a", "b"})
		require.Equal(t, []string{"x", "y"}, result)
	})

	t.Run("spread at beginning", func(t *testing.T) {
		result := SpreadStrings([]string{"...", "x"}, []string{"a", "b"})
		require.Equal(t, []string{"a", "b", "x"}, result)
	})

	t.Run("spread at end", func(t *testing.T) {
		result := SpreadStrings([]string{"x", "..."}, []string{"a", "b"})
		require.Equal(t, []string{"x", "a", "b"}, result)
	})

	t.Run("spread in middle", func(t *testing.T) {
		result := SpreadStrings([]string{"x", "...", "y"}, []string{"a", "b"})
		require.Equal(t, []string{"x", "a", "b", "y"}, result)
	})

	t.Run("empty right", func(t *testing.T) {
		result := SpreadStrings([]string{"x", "...", "y"}, []string{})
		require.Equal(t, []string{"x", "y"}, result)
	})
}

func TestSpread(t *testing.T) {
	t.Run("nil left returns right", func(t *testing.T) {
		right := []Layer{NewImageLayer("ubuntu:22.04")}
		result := Spread[Layer](nil, right)
		require.Equal(t, right, result)
	})

	t.Run("no spread marker", func(t *testing.T) {
		left := []Layer{NewImageLayer("alpine")}
		right := []Layer{NewImageLayer("ubuntu")}
		result := Spread(left, right)
		require.Len(t, result, 1)
		require.Equal(t, "alpine", result[0].Image)
	})
}

func TestNewBuildPlan(t *testing.T) {
	p := NewBuildPlan()
	require.NotNil(t, p)
	require.Empty(t, p.Steps)
	require.Empty(t, p.Caches)
	require.Empty(t, p.Secrets)
}

func TestBuildPlanAddStep(t *testing.T) {
	p := NewBuildPlan()
	step := *NewStep("build")
	step.Commands = []Command{NewExecShellCommand("make")}
	p.AddStep(step)
	require.Len(t, p.Steps, 1)
	require.Equal(t, "build", p.Steps[0].Name)
}

func TestNewStep(t *testing.T) {
	step := NewStep("install")
	require.Equal(t, "install", step.Name)
	require.NotNil(t, step.Assets)
	require.NotNil(t, step.Variables)
	require.Equal(t, []string{"*"}, step.Secrets)
}

func TestStepAddCommands(t *testing.T) {
	step := NewStep("build")
	step.AddCommands([]Command{
		NewExecShellCommand("npm install"),
		NewExecShellCommand("npm run build"),
	})
	require.Len(t, step.Commands, 2)
}

func TestNewCache(t *testing.T) {
	cache := NewCache("/root/.npm")
	require.Equal(t, "/root/.npm", cache.Directory)
	require.Equal(t, CacheTypeShared, cache.Type)
}

func TestNewFilter(t *testing.T) {
	f := NewFilter([]string{"dist"}, []string{"*.log"})
	require.Equal(t, []string{"dist"}, f.Include)
	require.Equal(t, []string{"*.log"}, f.Exclude)
}
