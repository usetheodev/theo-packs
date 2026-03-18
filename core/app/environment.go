package app

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Environment struct {
	Variables map[string]string
}

func NewEnvironment(variables *map[string]string) *Environment {
	if variables == nil {
		variables = &map[string]string{}
	}

	return &Environment{Variables: *variables}
}

// FromEnvs collects variables from the given environment variable names
func FromEnvs(envs []string) (*Environment, error) {
	env := NewEnvironment(nil)
	re := regexp.MustCompile(`([A-Za-z0-9_+\-]*)(?:=?)(.*)`)

	for _, e := range envs {
		matches := re.FindStringSubmatch(e)
		if len(matches) < 3 {
			continue
		}

		name := matches[1]
		value := matches[2]

		if value == "" {
			if v, ok := os.LookupEnv(name); ok {
				env.SetVariable(name, v)
			}
		} else {
			env.SetVariable(name, value)
		}
	}

	return env, nil
}

func (e *Environment) GetVariable(name string) string {
	return e.Variables[name]
}

func (e *Environment) SetVariable(name, value string) {
	e.Variables[name] = value
}

func (e *Environment) ConfigVariable(name string) string {
	return fmt.Sprintf("THEOPACKS_%s", name)
}

func (e *Environment) GetConfigVariable(name string) (string, string) {
	configVar := e.ConfigVariable(name)

	if val, exists := e.Variables[configVar]; exists {
		return strings.TrimSpace(val), configVar
	}
	return "", ""
}

func (e *Environment) GetConfigVariableList(name string) ([]string, string) {
	val, configVar := e.GetConfigVariable(name)
	if val == "" {
		return nil, ""
	}
	return strings.Split(val, " "), configVar
}

func (e *Environment) IsConfigVariableTruthy(name string) bool {
	if val, _ := e.GetConfigVariable(name); val != "" {
		lowerVal := strings.ToLower(val)
		return lowerVal == "1" || lowerVal == "true"
	}
	return false
}

func (e *Environment) GetSecretsWithPrefix(prefix string) []string {
	secrets := []string{}
	for secretName := range e.Variables {
		if strings.HasPrefix(secretName, prefix) {
			secrets = append(secrets, secretName)
		}
	}
	return secrets
}
