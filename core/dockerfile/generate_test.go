package dockerfile

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/plan"
)

// buildGoPlan creates a BuildPlan equivalent to what the Go provider generates.
// Step "build": FROM debian:bookworm-slim, COPY . ., RUN go build -o /app/server .
// Deploy: COPY --from=build /app/server, CMD /app/server
func buildGoPlan() *plan.BuildPlan {
	p := plan.NewBuildPlan()

	buildStep := plan.NewStep("build")
	buildStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	buildStep.Commands = []plan.Command{
		plan.NewExecShellCommand("go build -o /app/server ."),
	}
	p.AddStep(*buildStep)

	p.Deploy = plan.Deploy{
		Base: plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{
			plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/server"}}),
		},
		StartCmd: "/app/server",
	}

	return p
}

// buildShellPlan creates a BuildPlan equivalent to what the Shell provider generates.
// Step "build": FROM debian:bookworm-slim, COPY . .
// Deploy: COPY --from=build /app, CMD bash start.sh
func buildShellPlan() *plan.BuildPlan {
	p := plan.NewBuildPlan()

	buildStep := plan.NewStep("build")
	buildStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	buildStep.Commands = []plan.Command{
		plan.NewCopyCommand("."),
	}
	p.AddStep(*buildStep)

	p.Deploy = plan.Deploy{
		Base: plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{
			plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
		},
		StartCmd: "bash start.sh",
	}

	return p
}

func TestGenerate_GoSimple(t *testing.T) {
	p := buildGoPlan()
	got, err := Generate(p)
	require.NoError(t, err)
	assertGolden(t, "go_simple.dockerfile", got)
}

func TestGenerate_ShellScript(t *testing.T) {
	p := buildShellPlan()
	got, err := Generate(p)
	require.NoError(t, err)
	assertGolden(t, "shell_script.dockerfile", got)
}

// buildNodePlan creates a BuildPlan equivalent to what the Node provider generates.
// Step "install": FROM debian:bookworm-slim, COPY . ., RUN npm install
// Step "build": FROM install, COPY . .
// Deploy: COPY --from=build /app, CMD npm start
func buildNodePlan() *plan.BuildPlan {
	p := plan.NewBuildPlan()

	installStep := plan.NewStep("install")
	installStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	installStep.Commands = []plan.Command{
		plan.NewExecShellCommand("npm install"),
	}
	p.AddStep(*installStep)

	buildStep := plan.NewStep("build")
	buildStep.Inputs = []plan.Layer{
		plan.NewStepLayer("install"),
	}
	buildStep.Commands = []plan.Command{
		plan.NewCopyCommand("."),
	}
	p.AddStep(*buildStep)

	p.Deploy = plan.Deploy{
		Base: plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{
			plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
		},
		StartCmd: "npm start",
	}

	return p
}

// buildPythonRequirementsPlan creates a BuildPlan for Python with requirements.txt.
func buildPythonRequirementsPlan() *plan.BuildPlan {
	p := plan.NewBuildPlan()

	installStep := plan.NewStep("install")
	installStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	installStep.Commands = []plan.Command{
		plan.NewExecShellCommand("pip install -r requirements.txt"),
	}
	p.AddStep(*installStep)

	p.Deploy = plan.Deploy{
		Base: plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{
			plan.NewStepLayer("install", plan.Filter{Include: []string{"."}}),
		},
		StartCmd: "gunicorn app:app",
	}

	return p
}

// buildPythonPipfilePlan creates a BuildPlan for Python with Pipfile.
func buildPythonPipfilePlan() *plan.BuildPlan {
	p := plan.NewBuildPlan()

	installStep := plan.NewStep("install")
	installStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	installStep.Commands = []plan.Command{
		plan.NewExecShellCommand("pip install pipenv && pipenv install --deploy --system"),
	}
	p.AddStep(*installStep)

	p.Deploy = plan.Deploy{
		Base: plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{
			plan.NewStepLayer("install", plan.Filter{Include: []string{"."}}),
		},
		StartCmd: "python main.py",
	}

	return p
}

