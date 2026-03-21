package dockerfile

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/usetheo/theopacks/core/plan"
)

// Generate converts a BuildPlan into a Dockerfile string.
// Each Step becomes a named multi-stage build stage.
// The Deploy section becomes the final (unnamed) stage.
func Generate(p *plan.BuildPlan) (string, error) {
	if p == nil || len(p.Steps) == 0 {
		return "", fmt.Errorf("build plan has no steps")
	}

	var b strings.Builder

	for i, step := range p.Steps {
		if i > 0 {
			b.WriteString("\n")
		}
		if err := writeStep(&b, &step, p); err != nil {
			return "", fmt.Errorf("step %q: %w", step.Name, err)
		}
	}

	b.WriteString("\n")
	writeDeploy(&b, &p.Deploy)

	return b.String(), nil
}

func writeStep(b *strings.Builder, step *plan.Step, p *plan.BuildPlan) error {
	if len(step.Inputs) == 0 {
		return fmt.Errorf("step has no inputs")
	}

	// FROM — first input determines the base
	base := step.Inputs[0]
	writeFrom(b, base, step.Name)
	b.WriteString("WORKDIR /app\n")

	// Additional inputs (index 1+)
	for _, input := range step.Inputs[1:] {
		writeLayerCopy(b, input)
	}

	// ENV — sorted for determinism
	writeEnvVars(b, step.Variables)

	// Accumulate paths for ENV PATH
	var paths []string

	// Commands
	for _, cmd := range step.Commands {
		switch c := cmd.(type) {
		case plan.ExecCommand:
			writeRunCommand(b, c, step, p)
		case plan.CopyCommand:
			writeCopyCommand(b, c)
		case plan.FileCommand:
			writeFileCommand(b, c, step)
		case plan.PathCommand:
			paths = append(paths, c.Path)
		}
	}

	// PATH accumulation
	if len(paths) > 0 {
		fmt.Fprintf(b, "ENV PATH=%s:$PATH\n", strings.Join(paths, ":"))
	}

	return nil
}

func writeDeploy(b *strings.Builder, deploy *plan.Deploy) {
	writeFrom(b, deploy.Base, "")
	b.WriteString("WORKDIR /app\n")

	// COPY --from for each deploy input
	for _, input := range deploy.Inputs {
		writeDeployInput(b, input)
	}

	// ENV vars
	writeEnvVars(b, deploy.Variables)

	// PATH
	if len(deploy.Paths) > 0 {
		fmt.Fprintf(b, "ENV PATH=%s:$PATH\n", strings.Join(deploy.Paths, ":"))
	}

	// CMD
	if deploy.StartCmd != "" {
		fmt.Fprintf(b, "CMD [\"/bin/bash\", \"-c\", %q]\n", deploy.StartCmd)
	}
}

func writeFrom(b *strings.Builder, layer plan.Layer, stageName string) {
	var from string
	switch {
	case layer.Image != "":
		from = layer.Image
	case layer.Step != "":
		from = sanitizeStageName(layer.Step)
	default:
		from = "scratch"
	}

	if stageName != "" {
		fmt.Fprintf(b, "FROM %s AS %s\n", from, sanitizeStageName(stageName))
	} else {
		fmt.Fprintf(b, "FROM %s\n", from)
	}
}

func writeLayerCopy(b *strings.Builder, layer plan.Layer) {
	if layer.Local {
		b.WriteString("COPY . .\n")
		return
	}

	if layer.Step != "" {
		src := sanitizeStageName(layer.Step)
		for _, inc := range layer.Include {
			srcPath, destPath := resolveDeployPaths(inc)
			fmt.Fprintf(b, "COPY --from=%s %s %s\n", src, srcPath, destPath)
		}
		return
	}

	if layer.Image != "" {
		for _, inc := range layer.Include {
			srcPath, destPath := resolveDeployPaths(inc)
			fmt.Fprintf(b, "COPY --from=%s %s %s\n", layer.Image, srcPath, destPath)
		}
	}
}

