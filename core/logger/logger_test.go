package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	log := NewLogger()
	require.NotNil(t, log)
	require.Empty(t, log.Logs)
}

func TestLogInfo(t *testing.T) {
	log := NewLogger()
	log.LogInfo("hello %s", "world")

	require.Len(t, log.Logs, 1)
	require.Equal(t, Info, log.Logs[0].Level)
	require.Equal(t, "hello world", log.Logs[0].Msg)
}

func TestLogWarn(t *testing.T) {
	log := NewLogger()
	log.LogWarn("warning: %d issues", 3)

	require.Len(t, log.Logs, 1)
	require.Equal(t, Warn, log.Logs[0].Level)
	require.Equal(t, "warning: 3 issues", log.Logs[0].Msg)
}

func TestLogError(t *testing.T) {
	log := NewLogger()
	log.LogError("error occurred")

	require.Len(t, log.Logs, 1)
	require.Equal(t, Error, log.Logs[0].Level)
	require.Equal(t, "error occurred", log.Logs[0].Msg)
}

func TestLogMultipleMessages(t *testing.T) {
	log := NewLogger()
	log.LogInfo("step 1")
	log.LogWarn("step 2")
	log.LogError("step 3")

	require.Len(t, log.Logs, 3)
	require.Equal(t, Info, log.Logs[0].Level)
	require.Equal(t, Warn, log.Logs[1].Level)
	require.Equal(t, Error, log.Logs[2].Level)
}

func TestLogWithoutFormatArgs(t *testing.T) {
	log := NewLogger()
	log.LogInfo("simple message")

	require.Len(t, log.Logs, 1)
	require.Equal(t, "simple message", log.Logs[0].Msg)
}

// TestLogger_CapsAtMaxLogs — pathological input must not blow up
// BuildResult.Logs without bound (T3.5 L5).
func TestLogger_CapsAtMaxLogs(t *testing.T) {
	log := NewLogger()
	for i := 0; i < MaxLogs+500; i++ {
		log.LogInfo("msg %d", i)
	}
	// Exactly MaxLogs real messages + 1 truncation warning.
	require.Equal(t, MaxLogs+1, len(log.Logs))
	last := log.Logs[len(log.Logs)-1]
	require.Equal(t, Warn, last.Level)
	require.Contains(t, last.Msg, "truncated")
}

// TestLogger_NopDiscardsMessages — Nop() short-circuits append entirely
// (T3.3). Calling on Nop() must not allocate or grow any slice.
func TestLogger_NopDiscardsMessages(t *testing.T) {
	log := Nop()
	log.LogInfo("ignored")
	log.LogWarn("ignored")
	log.LogError("ignored")
	require.Empty(t, log.Logs)
}

// TestLogger_NilSafe — defensive: passing a nil receiver must not
// panic. Useful because some packages still construct Logger via the
// zero value.
func TestLogger_NilSafe(t *testing.T) {
	var log *Logger
	require.NotPanics(t, func() {
		log.LogInfo("ignored")
	})
}