// buildStaticfilePlan creates a BuildPlan equivalent to what the Staticfile provider generates.
func buildStaticfilePlan() *plan.BuildPlan {
	p := plan.NewBuildPlan()

	buildStep := plan.NewStep("build")
	buildStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	buildStep.Commands = []plan.Command{
		plan.NewCopyCommand("."),
	}
	p.AddStep(*buildStep)

	p.Deploy = plan.Deploy{
		Base: plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{
			plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
		},
		StartCmd: "python -m http.server 80",
	}

	return p
}

func TestGenerate_NodeNpm(t *testing.T) {
	p := buildNodePlan()
	got, err := Generate(p)
	require.NoError(t, err)
	assertGolden(t, "node_npm.dockerfile", got)
}

func TestGenerate_PythonRequirements(t *testing.T) {
	p := buildPythonRequirementsPlan()
	got, err := Generate(p)
	require.NoError(t, err)
	assertGolden(t, "python_requirements.dockerfile", got)
}

func TestGenerate_PythonPipfile(t *testing.T) {
	p := buildPythonPipfilePlan()
	got, err := Generate(p)
	require.NoError(t, err)
	assertGolden(t, "python_pipfile.dockerfile", got)
}

func TestGenerate_Staticfile(t *testing.T) {
	p := buildStaticfilePlan()
	got, err := Generate(p)
	require.NoError(t, err)
	assertGolden(t, "staticfile.dockerfile", got)
}

// buildPlanWithCaches creates a BuildPlan with APT cache mounts (locked).
func buildPlanWithCaches() *plan.BuildPlan {
	p := plan.NewBuildPlan()

	aptStep := plan.NewStep("packages-apt-runtime")
	aptStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
	}
	aptStep.Commands = []plan.Command{
		plan.NewExecShellCommand("apt-get update && apt-get install -y libpq-dev"),
	}
	aptStep.Caches = []string{"apt", "apt-lists"}
	aptStep.Secrets = []string{}
	p.AddStep(*aptStep)

	p.Caches = map[string]*plan.Cache{
		"apt":       {Directory: "/var/cache/apt", Type: plan.CacheTypeLocked},
		"apt-lists": {Directory: "/var/lib/apt/lists", Type: plan.CacheTypeLocked},
	}

	installStep := plan.NewStep("install")
	installStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	installStep.Commands = []plan.Command{
		plan.NewExecShellCommand("pip install -r requirements.txt"),
	}
	p.AddStep(*installStep)

	p.Deploy = plan.Deploy{
		Base: plan.NewStepLayer("packages-apt-runtime"),
		Inputs: []plan.Layer{
			plan.NewStepLayer("install", plan.Filter{Include: []string{"."}}),
		},
		StartCmd: "gunicorn app:app",
	}

	return p
}

// buildPlanWithSecrets creates a BuildPlan with secret mounts.
func buildPlanWithSecrets() *plan.BuildPlan {
	p := plan.NewBuildPlan()

	p.Secrets = []string{"DATABASE_URL", "API_KEY"}

	installStep := plan.NewStep("install")
	installStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	installStep.Commands = []plan.Command{
		plan.NewExecShellCommand("pip install -r requirements.txt"),
	}
	// Secrets: ["*"] is the default from NewStep
	p.AddStep(*installStep)

	p.Deploy = plan.Deploy{
		Base: plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{
			plan.NewStepLayer("install", plan.Filter{Include: []string{"."}}),
		},
		StartCmd: "python app.py",
	}

	return p
}

func TestGenerate_WithCaches(t *testing.T) {
	p := buildPlanWithCaches()
	got, err := Generate(p)
	require.NoError(t, err)
	assertGolden(t, "with_caches.dockerfile", got)
}

func TestGenerate_WithSecrets(t *testing.T) {
	p := buildPlanWithSecrets()
	got, err := Generate(p)
	require.NoError(t, err)
	assertGolden(t, "with_secrets.dockerfile", got)
}

