package plan

// BuildPlan is the serializable output of the build plan generation
type BuildPlan struct {
	Steps   []Step            `json:"steps,omitempty"`
	Caches  map[string]*Cache `json:"caches,omitempty"`
	Secrets []string          `json:"secrets,omitempty"`
	Deploy  Deploy            `json:"deploy,omitempty"`
}

type Deploy struct {
	Base      Layer             `json:"base,omitempty"`
	Inputs    []Layer           `json:"inputs,omitempty"`
	StartCmd  string            `json:"startCommand,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Paths     []string          `json:"paths,omitempty"`

	// HealthcheckPath, when set, drives the renderer to emit a HEALTHCHECK
	// directive that probes `http://localhost:<HealthcheckPort>{HealthcheckPath}`.
	// Empty → no HEALTHCHECK is emitted. The framework-aware providers
	// (Spring Boot Actuator, ASP.NET, Rails, etc.) set this when they
	// detect an HTTP server.
	HealthcheckPath string `json:"healthcheckPath,omitempty"`
	// HealthcheckPort defaults to "${PORT:-8080}" when empty (works for the
	// majority of HTTP frameworks via env-var expansion).
	HealthcheckPort string `json:"healthcheckPort,omitempty"`
}

func NewBuildPlan() *BuildPlan {
	return &BuildPlan{
		Steps:   []Step{},
		Deploy:  Deploy{},
		Caches:  make(map[string]*Cache),
		Secrets: []string{},
	}
}

func (p *BuildPlan) AddStep(step Step) {
	p.Steps = append(p.Steps, step)
}

func (p *BuildPlan) Normalize() {
	// Remove empty inputs from steps
	for i := range p.Steps {
		if p.Steps[i].Inputs == nil {
			continue
		}
		normalizedInputs := []Layer{}
		for _, input := range p.Steps[i].Inputs {
			if !input.IsEmpty() {
				normalizedInputs = append(normalizedInputs, input)
			}
		}
		p.Steps[i].Inputs = normalizedInputs
	}

	// Remove empty inputs from deploy
	if p.Deploy.Inputs != nil {
		normalizedDeployInputs := []Layer{}
		for _, input := range p.Deploy.Inputs {
			if !input.IsEmpty() {
				normalizedDeployInputs = append(normalizedDeployInputs, input)
			}
		}
		if len(normalizedDeployInputs) == 0 {
			p.Deploy.Inputs = nil
		} else {
			p.Deploy.Inputs = normalizedDeployInputs
		}
	}

	// Track which steps are referenced by deploy or transitively referenced steps
	referencedSteps := make(map[string]bool)

	if p.Deploy.Base.Step != "" {
		referencedSteps[p.Deploy.Base.Step] = true
	}

	if p.Deploy.Inputs != nil {
		for _, input := range p.Deploy.Inputs {
			if input.Step != "" {
				referencedSteps[input.Step] = true
			}
		}
	}

	checkedSteps := make(map[string]bool)
	maxIterations := len(p.Steps) * len(p.Steps)

	for iterations := 0; iterations < maxIterations; iterations++ {
		newReferences := false
		for _, step := range p.Steps {
			if !referencedSteps[step.Name] {
				continue
			}

			if checkedSteps[step.Name] {
				continue
			}

			checkedSteps[step.Name] = true

			if step.Inputs != nil {
				for _, input := range step.Inputs {
					if input.Step != "" && !referencedSteps[input.Step] {
						referencedSteps[input.Step] = true
						newReferences = true
					}
				}
			}
		}
		if !newReferences {
			break
		}
	}

	// Keep only steps that are referenced
	if len(referencedSteps) > 0 {
		normalizedSteps := make([]Step, 0, len(p.Steps))
		for _, step := range p.Steps {
			if referencedSteps[step.Name] {
				normalizedSteps = append(normalizedSteps, step)
			}
		}
		p.Steps = normalizedSteps
	}
}
