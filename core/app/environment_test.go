package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetConfigVariableList(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected []string
	}{
		{name: "single value", value: "node", expected: []string{"node"}},
		{name: "multiple values", value: "node python go", expected: []string{"node", "python", "go"}},
		{name: "extra spaces", value: "node  python   go", expected: []string{"node", "python", "go"}},
		{name: "leading/trailing spaces", value: "  node python  ", expected: []string{"node", "python"}},
		{name: "empty value", value: "", expected: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVars := map[string]string{}
			if tt.value != "" {
				envVars["THEOPACKS_PACKAGES"] = tt.value
			}
			env := NewEnvironment(&envVars)

			result, _ := env.GetConfigVariableList("PACKAGES")
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetConfigVariable(t *testing.T) {
	envVars := map[string]string{
		"THEOPACKS_START_CMD": "  node server.js  ",
	}
	env := NewEnvironment(&envVars)

	val, varName := env.GetConfigVariable("START_CMD")
	require.Equal(t, "node server.js", val, "should trim whitespace")
	require.Equal(t, "THEOPACKS_START_CMD", varName)

	val, varName = env.GetConfigVariable("NONEXISTENT")
	require.Empty(t, val)
	require.Empty(t, varName)
}

func TestGetSecretsWithPrefix(t *testing.T) {
	envVars := map[string]string{
		"NPM_TOKEN":      "secret1",
		"NPM_AUTH_TOKEN": "secret2",
		"GITHUB_TOKEN":   "secret3",
		"OTHER_VAR":      "value",
	}
	env := NewEnvironment(&envVars)

	npmSecrets := env.GetSecretsWithPrefix("NPM_")
	require.Len(t, npmSecrets, 2)
	require.Contains(t, npmSecrets, "NPM_TOKEN")
	require.Contains(t, npmSecrets, "NPM_AUTH_TOKEN")

	noSecrets := env.GetSecretsWithPrefix("MISSING_")
	require.Empty(t, noSecrets)
}

func TestIsConfigVariableTruthy(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{name: "true", value: "true", expected: true},
		{name: "True", value: "True", expected: true},
		{name: "TRUE", value: "TRUE", expected: true},
		{name: "1", value: "1", expected: true},
		{name: "false", value: "false", expected: false},
		{name: "0", value: "0", expected: false},
		{name: "empty", value: "", expected: false},
		{name: "random", value: "yes", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVars := map[string]string{}
			if tt.value != "" {
				envVars["THEOPACKS_FLAG"] = tt.value
			}
			env := NewEnvironment(&envVars)
			require.Equal(t, tt.expected, env.IsConfigVariableTruthy("FLAG"))
		})
	}
}

func TestFromEnvs(t *testing.T) {
	env, err := FromEnvs([]string{
		"VAR1=value1",
		"VAR2=value2",
		"THEOPACKS_APT_PACKAGES=apt1,apt2",
		"COMMA=this has, a comma",
		"THEOPACKS_TRUTHY_CASE=True ",
		"THEOPACKS_TRUTHY_INT_CASE= 1 ",
		"HELLO+WORLD=boop",
	})

	require.NoError(t, err)
	require.Equal(t, env.GetVariable("VAR1"), "value1")
	require.Equal(t, env.GetVariable("VAR2"), "value2")
	require.Equal(t, env.GetVariable("THEOPACKS_APT_PACKAGES"), "apt1,apt2")
	require.Equal(t, env.GetVariable("COMMA"), "this has, a comma")
	require.Equal(t, env.IsConfigVariableTruthy("TRUTHY_CASE"), true)
	require.Equal(t, env.IsConfigVariableTruthy("TRUTHY_INT_CASE"), true)
	require.Equal(t, env.GetVariable("HELLO+WORLD"), "boop")
}