func TestGenerate_EmptyPlan(t *testing.T) {
	p := plan.NewBuildPlan()
	_, err := Generate(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no steps")
}

func TestGenerate_NilPlan(t *testing.T) {
	_, err := Generate(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no steps")
}

func TestGenerate_NoStartCommand(t *testing.T) {
	p := plan.NewBuildPlan()

	buildStep := plan.NewStep("build")
	buildStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	buildStep.Commands = []plan.Command{
		plan.NewCopyCommand("."),
	}
	p.AddStep(*buildStep)

	p.Deploy = plan.Deploy{
		Base: plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{
			plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
		},
	}

	got, err := Generate(p)
	require.NoError(t, err, "no start command should not be an error")
	require.NotContains(t, got, "CMD", "should not have CMD when no start command")
}

func TestGenerate_StepBasedDeployBase(t *testing.T) {
	p := buildPlanWithCaches()
	got, err := Generate(p)
	require.NoError(t, err)
	require.Contains(t, got, "FROM packages-apt-runtime\n",
		"deploy should use step-based base")
}

func TestGenerate_DeployWithPaths(t *testing.T) {
	p := plan.NewBuildPlan()

	buildStep := plan.NewStep("build")
	buildStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	buildStep.Commands = []plan.Command{
		plan.NewExecShellCommand("go build -o /app/server ."),
	}
	p.AddStep(*buildStep)

	p.Deploy = plan.Deploy{
		Base: plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{
			plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/server"}}),
		},
		StartCmd: "/app/server",
		Paths:    []string{"/usr/local/go/bin", "/app/bin"},
	}

	got, err := Generate(p)
	require.NoError(t, err)
	require.Contains(t, got, "ENV PATH=/usr/local/go/bin:/app/bin:$PATH")
}

func TestGenerate_StepWithVariables(t *testing.T) {
	p := plan.NewBuildPlan()

	buildStep := plan.NewStep("build")
	buildStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	buildStep.Variables = map[string]string{
		"NODE_ENV":    "production",
		"CGO_ENABLED": "0",
	}
	buildStep.Commands = []plan.Command{
		plan.NewExecShellCommand("go build -o /app/server ."),
	}
	p.AddStep(*buildStep)

	p.Deploy = plan.Deploy{
		Base:     plan.NewImageLayer("debian:bookworm-slim"),
		Inputs:   []plan.Layer{plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/server"}})},
		StartCmd: "/app/server",
	}

	got, err := Generate(p)
	require.NoError(t, err)
	// ENV vars should be sorted
	require.Contains(t, got, "ENV CGO_ENABLED=\"0\"\nENV NODE_ENV=\"production\"\n")
}

func TestGenerate_DeployWithVariables(t *testing.T) {
	p := plan.NewBuildPlan()

	buildStep := plan.NewStep("build")
	buildStep.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	buildStep.Commands = []plan.Command{
		plan.NewExecShellCommand("npm install"),
	}
	p.AddStep(*buildStep)

	p.Deploy = plan.Deploy{
		Base:   plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{plan.NewStepLayer("build", plan.Filter{Include: []string{"."}})},
		Variables: map[string]string{
			"NODE_ENV": "production",
			"PORT":     "3000",
		},
		StartCmd: "npm start",
	}

	got, err := Generate(p)
	require.NoError(t, err)
	require.Contains(t, got, "ENV NODE_ENV=\"production\"\nENV PORT=\"3000\"\n")
}

func TestGenerate_ExecCommandWithSingleQuotes(t *testing.T) {
	p := plan.NewBuildPlan()

	step := plan.NewStep("build")
	step.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	step.Commands = []plan.Command{
		plan.NewExecShellCommand("echo 'hello world'"),
	}
	p.AddStep(*step)

	p.Deploy = plan.Deploy{
		Base:     plan.NewImageLayer("debian:bookworm-slim"),
		Inputs:   []plan.Layer{plan.NewStepLayer("build", plan.Filter{Include: []string{"."}})},
		StartCmd: "echo 'hello'",
	}

	got, err := Generate(p)
	require.NoError(t, err)
	// The shell command wraps in single quotes via ShellCommandString,
	// so embedded single quotes appear in the RUN instruction.
	require.Contains(t, got, "RUN sh -c 'echo 'hello world''")
}

func TestGenerate_ExecCommandWithEnvVarReference(t *testing.T) {
	p := plan.NewBuildPlan()

	step := plan.NewStep("build")
	step.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	step.Commands = []plan.Command{
		plan.NewExecShellCommand("echo $HOME && ls $GOPATH/bin"),
	}
	p.AddStep(*step)

	p.Deploy = plan.Deploy{
		Base:   plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{plan.NewStepLayer("build", plan.Filter{Include: []string{"."}})},
	}

	got, err := Generate(p)
	require.NoError(t, err)
	// Environment variable references must be preserved verbatim in the RUN instruction.
	require.Contains(t, got, "$HOME")
	require.Contains(t, got, "$GOPATH/bin")
	require.Contains(t, got, "RUN sh -c 'echo $HOME && ls $GOPATH/bin'")
}

func TestGenerate_StepWithZeroCommands(t *testing.T) {
	p := plan.NewBuildPlan()

	// A step with no commands should still produce FROM + WORKDIR
	step := plan.NewStep("base")
	step.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
	}
	step.Commands = []plan.Command{}
	p.AddStep(*step)

	p.Deploy = plan.Deploy{
		Base:     plan.NewStepLayer("base"),
		StartCmd: "/app/server",
	}

	got, err := Generate(p)
	require.NoError(t, err)
	require.Contains(t, got, "FROM debian:bookworm-slim AS base\n")
	require.Contains(t, got, "WORKDIR /app\n")
	// No RUN, COPY, or ENV instructions should appear in the step
	// (only FROM + WORKDIR, then the deploy stage)
	lines := strings.Split(got, "\n")
	stepLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "RUN ") {
			stepLines++
		}
	}
	require.Equal(t, 0, stepLines, "step with no commands should produce no RUN instructions")
}

