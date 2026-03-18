package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
