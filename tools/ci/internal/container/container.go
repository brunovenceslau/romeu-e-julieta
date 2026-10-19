// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package container is the e2e step of 10 10.2 (REQ-0837) and the
// runner-class table that selects it (REQ-0812). The workflow has no if
// to select a step for some runners, so tools/ci selects it from the OS
// and the architecture it runs on: the container e2e runs on the Linux
// runners alone, native, since the workload's base is a Linux image and
// julieta has no darwin build (10 10.1).
//
// The step runs "go test -json" and reads its events (read): a run in
// which no test ran or passed fails, so a selection that runs nothing is
// red; a skipped test fails, since a precondition never ends as a skip
// (REQ-0798); and a run whose every failed test met a precondition the
// harness could not meet (the Docker daemon, a transport failure of the
// image pull, the herdr or the mise download) is an infrastructure
// error, which all ends with an exit status of its own. The harness names
// those tests in a private file the step makes and names in InfraFileEnv,
// never in the output, so no text a test prints (a registry's answer, a
// container's output) can turn a failure into an infrastructure error. A
// digest or a checksum that differs from its pin is a test failure,
// always: the harness never names a test for one.
//
// Until tools/release builds romeu (REQ-0131), the e2e tests are the
// julieta cases of the container e2e, which need no romeu build.
package container

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// InfraFileEnv names the variable that gives the e2e tests the path of
// the file they name, one per line, each test that met an
// infrastructure error in. It equals the constant of the same name in
// e2e/internal/container, which a test of this package holds.
const InfraFileEnv = "ROMEU_E2E_INFRA_FILE"

// CacheDir is the directory the harness keeps its downloads in, which
// ci.yml restores and saves with actions/cache, and FixtureLock is the
// fixture mise.lock that, with the herdr and mise pins, makes the key of
// that step (CacheKey; REQ-0796, REQ-0797). CacheDir equals the constant
// of the same name in e2e/internal/container, and the workflows test
// holds the step of ci.yml and of x64.yml to both.
const (
	CacheDir    = "/tmp/romeu-e-julieta-container-e2e"
	FixtureLock = "e2e/internal/container/testdata/fixture.mise.lock"
)

// CacheKey returns the key of the cache step of the workflows: the
// runner's OS and architecture, as the runner names them, so that each
// architecture saves its own entry and a macOS runner restores none,
// then the sha256 of the fixture mise.lock and the sha256 of the herdr
// and mise pins, so that a change of either is a new entry.
func CacheKey(lock []byte, p pins.Pins) string {
	var lines strings.Builder
	for _, r := range []struct {
		name string
		pin  *pins.Release
	}{{"herdr", p.Herdr}, {"mise", p.Mise}} {
		if r.pin != nil {
			fmt.Fprintf(&lines, "%s %s %s %s\n", r.name, r.pin.Version, r.pin.SHA256.Amd64, r.pin.SHA256.Arm64)
		}
	}
	return "container-e2e-${{ runner.os }}-${{ runner.arch }}-" + sha256Hex([]byte(lines.String()+"\x00"+string(lock)))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Classes is the runner-class table of 10 10.2: by "<GOOS>/<GOARCH>",
// the runner-selected steps each class runs. The four classes are the
// runners of 10 10.2, Runners; a platform outside it is no runner.
var Classes = map[string][]string{
	"linux/amd64":  {"e2e"},
	"linux/arm64":  {"e2e"},
	"darwin/amd64": nil,
	"darwin/arm64": nil,
}

// testTimeout is the -timeout of the e2e run: longer than the 10
// minutes go test allows by default, for the pull of the workload's
// image, and shorter than the limit of the step's class, so that a hung
// test prints its stack before the group is killed. The step runs in
// the zero class, procgroup.ToolStep.
const testTimeout = "14m"

// Steps returns the e2e step of this runner (stepsFor).
func Steps(_ context.Context, p check.Plan) []check.Step {
	return stepsFor(runtime.GOOS, runtime.GOARCH, p)
}

// stepsFor returns the e2e step as the runner class goos/goarch selects
// it: the run of the e2e suite, in the environment of the steps with
// the network and the Docker daemon's variables, since the container
// e2e pulls the workload's image and downloads herdr and mise (REQ-0796),
// around the private file of its infrastructure errors (around); on a
// class that selects none, a line that says so; and on a platform that
// is no class, a step that fails.
func stepsFor(goos, goarch string, p check.Plan) []check.Step {
	class := goos + "/" + goarch
	s := check.Step{Name: "e2e"}
	selected, known := Classes[class]
	switch {
	case !known:
		s.Unavailable = fmt.Errorf("%s is no runner class of tools/ci/internal/container", class)
	case !slices.Contains(selected, "e2e"):
		s.Skip = "not run on " + class + ": the container e2e runs on the Linux runners, native (the runner-class table of tools/ci/internal/container)"
	default:
		s.Argv = []string{filepath.Join(p.Tools.GoDir, "go"), "test", "-json", "-race", "-count=1", "-timeout=" + testTimeout, "-tags", "e2e", "./e2e/..."}
		s.Environ = check.WithProcessEnv(check.WithProcessEnv(check.StepEnv(p.Tools.GoDir), check.NetworkPassThrough), check.DockerPassThrough)
		s.Around = around
	}
	return []check.Step{s}
}

// around makes a directory of this user alone with an empty file of
// mode 0600 in it, which the tests name their infrastructure errors in,
// and returns the variable that names it and the reader of the run,
// which reads the file and then removes the directory.
func around() ([]string, check.Read, error) {
	dir, err := os.MkdirTemp("", "ci-e2e-")
	if err != nil {
		return nil, nil, err
	}
	path := filepath.Join(dir, "infra")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, nil, err
	}
	return []string{InfraFileEnv + "=" + path}, func(out []byte, runErr error) ([]byte, error) {
		defer func() { _ = os.RemoveAll(dir) }()
		names, err := os.ReadFile(path)
		if err != nil {
			return out, fmt.Errorf("read the infrastructure errors of the e2e: %w", err)
		}
		return read(out, runErr, strings.Fields(string(names)))
	}, nil
}

