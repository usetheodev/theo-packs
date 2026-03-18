package generate

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/plan"
)

func TestHasIncludeForStep(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		builder := NewDeployBuilder()
		builder.AddInputs([]plan.Layer{
			plan.NewStepLayer("build", plan.Filter{Include: []string{"dist"}}),
		})

		require.True(t, builder.HasIncludeForStep("build", "dist"))
		require.False(t, builder.HasIncludeForStep("build", "src"))
		require.False(t, builder.HasIncludeForStep("install", "dist"))
	})

	t.Run("dot covers everything", func(t *testing.T) {
		builder := NewDeployBuilder()
		builder.AddInputs([]plan.Layer{
			plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
		})

		require.True(t, builder.HasIncludeForStep("build", "dist"))
		require.True(t, builder.HasIncludeForStep("build", "."))
		require.True(t, builder.HasIncludeForStep("build", "anything"))
		require.False(t, builder.HasIncludeForStep("install", "dist"))
	})

	t.Run("no match", func(t *testing.T) {
		builder := NewDeployBuilder()
		require.False(t, builder.HasIncludeForStep("build", "dist"))
	})
}
