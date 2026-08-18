// Package scanner validates project build commands against a versioned
// security policy before they are persisted.
package scanner

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"path/filepath"
	"strings"
)

const (
	PolicyVersion = "v1"
)

// allowedExecutables is the allowlist of build tool executables a command may
// invoke. Anything else — especially a shell interpreter — is denied. Entries
// are matched against the basename of the executable (after trimming "./").
var allowedExecutables = map[string]bool{
	"npm": true, "npx": true, "yarn": true, "pnpm": true, "bun": true, "node": true,
	"go": true, "cargo": true, "rustc": true,
	"mvn": true, "mvnw": true, "gradle": true, "gradlew": true, "sbt": true,
	"python": true, "python3": true, "py": true, "pip": true, "pip3": true,
	"pipenv": true, "poetry": true,
	"ruby": true, "bundle": true, "rake": true,
	"php": true, "composer": true,
	"mix": true, "elixir": true,
	"dotnet": true, "make": true,
}

// shellExecutables are command interpreters. Invoking a shell with raw build
// args would enable arbitrary command execution, so they are always denied.
var shellExecutables = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "korn": true, "fish": true,
	"cmd": true, "powershell": true, "pwsh": true,
}

// injectionRunes are argument fragments that allow command chaining or
// injection when a tool indirection is present. Any argument containing them
// is denied.
var deniedArgFragments = []string{
	";", "|", "&", "`", "$(", "${", "\n",
}

// DefaultScanner is a shared, stateless policy scanner over the "v1" policy.
var DefaultScanner domain.CommandScanner = &policyScanner{}

type policyScanner struct{}

// Scan implements domain.CommandScanner. It never returns an error; an unsafe
// or unrecognized command yields a denied result. The provided policy version
// is not trusted — the scanner records the version it actually enforces.
func (s *policyScanner) Scan(ctx context.Context, cfg domain.BuildConfiguration) domain.CommandScanResult {
	_ = ctx

	executable := strings.TrimSpace(cfg.Executable)
	if executable == "" {
		return denied(PolicyVersion, "executable cannot be empty")
	}

	name := strings.TrimPrefix(executable, "./")
	name = strings.TrimPrefix(name, ".\\")
	name = strings.ToLower(filepath.Base(name))

	if shellExecutables[name] {
		return denied(PolicyVersion, "shell interpreters are not allowed as build executables: "+name)
	}

	if !allowedExecutables[name] {
		return denied(PolicyVersion, "executable is not allowlisted: "+name)
	}

	for _, arg := range cfg.Args {
		if containsFragment(arg, deniedArgFragments) {
			return denied(PolicyVersion, "argument contains command injection fragment")
		}
	}

	return domain.CommandScanResult{
		Status:        domain.CommandScanStatusApproved,
		PolicyVersion: PolicyVersion,
		Source:        domain.CommandScanSourceManual,
		Message:       "command accepted by policy " + PolicyVersion,
	}
}

func denied(reason string, denials ...string) domain.CommandScanResult {
	r := reason
	if len(denials) > 0 && denials[0] != "" {
		r = denials[0]
	}
	return domain.CommandScanResult{
		Status:       domain.CommandScanStatusDenied,
		PolicyVersion: PolicyVersion,
		Source:       domain.CommandScanSourceManual,
		DeniedReason: r,
	}
}

func containsFragment(s string, fragments []string) bool {
	for _, f := range fragments {
		if strings.Contains(s, f) {
			return true
		}
	}
	return false
}