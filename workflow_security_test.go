package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	actionUse = regexp.MustCompile(`^\s*(?:-\s*)?uses:\s*([^@\s]+)@([^\s#]+)`)
	fullSHA   = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// A release job eventually handles signing and publishing credentials. A
// movable action tag lets code outside this repository change what runs on
// that same runner without changing this workflow, so every external action is
// pinned to one reviewed commit. Dependabot keeps those immutable pins moving.
func TestGitHubActionsUseImmutableCommits(t *testing.T) {
	entries, err := os.ReadDir(".github/workflows")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext == ".yml" || ext == ".yaml" {
			paths = append(paths, filepath.Join(".github/workflows", entry.Name()))
		}
	}
	if len(paths) == 0 {
		t.Fatal("no GitHub Actions workflows found")
	}

	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			t.Errorf("open %s: %v", path, err)
			continue
		}

		scanner := bufio.NewScanner(f)
		for line := 1; scanner.Scan(); line++ {
			match := actionUse.FindStringSubmatch(scanner.Text())
			if match == nil {
				continue
			}
			if !fullSHA.MatchString(match[2]) {
				t.Errorf("%s:%d: %s@%s is mutable; pin it to a full commit SHA",
					path, line, match[1], match[2])
			}
		}
		if err := scanner.Err(); err != nil {
			t.Errorf("scan %s: %v", path, err)
		}
		f.Close()
	}
}

// Secret files are short-lived even on a failed notarization run. RUNNER_TEMP
// is private on GitHub-hosted runners, but restrictive creation permissions and
// an always-running cleanup step keep that hosting assumption from becoming
// the only control protecting the imported credentials.
func TestReleaseWorkflowCleansSigningMaterial(t *testing.T) {
	raw, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(raw)

	if strings.Count(workflow, "umask 077") < 2 {
		t.Error("release workflow must set umask 077 before creating certificate and API key files")
	}
	for _, required := range []string{
		"if: always() && runner.os == 'macOS' && needs.verify.outputs.signed == 'true'",
		`rm -f "$RUNNER_TEMP/certificate.p12" "$RUNNER_TEMP/AuthKey.p8"`,
		`security delete-keychain "$RUNNER_TEMP/build.keychain-db"`,
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow is missing cleanup control %q", required)
		}
	}
}

// setup-node controls the project's build toolchain. The action implementations
// are pinned separately to releases whose metadata declares the Node 24 runtime.
func TestGitHubWorkflowsUseNode24(t *testing.T) {
	for _, path := range []string{
		".github/workflows/ci.yml",
		".github/workflows/release.yml",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		workflow := string(raw)
		for _, required := range []string{`NODE_VERSION: "24"`} {
			if !strings.Contains(workflow, required) {
				t.Errorf("%s is missing %s", path, required)
			}
		}
	}
}
