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

type ExecOptions struct {
	CustomName string
}

type ExecCommand struct {
	Cmd        string `json:"cmd"`
	CustomName string `json:"customName,omitempty"`
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

func NewExecCommand(cmd string, options ...ExecOptions) Command {
	exec := ExecCommand{Cmd: cmd}
	if len(options) > 0 {
		exec.CustomName = options[0].CustomName
	}
	return exec
}

func ShellCommandString(cmd string) string {
	return "sh -c '" + cmd + "'"
}

func NewExecShellCommand(cmd string, options ...ExecOptions) Command {
	if len(options) == 0 {
		options = []ExecOptions{
			{CustomName: cmd},
		}
	}

	exec := NewExecCommand(ShellCommandString(cmd), options...)
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
	return e.Cmd == ShellCommandString("...") || e.Cmd == "..."
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
