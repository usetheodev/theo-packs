package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Command interface {
	CommandType() string
	Spreadable
}

// CommandKind distinguishes the rendering form an ExecCommand should use:
// raw exec (no shell wrap) vs sh -c '...' (shell features).
//
// The renderer (core/dockerfile/generate.go) reads this to emit either
//
//	RUN <cmd>
//
// or
//
//	RUN sh -c '<cmd>'
//
// without ambiguity. Other Command kinds (Copy/File/Path) have their own
// dedicated render paths and don't carry a CommandKind.
type CommandKind int

const (
	// CommandKindShell wraps the command body in `sh -c '...'`. This is the
	// historical default produced by NewExecShellCommand and preserves
	// backward compatibility with any provider call site that depends on
	// shell features (pipes, $(), env-var expansion, ;).
	CommandKindShell CommandKind = iota
	// CommandKindExec emits the command body raw, without any shell wrapping.
	// Use NewExecCommand for plain `<binary> <args>` forms that don't need
	// shell features. Saves an `sh` process per RUN.
	CommandKindExec
)

type ExecOptions struct {
	CustomName string
}

type ExecCommand struct {
	Cmd        string      `json:"cmd"`
	CustomName string      `json:"customName,omitempty"`
	Kind       CommandKind `json:"kind,omitempty"`
}

type PathCommand struct {
	Path string `json:"path"`
}

type CopyCommand struct {
	Image string `json:"image,omitempty"`
	Src   string `json:"src"`
	Dest  string `json:"dest"`
}

type FileOptions struct {
	Mode       os.FileMode
	CustomName string
}

type FileCommand struct {
	Path       string      `json:"path"`
	Name       string      `json:"name"`
	Mode       os.FileMode `json:"mode,omitempty"`
	CustomName string      `json:"customName,omitempty"`
}

func (e ExecCommand) CommandType() string { return "exec" }
func (g PathCommand) CommandType() string { return "globalPath" }
func (c CopyCommand) CommandType() string { return "copy" }
func (f FileCommand) CommandType() string { return "file" }

// NewExecCommand emits a raw `RUN <cmd>` directive without any shell wrapping.
// Use this when the command is a single binary invocation (e.g., `go mod
// download`, `cargo fetch`, `npm ci`) that doesn't need pipes / `$()` /
// env-var expansion. Saves the `sh` process the legacy NewExecShellCommand
// always added.
func NewExecCommand(cmd string, options ...ExecOptions) Command {
	exec := ExecCommand{Cmd: cmd, Kind: CommandKindExec}
	if len(options) > 0 {
		exec.CustomName = options[0].CustomName
	}
	return exec
}

// ShellCommandString is kept for tests and legacy code paths that want the
// literal `sh -c '...'` string form. NewExecShellCommand below uses
// CommandKindShell so the renderer can produce the same output without
// double-wrapping.
func ShellCommandString(cmd string) string {
	return "sh -c '" + cmd + "'"
}

// NewExecShellCommand emits `RUN sh -c '<cmd>'`. Use this when the command
// body uses shell features (pipes, redirects, `$()`, env-var expansion,
// `;`/`&&`/`||`). The renderer wraps the body in `sh -c '...'` exactly once
// — providers must NOT pre-wrap their command strings.
func NewExecShellCommand(cmd string, options ...ExecOptions) Command {
	exec := ExecCommand{Cmd: cmd, Kind: CommandKindShell}
	if len(options) > 0 {
		exec.CustomName = options[0].CustomName
	} else {
		exec.CustomName = cmd
	}
	return exec
}

func NewPathCommand(path string, customName ...string) Command {
	pathCmd := PathCommand{Path: path}
	return pathCmd
}

func NewCopyCommand(src string, dst ...string) Command {
	dstPath := src
	if len(dst) > 0 {
		dstPath = dst[0]
	}

	copyCmd := CopyCommand{Src: src, Dest: dstPath}
	return copyCmd
}

func NewFileCommand(path, name string, options ...FileOptions) Command {
	fileCmd := FileCommand{Path: path, Name: name}
	if len(options) > 0 {
		fileCmd.CustomName = options[0].CustomName
		fileCmd.Mode = options[0].Mode
	}
	return fileCmd
}

func UnmarshalCommand(data []byte) (Command, error) {
	if cmd, err := UnmarshalJsonCommand(data); err == nil {
		return cmd, nil
	}

	return UnmarshalStringCommand(data)
}

func UnmarshalJsonCommand(data []byte) (Command, error) {
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, err
	}

	if _, ok := rawMap["cmd"]; ok {
		var cmd ExecCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			return nil, err
		}
		return cmd, nil
	}

	if _, ok := rawMap["path"]; ok {
		if _, ok := rawMap["name"]; ok {
			var file FileCommand
			if err := json.Unmarshal(data, &file); err != nil {
				return nil, err
			}
			return file, nil
		}
		var path PathCommand
		if err := json.Unmarshal(data, &path); err != nil {
			return nil, err
		}
		return path, nil
	}

	if _, ok := rawMap["src"]; ok {
		var copy CopyCommand
		if err := json.Unmarshal(data, &copy); err != nil {
			return nil, err
		}
		return copy, nil
	}

	return nil, fmt.Errorf("unknown command type: %v", rawMap)
}

func UnmarshalStringCommand(data []byte) (Command, error) {
	str := string(data)

	if !strings.Contains(str, ":") {
		cmdToRun := strings.Trim(str, "\"")
		return NewExecShellCommand(cmdToRun, ExecOptions{CustomName: cmdToRun}), nil
	}

	parts := strings.SplitN(str, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid command format: %s", str)
	}

	prefix := parts[0]
	payload := parts[1]

	prefixParts := strings.SplitN(prefix, "#", 2)
	cmdType := prefixParts[0]
	customName := ""
	if len(prefixParts) > 1 {
		customName = prefixParts[1]
	}

	switch cmdType {
	case "RUN":
		return NewExecShellCommand(payload, ExecOptions{CustomName: customName}), nil
	case "PATH":
		return NewPathCommand(payload), nil
	case "COPY":
		copyParts := strings.Fields(payload)
		if len(copyParts) != 2 {
			return nil, fmt.Errorf("invalid COPY format: %s", payload)
		}
		return NewCopyCommand(copyParts[0], copyParts[1]), nil
	case "FILE":
		fileParts := strings.Fields(payload)
		if len(fileParts) != 2 {
			return nil, fmt.Errorf("invalid FILE format: %s", payload)
		}
		return NewFileCommand(fileParts[0], fileParts[1], FileOptions{CustomName: customName}), nil
	}

	cmdToRun := strings.Trim(str, "\"")
	if customName == "" {
		customName = cmdToRun
	}
	return NewExecShellCommand(cmdToRun, ExecOptions{CustomName: customName}), nil
}

func (e ExecCommand) IsSpread() bool {
	// Spread commands are recognized whether they use the legacy sh -c
	// wrapping or the new raw exec form.
	return e.Cmd == "..." || e.Cmd == ShellCommandString("...")
}

func (p PathCommand) IsSpread() bool {
	return false
}

func (c CopyCommand) IsSpread() bool {
	return false
}

func (f FileCommand) IsSpread() bool {
	return false
}
