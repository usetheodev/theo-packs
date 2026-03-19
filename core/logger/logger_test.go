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
