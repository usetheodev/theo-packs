package dockerfile

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/usetheo/theopacks/core/plan"
)

// SyntaxDirective is the BuildKit dockerfile-frontend pin emitted at the top
// of every generated Dockerfile. Pinning the frontend version ensures that
// `--mount=type=cache` (and any future feature we adopt) is parsed even when
// the build host's default frontend is older or has BuildKit explicitly
// disabled. The bare `1` tag resolves to the latest stable v1.x at build time.
const SyntaxDirective = "# syntax=docker/dockerfile:1\n\n"

// HeaderComment returns the metadata block emitted between the syntax
// directive and the first FROM. It names the provider that produced the
// plan and explicitly states the expected docker build context — a
// frequent source of confusion when a monorepo Dockerfile is invoked
// with the wrong context (see docs/contracts/theo-packs-cli-contract.md).
func HeaderComment(providerName string) string {
	if providerName == "" {
		providerName = "unknown"
	}
	return fmt.Sprintf(`# theo-packs: generated for provider %q.
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

`, providerName)
}

// Generate converts a BuildPlan into a Dockerfile string.
// Each Step becomes a named multi-stage build stage.
// The Deploy section becomes the final (unnamed) stage.
func Generate(p *plan.BuildPlan) (string, error) {
	if p == nil || len(p.Steps) == 0 {
		return "", fmt.Errorf("build plan has no steps")
	}

	var b strings.Builder
	b.WriteString(SyntaxDirective)
	b.WriteString(HeaderComment(p.ProviderName))

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

	user, userSetup := resolveDeployUser(deploy.Base)
	if userSetup != "" {
		b.WriteString(userSetup)
	}

	b.WriteString("WORKDIR /app\n")

	if user != "" {
		// /app was created by WORKDIR as root; chown so the eventual `USER`
		// can write to it (logs, runtime asset gen, sqlite DB, etc.).
		fmt.Fprintf(b, "RUN chown %s:%s /app\n", user, user)
	}

	// COPY --from for each deploy input — with --chown when a non-root user is
	// present so the user can read/write the deployed bundle.
	for _, input := range deploy.Inputs {
		writeDeployInputWithUser(b, input, user)
	}

	// ENV vars
	writeEnvVars(b, deploy.Variables)

	// PATH
	if len(deploy.Paths) > 0 {
		fmt.Fprintf(b, "ENV PATH=%s:$PATH\n", strings.Join(deploy.Paths, ":"))
	}

	if user != "" {
		fmt.Fprintf(b, "USER %s\n", user)
	}

	// HEALTHCHECK (when the deploy declares an HTTP healthcheck endpoint).
	if deploy.HealthcheckPath != "" {
		writeHealthcheck(b, deploy.HealthcheckPath, deploy.HealthcheckPort)
	}

	// CMD — exec form when the start command is shell-feature-free, fallback
	// to /bin/sh -c (NEVER /bin/bash — bash is absent on slim/distroless).
	if deploy.StartCmd != "" {
		emitCMD(b, deploy.StartCmd)
	}
}

// resolveDeployUser returns the non-root username to switch to (USER directive)
// and the RUN setup needed to create that user, given the deploy base layer.
//
// Three regimes:
//
//  1. Distroless `:nonroot` (Go/Rust runtime): the image already runs as
//     UID 65532. Empty user + empty setup = no USER directive emitted.
//
//  2. mcr.microsoft.com/dotnet/aspnet (any tag): MS ships an `app` user
//     (UID 1654) by default. Use it without re-creating.
//
//  3. Everything else (debian-slim, eclipse-temurin, ruby/php-cli, deno,
//     dotnet/runtime, nodejs slim, etc.): emit `RUN useradd -r -u 1000 -m
//     appuser` (works on debian) or `RUN adduser -D -u 1000 appuser` for
//     alpine variants.
//
// Operators who need a different UID/username override
// theopacks.json deploy.base.
func resolveDeployUser(base plan.Layer) (user, setup string) {
	image := base.Image
	if image == "" {
		// Step-based deploy bases (rare); leave as-is.
		return "", ""
	}

	// Distroless :nonroot runs as UID 65532 with no shell. No USER directive
	// needed — the image's default already handles it.
	if strings.Contains(image, ":nonroot") || strings.HasPrefix(image, "gcr.io/distroless/") {
		return "", ""
	}

	// MS .NET runtime images ship a built-in `app` user (UID 1654).
	if strings.Contains(image, "mcr.microsoft.com/dotnet/") {
		return "app", ""
	}

	// Alpine variants don't have useradd; use adduser. UID 10001 to avoid
	// conflicts with image-shipped users.
	if strings.Contains(image, "alpine") {
		return "appuser", "RUN adduser -D -u 10001 appuser\n"
	}

	// Default debian/glibc-based runtimes. UID 10001 avoids conflicts with
	// image-shipped users that already claim 1000 (e.g., `node` in node:20-*,
	// `ubuntu` in eclipse-temurin:21).
	return "appuser", "RUN useradd -r -u 10001 -m appuser\n"
}

