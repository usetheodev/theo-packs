package dockerfile

import (
	"os/exec"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// TestShellEscape contract — single-quote wrapping with the `'\''`
// close/escape/reopen idiom must round-trip through /bin/sh. We assert
// each output literally AND, for the structural cases, by handing the
// escaped value to /bin/sh -c "printf '%s' <escaped>" and comparing
// back to the original. The latter catches regressions where the
// algorithm looks correct in tests but produces a string sh refuses.

func TestShellEscape_SimpleString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "'hello'", shellEscape("hello"))
}

func TestShellEscape_EmptyString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "''", shellEscape(""))
}

func TestShellEscape_WithSingleQuote(t *testing.T) {
	t.Parallel()
	// `a'b` → 'a'\''b'
	require.Equal(t, `'a'\''b'`, shellEscape("a'b"))
}

func TestShellEscape_OnlySingleQuote(t *testing.T) {
	t.Parallel()
	// Single character `'` → ''\''' (close, escape, reopen)
	require.Equal(t, `''\'''`, shellEscape("'"))
}

func TestShellEscape_MultiLine(t *testing.T) {
	t.Parallel()
	require.Equal(t, "'line1\nline2'", shellEscape("line1\nline2"))
}

func TestShellEscape_ShellMetacharacters(t *testing.T) {
	t.Parallel()
	// $, `, |, &, ;, *, ?, <, >, etc. — must all survive un-interpreted.
	in := "$VAR `whoami` | rm -rf /; echo *"
	require.Equal(t, "'"+in+"'", shellEscape(in))
}

// TestShellEscape_RoundTripsThroughSh asserts that the escape produces
// shell-valid output. We pipe the escaped value through `sh -c "printf
// '%s' <escaped>"` and confirm the output matches the original.
func TestShellEscape_RoundTripsThroughSh(t *testing.T) {
	t.Parallel()
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

// FuzzShellEscape_RoundTripsThroughSh — T3.1 adversarial defense.
//
// For arbitrary input that sh can carry (no NUL bytes, valid UTF-8 per
// the locale), shellEscape(s) interpolated into `sh -c "printf '%s' <esc>"`
// must return exactly s. Catches edge cases that the seed table missed.
//
// Corpus persisted under testdata/fuzz/FuzzShellEscape_RoundTripsThroughSh/
// so future runs replay any crash inputs found locally.
func FuzzShellEscape_RoundTripsThroughSh(f *testing.F) {
	if _, err := exec.LookPath("sh"); err != nil {
		f.Skip("sh not on PATH")
	}
	for _, seed := range []string{
		"", "'", "''", "a'b", "a'b'c",
		"$VAR", "`whoami`", "\n", "\t",
		"hello world",
		"\\", "\\'", "\\\\",
		" ", "  ",
		"line1\nline2",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in string) {
		// Skip inputs sh can't carry; the function isn't expected to
		// handle these regardless.
		if !utf8.ValidString(in) {
			t.Skip("invalid utf-8")
		}
		if strings.ContainsRune(in, 0) {
			t.Skip("contains NUL byte")
		}
		escaped := shellEscape(in)
		out, err := exec.Command("sh", "-c", "printf '%s' "+escaped).Output()
		require.NoError(t, err, "sh rejected escaped value %q (input %q)", escaped, in)
		require.Equal(t, in, string(out),
			"round-trip mismatch:\n  in:      %q\n  escaped: %q\n  out:     %q",
			in, escaped, string(out))
	})
}
