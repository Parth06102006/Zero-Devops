package deployments

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var nodePin = regexp.MustCompile(`^v?(20|22|24)(\.(0|[1-9][0-9]*))?(\.(0|[1-9][0-9]*))?$`)
var rangeVersion = regexp.MustCompile(`[0-9]+(?:\.[0-9]+){0,2}`)
var nodeBase = regexp.MustCompile(`(?m)^FROM docker\.io/library/node:[A-Za-z0-9_.-]+`)
var nodeBuilderBase = regexp.MustCompile(`(?m)^FROM docker\.io/library/node:[A-Za-z0-9_.-]+ AS builder$`)
var nodeUnnamedBase = regexp.MustCompile(`(?m)^FROM docker\.io/library/node:[A-Za-z0-9_.-]+$`)

// Choose a compatibility builder from the supported image families. Check the
// actual pulled Node version again before installation; tags can move and a
// bounded engine range might require an exact project-provided pin.
func selectNodeVersion(appDir, engine string) (string, error) {
	var constraint *semver.Constraints
	var err error
	if engine != "" {
		constraint, err = semver.NewConstraint(engine)
		if err != nil {
			return "", fmt.Errorf("invalid package.json engines.node: %w", err)
		}
	}
	pin := ""
	for _, file := range []string{".node-version", ".nvmrc"} {
		path := filepath.Join(appDir, file)
		_, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if err := requireContainedPath(appDir, path); err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		value := strings.TrimSpace(string(data))
		if !nodePin.MatchString(value) {
			return "", fmt.Errorf("%s must pin a numeric Node 20, 22, or 24 version", file)
		}
		value = strings.TrimPrefix(value, "v")
		if pin != "" && pin != value {
			return "", errors.New("conflicting .node-version and .nvmrc pins")
		}
		pin = value
	}
	if pin != "" {
		if constraint != nil && strings.Count(pin, ".") == 2 {
			version, _ := semver.NewVersion(pin)
			if !constraint.Check(version) {
				return "", errors.New("Node version pin does not satisfy engines.node")
			}
		}
		return pin, nil
	}
	if constraint == nil {
		return "24", nil
	}
	for _, major := range []uint64{24, 22, 20} {
		candidates := []*semver.Version{semver.New(major, 0, 0, "", "")}
		for _, literal := range rangeVersion.FindAllString(engine, -1) {
			version, err := semver.NewVersion(literal)
			if err != nil || version.Major() != major {
				continue
			}
			candidates = append(candidates, version, semver.New(major, version.Minor(), version.Patch()+1, "", ""))
		}
		for _, version := range candidates {
			if constraint.Check(version) {
				return fmt.Sprint(major), nil
			}
		}
	}
	return "", fmt.Errorf("engines.node %q has no supported Node 20/22/24 builder; provide a compatible numeric version pin or update the project", engine)
}

func renderNodeToolchain(appDir, content string) (string, error) {
	if !nodeBase.MatchString(content) {
		return content, nil
	}
	pkg, err := readPackageJSON(appDir)
	if err != nil {
		return "", err
	}
	version, err := selectNodeVersion(appDir, pkg.Engines["node"])
	if err != nil {
		return "", err
	}
	content = nodeBase.ReplaceAllString(content, "FROM docker.io/library/node:"+version+"-alpine")
	if pkg.Engines["node"] == "" {
		return content, nil
	}
	// npm ships semver in the official Node images. Read the engine range as
	// data from the manifest; never interpolate repository strings into code.
	code := `const s=require('/usr/local/lib/node_modules/npm/node_modules/semver');const r=require('./package.json').engines.node;if(!s.satisfies(process.versions.node,r)){console.error('Selected Node '+process.versions.node+' does not satisfy engines.node '+r+'; set a compatible exact .node-version');process.exit(1)}`
	command, _ := json.Marshal([]string{"node", "-e", code})
	// Each Node dependency-install stage gets a check; Bun stages stay Bun.
	blocks := strings.Split(content, "\nFROM ")
	for i, block := range blocks {
		prefix := block
		if i > 0 {
			prefix = "FROM " + block
		}
		if !nodeBase.MatchString(prefix) {
			continue
		}
		lines := strings.Split(block, "\n")
		for j, line := range lines {
			if strings.HasPrefix(line, "COPY package") && strings.HasSuffix(line, " ./") {
				lines[j] = line + "\nRUN " + string(command)
			}
		}
		blocks[i] = strings.Join(lines, "\n")
	}
	return strings.Join(blocks, "\nFROM "), nil
}