// event is what read takes from one event of go test -json.
type event struct {
	Action  string
	Package string
	Test    string
	Output  string
}

// read reads the output of "go test -json" and the error of its run,
// with infra the tests the harness named as having met an
// infrastructure error. It returns the text of the run, the output of
// each event and every line that is no event, and the step's error: none
// when tests ran and passed and none failed or was skipped; a failure
// when no test ran or passed (REQ-0812) and when a test was skipped
// (REQ-0798); an error that wraps check.ErrInfrastructure when every
// failed test is in infra, or a subtest of it is, and every failed
// package holds such a test; and otherwise a failure that names the
// failed tests.
func read(out []byte, runErr error, infra []string) ([]byte, error) {
	var text bytes.Buffer
	ran, passed := 0, 0
	var failedTests, skipped, failedPkgs, pkgsWithFailedTests []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var e event
		if !bytes.HasPrefix(line, []byte("{")) || json.Unmarshal(line, &e) != nil || e.Action == "" {
			text.Write(line)
			text.WriteByte('\n')
			continue
		}
		text.WriteString(e.Output)
		switch {
		case e.Test == "" && e.Action == "fail":
			failedPkgs = append(failedPkgs, e.Package)
		case e.Test == "":
		case e.Action == "run":
			ran++
		case e.Action == "pass":
			passed++
		case e.Action == "skip":
			skipped = append(skipped, e.Test)
		case e.Action == "fail":
			failedTests = append(failedTests, e.Test)
			pkgsWithFailedTests = append(pkgsWithFailedTests, e.Package)
		}
	}
	if err := sc.Err(); err != nil {
		return text.Bytes(), fmt.Errorf("read the events of go test: %w", err)
	}
	if ran == 0 {
		return text.Bytes(), errors.New("no test ran: a step whose go test -json output reports zero tests fails (REQ-0812)")
	}
	if len(skipped) > 0 {
		return text.Bytes(), fmt.Errorf("skipped, which a test never is (REQ-0798): %s", strings.Join(skipped, ", "))
	}
	if len(failedTests) == 0 && len(failedPkgs) == 0 {
		switch {
		case runErr != nil:
			return text.Bytes(), fmt.Errorf("no test failed, and go test did: %w", runErr)
		case passed == 0:
			return text.Bytes(), errors.New("no test passed")
		}
		return text.Bytes(), nil
	}
	var notInfra []string
	for _, name := range failedTests {
		covered := slices.ContainsFunc(infra, func(i string) bool { return i == name || strings.HasPrefix(i, name+"/") })
		if !covered {
			notInfra = append(notInfra, name)
		}
	}
	for _, pkg := range failedPkgs {
		if !slices.Contains(pkgsWithFailedTests, pkg) {
			notInfra = append(notInfra, pkg)
		}
	}
	if len(failedTests) > 0 && len(notInfra) == 0 {
		return text.Bytes(), fmt.Errorf("every failed test met a precondition it could not meet: %w", check.ErrInfrastructure)
	}
	return text.Bytes(), fmt.Errorf("failed: %s", strings.Join(notInfra, ", "))
}