func TestGenerate_FileCommandWithMultilineContent(t *testing.T) {
	p := plan.NewBuildPlan()

	step := plan.NewStep("build")
	step.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
	}
	step.Assets["config.yaml"] = "server:\n  host: 0.0.0.0\n  port: 8080\n"
	step.Commands = []plan.Command{
		plan.NewFileCommand("/app", "config.yaml"),
	}
	p.AddStep(*step)

	p.Deploy = plan.Deploy{
		Base:     plan.NewStepLayer("build"),
		StartCmd: "/app/server",
	}

	got, err := Generate(p)
	require.NoError(t, err)
	// The multiline content should be shell-escaped and written via printf
	require.Contains(t, got, "RUN printf '%s'")
	require.Contains(t, got, "config.yaml")
	// Newlines should be preserved in the escaped content
	require.Contains(t, got, "server:\n  host: 0.0.0.0\n  port: 8080\n")
}

func TestGenerate_FileCommandWithSingleQuotesInContent(t *testing.T) {
	p := plan.NewBuildPlan()

	step := plan.NewStep("build")
	step.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
	}
	step.Assets["script.sh"] = "echo 'hello world'\nexit 0"
	step.Commands = []plan.Command{
		plan.NewFileCommand("/app", "script.sh"),
	}
	p.AddStep(*step)

	p.Deploy = plan.Deploy{
		Base:     plan.NewStepLayer("build"),
		StartCmd: "/app/server",
	}

	got, err := Generate(p)
	require.NoError(t, err)
	// Single quotes within file content should be escaped via shellEscape:
	// ' → '\'' (end quote, escaped literal quote, start quote)
	require.Contains(t, got, "'\\''hello world'\\''")
}

func TestGenerate_VeryLongCommandString(t *testing.T) {
	p := plan.NewBuildPlan()

	// Build a very long command (1000+ characters)
	longPkg := strings.Repeat("some-very-long-package-name ", 50)
	longCmd := "apt-get install -y " + strings.TrimSpace(longPkg)

	step := plan.NewStep("build")
	step.Inputs = []plan.Layer{
		plan.NewImageLayer("debian:bookworm-slim"),
		plan.NewLocalLayer(),
	}
	step.Commands = []plan.Command{
		plan.NewExecShellCommand(longCmd),
	}
	p.AddStep(*step)

	p.Deploy = plan.Deploy{
		Base:   plan.NewImageLayer("debian:bookworm-slim"),
		Inputs: []plan.Layer{plan.NewStepLayer("build", plan.Filter{Include: []string{"."}})},
	}

	got, err := Generate(p)
	require.NoError(t, err)
	// The full command should appear in the output without truncation
	require.Contains(t, got, longCmd)
	require.Contains(t, got, "RUN sh -c '"+longCmd+"'")
}
