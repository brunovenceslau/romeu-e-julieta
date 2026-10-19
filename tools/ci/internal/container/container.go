// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package container is the e2e step of 10 10.2 (REQ-0837) and the
// runner-class table that selects it (REQ-0812). The workflow has no if
// to select a step for some runners, so tools/ci selects it from the OS
// and the architecture it runs on: the container e2e runs on the Linux
// runners alone, native, since the workload's base is a Linux image and
// julieta has no darwin build (10 10.1).
//
// The step runs "go test -json" and reads its events (Read): a run that
// reports zero tests fails, so a selection that runs nothing is red,
// and a run whose every failed test failed on a precondition the
// harness could not meet (the Docker daemon, a transport failure of the
// image pull, the herdr or the mise download; it prints InfraPrefix) is
// an infrastructure error, which all ends with an exit status of its
// own (REQ-0798). A digest or a checksum that differs from its pin is a
// test failure, always: the harness never prints the prefix for one.
//
// Until tools/release builds romeu (REQ-0131), the e2e tests are the
// julieta cases of the container e2e, which need no romeu build.
package container

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// InfraPrefix starts the line the container e2e's harness prints when a
// precondition cannot be met, after the file and line go test prints.
// It equals the constant of the same name in e2e/internal/container,
// which a test of this package holds.
const InfraPrefix = "container e2e: infrastructure error: "

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
// e2e pulls the workload's image and downloads herdr and mise (REQ-0796);
// on a class that selects none, a line that says so; and on a platform
// that is no class, a step that fails.
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
		s.Read = Read
	}
	return []check.Step{s}
}

// event is what Read takes from one event of go test -json.
type event struct {
	Action  string
	Package string
	Test    string
	Output  string
}

// logPrefix is what go test prints before a line a test logs: the
// indentation and the file and line of the call.
var logPrefix = regexp.MustCompile(`^\s*[A-Za-z0-9_.-]+\.go:[0-9]+: `)

// infraLine reports whether a line of a test's output is the harness's
// report of a precondition: InfraPrefix right after the file and line,
// so a line that holds the prefix elsewhere, as an error text a
// registry wrote, is not one.
func infraLine(line string) bool {
	return strings.HasPrefix(logPrefix.ReplaceAllString(line, ""), InfraPrefix) && logPrefix.MatchString(line)
}

// Read reads the output of "go test -json" and the error of its run. It
// returns the text of the run, the output of each event and every line
// that is no event, and the step's error: none when tests ran and none
// failed; "no test ran" when no test ran (REQ-0812); an error that wraps
// check.ErrInfrastructure when every failed test, or a subtest of it,
// printed an infrastructure line and every failed package holds such a
// test; and otherwise a failure that names the failed tests.
func Read(out []byte, runErr error) ([]byte, error) {
	var text bytes.Buffer
	ran := 0
	var failedTests, infraTests, failedPkgs, pkgsWithFailedTests []string
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
		id := e.Package + " " + e.Test
		switch {
		case e.Test == "" && e.Action == "fail":
			failedPkgs = append(failedPkgs, e.Package)
		case e.Test == "":
		case e.Action == "run":
			ran++
		case e.Action == "fail":
			failedTests = append(failedTests, id)
			pkgsWithFailedTests = append(pkgsWithFailedTests, e.Package)
		case e.Action == "output" && infraLine(e.Output):
			infraTests = append(infraTests, id)
		}
	}
	if err := sc.Err(); err != nil {
		return text.Bytes(), fmt.Errorf("read the events of go test: %w", err)
	}
	if ran == 0 {
		return text.Bytes(), errors.New("no test ran: a step whose go test -json output reports zero tests fails (REQ-0812)")
	}
	if len(failedTests) == 0 && len(failedPkgs) == 0 {
		if runErr != nil {
			return text.Bytes(), fmt.Errorf("no test failed, and go test did: %w", runErr)
		}
		return text.Bytes(), nil
	}
	var notInfra []string
	for _, id := range failedTests {
		covered := slices.ContainsFunc(infraTests, func(i string) bool { return i == id || strings.HasPrefix(i, id+"/") })
		if !covered {
			notInfra = append(notInfra, id)
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