// writeHealthcheck emits a HEALTHCHECK directive that probes an HTTP endpoint
// via wget (universal across debian-slim, alpine, ruby/php/node images).
// We avoid curl because alpine images often don't ship it. wget is BusyBox
// in alpine and GNU wget elsewhere — both accept `-q -O- <url>` for a silent
// fetch that exits non-zero on HTTP error.
//
// For distroless runtimes (no shell, no wget), the check is skipped — the
// platform (Kubernetes) typically provides its own readiness probe.
func writeHealthcheck(b *strings.Builder, path, port string) {
	if path == "" {
		return
	}
	if port == "" {
		port = "${PORT:-8080}"
	}
	fmt.Fprintf(b,
		"HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \\\n"+
			"    CMD wget -q -O- http://localhost:%s%s || exit 1\n",
		port, path)
}

// USER non-root is provided implicitly by the distroless `:nonroot` images
// used for Go and Rust runtimes (UID 65532). Other runtimes (debian-slim,
// eclipse-temurin, ruby/php-cli, denoland/deno, dotnet/aspnet) run as root
// today; adding a `USER appuser` directive would require a `RUN useradd`
// pre-step that breaks distroless and complicates the renderer for marginal
// security gain. Operators who need it can override via theopacks.json
// deploy.base or by post-processing the generated Dockerfile. Tracking as
// follow-up.

// emitCMD writes the CMD directive choosing the safest form available:
//
//   - exec form `CMD ["arg0", "arg1", ...]` when startCmd is shell-feature-free
//     (no $, ;, &&, ||, |, >, <, `(`, `)`, `{`, `}`, `\`, quotes).
//     This makes the app PID 1 → SIGTERM propagates → graceful shutdown works.
//
//   - shell form `CMD ["/bin/sh", "-c", "<cmd>"]` for env-var expansion
//     (`${PORT:-3000}`), pipes, and other shell features. Uses /bin/sh, NOT
//     /bin/bash — bash is absent from debian:bookworm-slim and distroless.
func emitCMD(b *strings.Builder, startCmd string) {
	startCmd = strings.TrimSpace(startCmd)
	if startCmd == "" {
		return
	}

	if needsShell(startCmd) {
		fmt.Fprintf(b, "CMD [\"/bin/sh\", \"-c\", %q]\n", startCmd)
		return
	}

	// Exec form: split on whitespace into JSON array.
	parts := strings.Fields(startCmd)
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = fmt.Sprintf("%q", p)
	}
	fmt.Fprintf(b, "CMD [%s]\n", strings.Join(quoted, ", "))
}

