package config

import (
	"encoding/json"

	"github.com/usetheo/theopacks/core/plan"
	"github.com/usetheo/theopacks/internal/utils"
)

type DeployConfig struct {
	AptPackages []string          `json:"aptPackages,omitempty"`
	Base        *plan.Layer       `json:"base,omitempty"`
	Inputs      []plan.Layer      `json:"inputs,omitempty"`
	StartCmd    string            `json:"startCommand,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	Paths       []string          `json:"paths,omitempty"`
}

type StepConfig struct {
	plan.Step
	DeployOutputs []plan.Filter `json:"deployOutputs,omitempty"`
}

type Config struct {
	Provider         *string                `json:"provider,omitempty"`
	BuildAptPackages []string               `json:"buildAptPackages,omitempty"`
	Steps            map[string]*StepConfig `json:"steps,omitempty"`
	Deploy           *DeployConfig          `json:"deploy,omitempty"`
	Packages         map[string]string      `json:"packages,omitempty"`
	Caches           map[string]*plan.Cache `json:"caches,omitempty"`
	Secrets          []string               `json:"secrets,omitempty"`
}

func EmptyConfig() *Config {
	return &Config{
		Steps:    make(map[string]*StepConfig),
		Packages: make(map[string]string),
		Caches:   make(map[string]*plan.Cache),
		Deploy:   &DeployConfig{},
	}
}

func (c *Config) GetOrCreateStep(name string) *StepConfig {
	if existingStep, exists := c.Steps[name]; exists {
		return existingStep
	}

	step := &StepConfig{
		Step: *plan.NewStep(name),
	}
	c.Steps[name] = step

	return step
}

// Merge combines multiple configs by merging their values with later configs taking precedence
func Merge(configs ...*Config) *Config {
	if len(configs) == 0 {
		return EmptyConfig()
	}

	result := EmptyConfig()
	for _, config := range configs {
		if config == nil {
			continue
		}

		utils.MergeStructs(result, config)
	}

	return result
}

func (s *StepConfig) UnmarshalJSON(data []byte) error {
	var temp struct {
		DeployOutputs []plan.Filter `json:"deployOutputs,omitempty"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	s.DeployOutputs = temp.DeployOutputs

	return s.Step.UnmarshalJSON(data)
}
