package dockerfile

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestShellEscape contract — single-quote wrapping with the `'\''`
// close/escape/reopen idiom must round-trip through /bin/sh. We assert
// each output literally AND, for the structural cases, by handing the
// escaped value to /bin/sh -c "printf '%s' <escaped>" and comparing
// back to the original. The latter catches regressions where the
// algorithm looks correct in tests but produces a string sh refuses.

func TestShellEscape_SimpleString(t *testing.T) {
	require.Equal(t, "'hello'", shellEscape("hello"))
}

func TestShellEscape_EmptyString(t *testing.T) {
	require.Equal(t, "''", shellEscape(""))
}

func TestShellEscape_WithSingleQuote(t *testing.T) {
	// `a'b` → 'a'\''b'
	require.Equal(t, `'a'\''b'`, shellEscape("a'b"))
}

func TestShellEscape_OnlySingleQuote(t *testing.T) {
	// Single character `'` → ''\''' (close, escape, reopen)
	require.Equal(t, `''\'''`, shellEscape("'"))
}

func TestShellEscape_MultiLine(t *testing.T) {
	require.Equal(t, "'line1\nline2'", shellEscape("line1\nline2"))
}

func TestShellEscape_ShellMetacharacters(t *testing.T) {
	// $, `, |, &, ;, *, ?, <, >, etc. — must all survive un-interpreted.
	in := "$VAR `whoami` | rm -rf /; echo *"
	require.Equal(t, "'"+in+"'", shellEscape(in))
}

// TestShellEscape_RoundTripsThroughSh asserts that the escape produces
// shell-valid output. We pipe the escaped value through `sh -c "printf
// '%s' <escaped>"` and confirm the output matches the original.
func TestShellEscape_RoundTripsThroughSh(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not on PATH")
	}
	inputs := []string{
		"hello world",
		"a'b'c",
		"path with spaces",
		"line1\nline2",
		"$HOME and `pwd`",
		"",
		"'",
		"''",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			escaped := shellEscape(in)
			cmd := exec.Command("sh", "-c", "printf '%s' "+escaped)
			out, err := cmd.Output()
			require.NoError(t, err, "sh rejected escaped value %q", escaped)
			require.Equal(t, in, string(out),
				"round-trip mismatch: in=%q escaped=%q out=%q", in, escaped, string(out))
		})
	}
}