// needsShell reports whether the command body uses any feature that requires
// a shell to interpret. Conservative — false positives fall back to the safe
// /bin/sh form.
func needsShell(cmd string) bool {
	for _, r := range cmd {
		switch r {
		case '$', ';', '&', '|', '>', '<', '(', ')', '{', '}', '\\', '"', '\'', '`', '*', '?':
			return true
		}
	}
	return false
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

// writeDeployInputWithUser emits a COPY --from directive with optional
// --chown=<user>:<user> when a non-root user is set. The user must already
// exist (resolveDeployUser's setup handles that before the first COPY).
func writeDeployInputWithUser(b *strings.Builder, layer plan.Layer, user string) {
	if layer.Step == "" {
		return
	}
	src := sanitizeStageName(layer.Step)
	chown := ""
	if user != "" {
		chown = fmt.Sprintf("--chown=%s:%s ", user, user)
	}

	if len(layer.Include) == 0 {
		fmt.Fprintf(b, "COPY --from=%s %s/app /app\n", src, chown)
		return
	}

	for _, inc := range layer.Include {
		srcPath, destPath := resolveDeployPaths(inc)
		fmt.Fprintf(b, "COPY --from=%s %s%s %s\n", src, chown, srcPath, destPath)
	}
}

func writeRunCommand(b *strings.Builder, cmd plan.ExecCommand, step *plan.Step, p *plan.BuildPlan) {
	var mounts []string

	// BuildKit cache mounts (per-step, typed) — emitted before legacy named caches.
	for _, mount := range step.BuildKitCaches {
		sharing := mount.Sharing
		if sharing == "" {
			sharing = "locked"
		}
		mounts = append(mounts, fmt.Sprintf("--mount=type=cache,target=%s,sharing=%s", mount.Target, sharing))
	}

	// Legacy named cache mounts (BuildPlan.Caches map).
	for _, cacheName := range step.Caches {
		if cache, ok := p.Caches[cacheName]; ok {
			sharing := cache.Type
			if sharing == "" {
				sharing = plan.CacheTypeShared
			}
			mounts = append(mounts, fmt.Sprintf("--mount=type=cache,target=%s,sharing=%s", cache.Directory, sharing))
		}
	}

	// Secret mounts — only those actually referenced in the command body
	// (substring match on $NAME or ${NAME}). Step.Secrets=["*"] preserves
	// the legacy "mount everything" behavior as an explicit opt-in.
	secrets := resolveSecrets(step.Secrets, p.Secrets, cmd.Cmd)
	for _, secret := range secrets {
		mounts = append(mounts, fmt.Sprintf("--mount=type=secret,id=%s", secret))
	}

	// Render either RUN <body> (Exec) or RUN sh -c '<body>' (Shell). The
	// wrap happens here, not in the constructor, so providers always pass
	// bare command strings and the renderer is the single source of truth
	// for shell-wrapping semantics.
	body := cmd.Cmd
	if cmd.Kind == plan.CommandKindShell {
		body = plan.ShellCommandString(cmd.Cmd)
	}

	if len(mounts) > 0 {
		sort.Strings(mounts)
		fmt.Fprintf(b, "RUN %s \\\n    %s\n", strings.Join(mounts, " \\\n    "), body)
	} else {
		fmt.Fprintf(b, "RUN %s\n", body)
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
	escapedContent := shellEscape(content)
	escapedDest := shellEscape(dest)
	fmt.Fprintf(b, "RUN printf '%%s' %s > %s\n", escapedContent, escapedDest)
}

// shellEscape wraps a string in single quotes, escaping any embedded single quotes.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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

// resolveSecrets returns the secrets to mount on a specific RUN command.
//
// Three modes:
//
//  1. stepSecrets contains "*" (explicit wildcard, opt-in escape hatch):
//     mount every plan-level secret regardless of usage. This preserves
//     the legacy behavior for callers that genuinely need every secret.
//
//  2. stepSecrets non-empty without "*": mount exactly those secrets
//     (sorted, deduped), regardless of usage. The provider has declared
//     intent to expose them.
//
//  3. stepSecrets empty (the new default): substring-match every plan-
//     level secret against the command body and mount only those whose
//     `$NAME` or `${NAME}` token actually appears. This eliminates the
//     spurious mounts that polluted every RUN under the old `["*"]`
//     default.
func resolveSecrets(stepSecrets, planSecrets []string, cmdBody string) []string {
	if len(planSecrets) == 0 {
		return nil
	}

	// Mode 1: explicit "*" wildcard.
	for _, s := range stepSecrets {
		if s == "*" {
			sorted := make([]string, len(planSecrets))
			copy(sorted, planSecrets)
			sort.Strings(sorted)
			return sorted
		}
	}

	// Mode 2: explicit list.
	if len(stepSecrets) > 0 {
		sorted := make([]string, len(stepSecrets))
		copy(sorted, stepSecrets)
		sort.Strings(sorted)
		return sorted
	}

	// Mode 3: auto-detect via substring match.
	return autoDetectSecrets(cmdBody, planSecrets)
}

// autoDetectSecrets returns the subset of planSecrets whose `$NAME` or
// `${NAME}` token appears in cmdBody. Token boundaries are checked so
// `$THEOPACKS_VARS_FOO` does NOT match a secret named `THEOPACKS_VARS`
// — only `$NAME` followed by a non-identifier character (or end of string)
// counts.
func autoDetectSecrets(cmdBody string, planSecrets []string) []string {
	var matched []string
	for _, name := range planSecrets {
		if name == "" {
			continue
		}
		if secretReferencedIn(cmdBody, name) {
			matched = append(matched, name)
		}
	}
	if len(matched) == 0 {
		return nil
	}
	sort.Strings(matched)
	return matched
}

// secretReferencedIn reports whether body contains either `${NAME}` (always
// safe — braces close the token) or `$NAME` followed by a non-identifier
// character / end-of-string. Identifier chars are [A-Za-z0-9_]; anything
// else (space, quote, end of body) terminates the token.
func secretReferencedIn(body, name string) bool {
	// ${NAME} form — unambiguous.
	if strings.Contains(body, "${"+name+"}") {
		return true
	}
	// $NAME form — needs token-boundary check.
	prefix := "$" + name
	for i := 0; i+len(prefix) <= len(body); {
		idx := strings.Index(body[i:], prefix)
		if idx < 0 {
			break
		}
		end := i + idx + len(prefix)
		if end == len(body) || !isIdentChar(body[end]) {
			return true
		}
		i = i + idx + 1
	}
	return false
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// sanitizeStageName converts step names to valid Dockerfile stage names.
// "packages:apt:runtime" → "packages-apt-runtime"
func sanitizeStageName(name string) string {
	return strings.ReplaceAll(name, ":", "-")
}
