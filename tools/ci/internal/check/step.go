// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/goenv"
)

// Step is one command step of fast or all.
type Step struct {
	Name string
	Argv []string
	// Quiet marks a command whose output is its finding, and it means
	// two things: the step passes when it prints nothing, as "gofmt -l"
	// does, and what it prints goes through git.SafeLines, because that
	// output is paths of the repository (the runner of tools/ci).
	Quiet bool
	// Environ is the whole environment of the command (StepEnv). The
	// environment of this process never reaches it, so an empty one is
	// an empty environment.
	Environ []string
	// Unavailable, when set, says why the step's tool could not be
	// resolved: the step fails with it and starts nothing.
	Unavailable error
	// Class is the timeout class the step runs under; the zero value
	// means procgroup.ToolStep, whose limit leaves the unit tests, the
	// longest step, room for a cold build cache.
	Class procgroup.Class
	// Skip, when set, says why the step does not run on this platform:
	// it is reported with "--", neither passed nor failed.
	Skip string
	// NoticeOn is a line prefix: when a step that failed printed a line
	// that starts with it, the verdict adds the line Notice.
	NoticeOn, Notice string
	// Around, when set, runs right before the command: it returns the
	// variables to add to Environ, and Read, which reads what the
	// command printed and the error of its run and returns the text the
	// verdict shows and the step's error, which decides the verdict in
	// place of the exit status, and releases what Around made. The e2e
	// step makes a private file there, which its tests name the tests
	// that met an infrastructure error in, and reads the events of "go
	// test -json". An error that wraps ErrInfrastructure marks the step
	// an infrastructure error. An error of Around fails the step, which
	// then starts nothing.
	Around func() (env []string, read Read, err error)
}

// Read reads the output and the error of a step's run (Step.Around).
type Read func(out []byte, runErr error) ([]byte, error)

// Tools are the pinned tools of the steps, as mise resolves them: the
// path of golangci-lint, and the directory of the go command.
type Tools struct {
	Linter, GoDir string
}

// Three more sets say how to reach something, and only the steps that
// reach it keep them (WithProcessEnv): NetworkPassThrough, a proxy and
// a certificate bundle, for the steps that use the network;
// DockerPassThrough, the Docker daemon, for the license step; and
// GoModulePassThrough, for the go run that the pre-push hook starts
// before tools/ci: where a module comes from (GOPROXY) and which paths
// are private or unchecked (GOPRIVATE, GONOSUMDB, GOSUMDB, GOINSECURE),
// so a person who reaches only a company proxy can run the hook. They
// are safe there because that go run builds with GOFLAGS=-mod=readonly:
// go.sum decides which module content is accepted, and none of the five
// can add an entry to it. They are NOT kept by the mise installs and by
// "go mod download" of setup, where the checksum database is the guard
// (the installs that build govulncheck and gopls have no go.sum).
var (
	NetworkPassThrough  = []string{"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy", "SSL_CERT_FILE"}
	DockerPassThrough   = []string{"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG"}
	GoModulePassThrough = []string{"GOPROXY", "GOPRIVATE", "GONOSUMDB", "GOSUMDB", "GOINSECURE"}
)

// WithProcessEnv returns env with the variables of keys that are set in
// this process, in the order of keys.
func WithProcessEnv(env []string, keys []string) []string {
	out := slices.Clone(env)
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			out = append(out, key+"="+value)
		}
	}
	return out
}

// PassThroughEnv returns the variables of goenv.PassThrough that are set
// in this process, in the order of the list. An empty variable is left
// out, as the go command reads an empty variable as an unset one.
func PassThroughEnv() []string {
	var env []string
	for _, key := range goenv.PassThrough {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// StepEnv is the environment of every step of fast, built from nothing
// rather than from this process's: PATH with goDir first, so that a go
// command the linter or a test starts is the pinned one, the rest of
// PassThroughEnv, then the variables that decide what the go command
// builds and runs, each with a fixed value. Nothing else reaches a
// step, so a variable that changes what a step checks (GOFLAGS with
// "-run" or build tags, a GOLANGCI_ or GL_ variable, GOEXPERIMENT or
// GOAMD64, which change the build tags) has no effect on it.
//
//   - goenv.Pin: GOENV=off, so the go command reads no "go env -w" file,
//     which could set GOFLAGS or any of the variables below, and
//     GOTOOLCHAIN=local, so the go of goDir runs and no other is
//     downloaded.
//   - GOWORK=off: a go.work file in a parent directory does not change
//     the modules.
//   - GOPROXY=off: no step fetches a module, so the module cache must
//     hold the modules of go.sum (10 10.1). Two steps of all reach the
//     network for other reasons: vulnerabilities reads the Go
//     vulnerability database, so its result depends on the date, and
//     license may pull its image, by digest.
//   - GOFLAGS=-mod=readonly: the go command builds from go.mod and the
//     module cache, never from a vendor directory
//     (https://go.dev/ref/mod#build-commands); in a pre-push run,
//     judgeCommit also refuses one that is not tracked.
func StepEnv(goDir string) []string {
	path := goDir
	if value := os.Getenv("PATH"); value != "" {
		path += string(os.PathListSeparator) + value
	}
	rest := slices.DeleteFunc(PassThroughEnv(), func(kv string) bool { return strings.HasPrefix(kv, "PATH=") })
	return slices.Concat([]string{"PATH=" + path}, rest, goenv.Pin, goenv.Build, []string{
		"GOPROXY=off",
	})
}

// Run runs the step in dir and returns its output, or what the Read of
// s.Around makes of it, through
// procgroup.CombinedOutput: the command gets exactly s.Environ, an
// empty environment when it is nil, runs in a process group of its own
// that is killed when the leader ends, and a run past the limit of its
// class is a *procgroup.TimeoutError.
func (s Step) Run(ctx context.Context, dir string) ([]byte, error) {
	if s.Unavailable != nil {
		return nil, s.Unavailable
	}
	env, read := s.Environ, Read(nil)
	if s.Around != nil {
		extra, r, err := s.Around()
		if err != nil {
			return nil, err
		}
		env, read = append(slices.Clone(s.Environ), extra...), r
	}
	out, err := procgroup.CombinedOutput(ctx, procgroup.Command{
		Class: cmp.Or(s.Class, procgroup.ToolStep),
		Argv:  s.Argv,
		Dir:   dir,
		Env:   env,
	})
	if err == nil && s.Quiet && len(bytes.TrimSpace(out)) > 0 {
		err = errors.New("the command printed what it found")
	}
	if read != nil {
		return read(out, err)
	}
	return out, err
}
