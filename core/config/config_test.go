package config

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestEmptyConfig(t *testing.T) {
	config := EmptyConfig()
	require.NotNil(t, config)
	require.Empty(t, config.Caches)
	require.Empty(t, config.Packages)
	require.Empty(t, config.BuildAptPackages)
	require.Empty(t, config.Steps)
	require.Nil(t, config.Provider)

	require.NotNil(t, config.Deploy)
	require.Nil(t, config.Deploy.Inputs)
	require.Nil(t, config.Deploy.Paths)
}

func TestMergeConfigSmall(t *testing.T) {
	config1JSON := `{
		"baseImage": "ubuntu:20.04",
		"packages": {
			"python": "latest",
			"node": "22"
		},
		"aptPackages": ["git"],
		"steps": {
			"install": {
				"dependsOn": ["packages"],
				"commands": [
					"echo first"
				],
				"variables": {
					"HELLO": "world"
				},
				"caches": ["pip"]
			}
		},
		"deploy": {
			"variables": {
				"SHARED": "one",
				"HELLO": "world"
			}
		}
	}`

	config2JSON := `{
		"baseImage": "secondd",
		"packages": {
			"node": "23",
			"bun": "latest"
		},
		"steps": {
			"install": {
				"variables": {
					"another": "boop"
				}
			}
		},
		"deploy": {
			"variables": {
				"SHARED": "two",
				"FOO": "bar"
			}
		}
	}`

	expectedJSON := `{
		"baseImage": "secondd",
		"packages": {
			"python": "latest",
			"node": "23",
			"bun": "latest"
		},
		"aptPackages": ["git"],
		"steps": {
			"install": {
				"dependsOn": ["packages"],
				"commands": [
					"echo first"
				],
				"variables": {
					"HELLO": "world",
					"another": "boop"
				},
				"caches": ["pip"]
			}
		},
		"deploy": {
			"variables": {
				"HELLO": "world",
				"SHARED": "two",
				"FOO": "bar"
			}
		},
		"caches": {}
	}`

	var config1, config2, expected Config
	require.NoError(t, json.Unmarshal([]byte(config1JSON), &config1))
	require.NoError(t, json.Unmarshal([]byte(config2JSON), &config2))
	require.NoError(t, json.Unmarshal([]byte(expectedJSON), &expected))

	result := Merge(&config1, &config2)

	if diff := cmp.Diff(expected, *result); diff != "" {
		t.Errorf("configs mismatch (-want +got):\n%s", diff)
	}
}

func TestMergeConfigStart(t *testing.T) {
	config1JSON := `{
		"deploy": {
			"startCommand": "python app.py"
		}
	}`

	config2JSON := `{
		"packages": {
			"node": "23"
		}
	}`

	expectedJSON := `{
		"packages": {
			"node": "23"
		},
		"deploy": {
			"startCommand": "python app.py"
		},
		"steps": {},
		"caches": {}
	}`

	var config1, config2, expected Config
	require.NoError(t, json.Unmarshal([]byte(config1JSON), &config1))
	require.NoError(t, json.Unmarshal([]byte(config2JSON), &config2))
	require.NoError(t, json.Unmarshal([]byte(expectedJSON), &expected))

	result := Merge(&config1, &config2)

	if diff := cmp.Diff(expected, *result); diff != "" {
		t.Errorf("configs mismatch (-want +got):\n%s", diff)
	}
}
