package logger

import "fmt"

type Level string

const (
	Info  Level = "info"
	Warn  Level = "warn"
	Error Level = "error"
)

// MaxLogs caps the number of entries retained in memory. Pathological
// build plans could otherwise inflate BuildResult.Logs without bound;
// when the cap is hit further messages are dropped and a single
// "truncated" warning replaces the most recent ones. (T3.5 L5.)
const MaxLogs = 1000

type Msg struct {
	Level Level
	Msg   string
}

type Logger struct {
	Logs      []Msg
	truncated bool
}

func NewLogger() *Logger {
	return &Logger{
		Logs: []Msg{},
	}
}

// Nop returns a logger that discards messages. Useful in tests and
// internal helpers where the API requires a non-nil *Logger but the
// caller doesn't care about the output. Replaces the historical
// `log ...*Logger` variadic hack in workspace / go.work parsers.
func Nop() *Logger { return nopLogger }

func (l *Logger) LogInfo(format string, args ...interface{}) { l.append(Info, format, args...) }
func (l *Logger) LogWarn(format string, args ...interface{}) { l.append(Warn, format, args...) }
func (l *Logger) LogError(format string, args ...interface{}) {
	l.append(Error, format, args...)
}

func (l *Logger) append(lvl Level, format string, args ...interface{}) {
	if l == nil || l == nopLogger {
		return
	}
	if len(l.Logs) >= MaxLogs {
		if !l.truncated {
			l.truncated = true
			l.Logs = append(l.Logs, Msg{
				Level: Warn,
				Msg:   fmt.Sprintf("log buffer truncated at %d entries; further messages dropped", MaxLogs),
			})
		}
		return
	}
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	l.Logs = append(l.Logs, Msg{Level: lvl, Msg: msg})
}

// nopLogger is the sentinel returned by Nop(); the append fast-path
// short-circuits before any allocation.
var nopLogger = &Logger{}