func writeDeployInput(b *strings.Builder, layer plan.Layer) {
	if layer.Step == "" {
		return
	}
	src := sanitizeStageName(layer.Step)

	if len(layer.Include) == 0 {
		fmt.Fprintf(b, "COPY --from=%s /app /app\n", src)
		return
	}

	for _, inc := range layer.Include {
		srcPath, destPath := resolveDeployPaths(inc)
		fmt.Fprintf(b, "COPY --from=%s %s %s\n", src, srcPath, destPath)
	}
}

func writeRunCommand(b *strings.Builder, cmd plan.ExecCommand, step *plan.Step, p *plan.BuildPlan) {
	var mounts []string

	// Cache mounts
	for _, cacheName := range step.Caches {
		if cache, ok := p.Caches[cacheName]; ok {
			sharing := cache.Type
			if sharing == "" {
				sharing = plan.CacheTypeShared
			}
			mounts = append(mounts, fmt.Sprintf("--mount=type=cache,target=%s,sharing=%s", cache.Directory, sharing))
		}
	}

	// Secret mounts
	secrets := resolveSecrets(step.Secrets, p.Secrets)
	for _, secret := range secrets {
		mounts = append(mounts, fmt.Sprintf("--mount=type=secret,id=%s", secret))
	}

	if len(mounts) > 0 {
		sort.Strings(mounts)
		fmt.Fprintf(b, "RUN %s \\\n    %s\n", strings.Join(mounts, " \\\n    "), cmd.Cmd)
	} else {
		fmt.Fprintf(b, "RUN %s\n", cmd.Cmd)
	}
}

func writeCopyCommand(b *strings.Builder, cmd plan.CopyCommand) {
	if cmd.Image != "" {
		fmt.Fprintf(b, "COPY --from=%s %s %s\n", cmd.Image, cmd.Src, cmd.Dest)
	} else {
		fmt.Fprintf(b, "COPY %s %s\n", cmd.Src, cmd.Dest)
	}
}

func writeFileCommand(b *strings.Builder, cmd plan.FileCommand, step *plan.Step) {
	content, ok := step.Assets[cmd.Name]
	if !ok {
		return
	}

	dest := filepath.Join(cmd.Path, cmd.Name)
	escaped := strings.ReplaceAll(content, "'", "'\\''")
	fmt.Fprintf(b, "RUN printf '%%s' '%s' > %s\n", escaped, dest)
}

func writeEnvVars(b *strings.Builder, vars map[string]string) {
	if len(vars) == 0 {
		return
	}

	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(b, "ENV %s=%q\n", k, vars[k])
	}
}

// resolveDeployPaths converts a BuildPlan include path to Dockerfile COPY src/dest.
// "." → "/app /app" (copy entire workdir)
// "/app/server" → "/app/server /app/server" (absolute path, keep as-is)
// "requirements.txt" → "/app/requirements.txt /app/requirements.txt" (relative, prefix with /app)
func resolveDeployPaths(include string) (string, string) {
	if include == "." {
		return "/app", "/app"
	}

	if filepath.IsAbs(include) {
		return include, include
	}

	abs := filepath.Join("/app", include)
	return abs, abs
}

// resolveSecrets expands "*" wildcard into all plan-level secrets.
func resolveSecrets(stepSecrets, planSecrets []string) []string {
	if len(stepSecrets) == 0 || len(planSecrets) == 0 {
		return nil
	}

	for _, s := range stepSecrets {
		if s == "*" {
			sorted := make([]string, len(planSecrets))
			copy(sorted, planSecrets)
			sort.Strings(sorted)
			return sorted
		}
	}

	sorted := make([]string, len(stepSecrets))
	copy(sorted, stepSecrets)
	sort.Strings(sorted)
	return sorted
}

// sanitizeStageName converts step names to valid Dockerfile stage names.
// "packages:apt:runtime" → "packages-apt-runtime"
func sanitizeStageName(name string) string {
	return strings.ReplaceAll(name, ":", "-")
}
