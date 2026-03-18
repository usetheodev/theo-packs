package plan

import (
	"encoding/json"
)

type Step struct {
	Name      string            `json:"name,omitempty"`
	Inputs    []Layer           `json:"inputs,omitempty"`
	Commands  []Command         `json:"commands,omitempty"`
	Secrets   []string          `json:"secrets,omitempty"`
	Assets    map[string]string `json:"assets,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Caches    []string          `json:"caches,omitempty"`
}

func NewStep(name string) *Step {
	return &Step{
		Name:      name,
		Assets:    make(map[string]string),
		Variables: make(map[string]string),
		Secrets:   []string{"*"},
	}
}

func (s *Step) AddCommands(commands []Command) {
	if s.Commands == nil {
		s.Commands = []Command{}
	}
	s.Commands = append(s.Commands, commands...)
}

func (s *Step) UnmarshalJSON(data []byte) error {
	type Alias Step
	aux := &struct {
		Commands *[]json.RawMessage `json:"commands"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.Commands != nil {
		s.Commands = []Command{}
		for _, rawCmd := range *aux.Commands {
			cmd, err := UnmarshalCommand(rawCmd)
			if err != nil {
				return err
			}
			s.Commands = append(s.Commands, cmd)
		}
	}

	return nil
}
