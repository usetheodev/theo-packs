package generate

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/plan"
)

func TestDeployBuilderSetInputs(t *testing.T) {
	builder := NewDeployBuilder()
	layers := []plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	}
	builder.SetInputs(layers)
	require.Len(t, builder.DeployInputs, 1)

	builder.SetInputs([]plan.Layer{})
	require.Empty(t, builder.DeployInputs)
}

func TestDeployBuilderAddAptPackages(t *testing.T) {
	builder := NewDeployBuilder()
	builder.AddAptPackages([]string{"curl", "wget"})
	require.Equal(t, []string{"curl", "wget"}, builder.AptPackages)

	builder.AddAptPackages([]string{"git"})
	require.Equal(t, []string{"curl", "wget", "git"}, builder.AptPackages)
}

func TestDeployBuilderBuildWithAptPackages(t *testing.T) {
	builder := NewDeployBuilder()
	builder.StartCmd = "node server.js"
	builder.AddAptPackages([]string{"libssl-dev"})
	builder.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})

	buildPlan := plan.NewBuildPlan()
	builder.Build(buildPlan, &BuildStepOptions{
		Caches: NewCacheContext(),
	})

	require.Equal(t, "node server.js", buildPlan.Deploy.StartCmd)
	// apt packages create an extra step
	require.Len(t, buildPlan.Steps, 1)
	require.Equal(t, "packages:apt:runtime", buildPlan.Steps[0].Name)
}

func TestHasIncludeForStep(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []plan.Layer
		stepName string
		path     string
		expected bool
	}{
		{
			name:     "empty inputs",
			inputs:   []plan.Layer{},
			stepName: "build",
			path:     ".",
			expected: false,
		},
		{
			name: "exact match",
			inputs: []plan.Layer{
				{Step: "build", Filter: plan.Filter{Include: []string{"."}}},
			},
			stepName: "build",
			path:     ".",
			expected: true,
		},
		{
			name: "dot covers any path",
			inputs: []plan.Layer{
				{Step: "build", Filter: plan.Filter{Include: []string{"."}}},
			},
			stepName: "build",
			path:     "/app/dist",
			expected: true,
		},
		{
			name: "any path covers dot",
			inputs: []plan.Layer{
				{Step: "build", Filter: plan.Filter{Include: []string{"/root/.cache", "."}}},
			},
			stepName: "build",
			path:     ".",
			expected: true,
		},
		{
			name: "different step name",
			inputs: []plan.Layer{
				{Step: "install", Filter: plan.Filter{Include: []string{"."}}},
			},
			stepName: "build",
			path:     ".",
			expected: false,
		},
		{
			name: "specific path match",
			inputs: []plan.Layer{
				{Step: "build", Filter: plan.Filter{Include: []string{"/app/node_modules"}}},
			},
			stepName: "build",
			path:     "/app/node_modules",
			expected: true,
		},
		{
			name: "specific path no match",
			inputs: []plan.Layer{
				{Step: "build", Filter: plan.Filter{Include: []string{"/app/node_modules"}}},
			},
			stepName: "build",
			path:     "/app/dist",
			expected: false,
		},
		{
			name: "specific path does not cover dot",
			inputs: []plan.Layer{
				{Step: "build", Filter: plan.Filter{Include: []string{"/app/node_modules"}}},
			},
			stepName: "build",
			path:     ".",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewDeployBuilder()
			builder.DeployInputs = tt.inputs
			result := builder.HasIncludeForStep(tt.stepName, tt.path)
			require.Equal(t, tt.expected, result)
		})
	}
}
