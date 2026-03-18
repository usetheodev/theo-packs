package plan

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name            string
		command         Command
		expectedJSON    string
		unmarshalString string
	}{
		{
			name:            "exec command without custom name",
			command:         NewExecShellCommand("echo hello", ExecOptions{CustomName: "echo hello"}),
			expectedJSON:    `{"cmd":"sh -c 'echo hello'","customName":"echo hello"}`,
			unmarshalString: "echo hello",
		},
		{
			name:            "exec command with custom name",
			command:         NewExecShellCommand("echo hello", ExecOptions{CustomName: "Say Hello"}),
			expectedJSON:    `{"cmd":"sh -c 'echo hello'","customName":"Say Hello"}`,
			unmarshalString: "RUN#Say Hello:echo hello",
		},
		{
			name:            "path command",
			command:         NewPathCommand("/usr/local/bin"),
			expectedJSON:    `{"path":"/usr/local/bin"}`,
			unmarshalString: "PATH:/usr/local/bin",
		},
		{
			name:            "copy command",
			command:         NewCopyCommand("src.txt", "dst.txt"),
			expectedJSON:    `{"src":"src.txt","dest":"dst.txt"}`,
			unmarshalString: "COPY:src.txt dst.txt",
		},
		{
			name:            "file command without custom name",
			command:         NewFileCommand("/etc/conf", "config.yaml"),
			expectedJSON:    `{"path":"/etc/conf","name":"config.yaml"}`,
			unmarshalString: "FILE:/etc/conf config.yaml",
		},
		{
			name:            "file command with custom name",
			command:         NewFileCommand("/etc/conf", "config.yaml", FileOptions{CustomName: "Config File"}),
			expectedJSON:    `{"path":"/etc/conf","name":"config.yaml","customName":"Config File"}`,
			unmarshalString: "FILE#Config File:/etc/conf config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.command)
			require.NoError(t, err, "failed to marshal command")
			require.Equal(t, string(data), tt.expectedJSON, "marshal result")

			cmd, err := UnmarshalCommand([]byte(tt.expectedJSON))
			require.NoError(t, err, "failed to unmarshal JSON command")

			roundTrip, err := json.Marshal(cmd)
			require.NoError(t, err, "failed to marshal unmarshalled command")
			require.Equal(t, string(roundTrip), tt.expectedJSON, "round-trip JSON result")

			if tt.unmarshalString != "" {
				cmd, err = UnmarshalCommand([]byte(tt.unmarshalString))
				require.NoError(t, err, "failed to unmarshal string command")

				roundTrip, err = json.Marshal(cmd)
				require.NoError(t, err, "failed to marshal string-unmarshalled command")
				require.Equal(t, string(roundTrip), tt.expectedJSON, "string unmarshal to JSON result")
			}
		})
	}
}
