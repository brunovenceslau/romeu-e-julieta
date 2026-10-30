// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hostpaths"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// sbxReader is what doctor reads from sbx; every method is a read.
type sbxReader interface {
	// RequireFloor reads sbx version and refuses one below the floor
	// with sbx-floor (RJ-343), returning the version it read all the
	// same.
	RequireFloor(ctx context.Context) (sbxdrv.Release, error)
	// List is sbx ls --json, decoded.
	List(ctx context.Context) ([]sbxdrv.Sandbox, error)
	// PolicyList and SecretList are the two listings of I22.
	PolicyList(ctx context.Context) ([]byte, error)
	SecretList(ctx context.Context) ([]byte, error)
}

// seams are what doctor reaches outside the process through; a test
// replaces them.
type seams struct {
	// getenv reads the environment.
	getenv func(string) string
	// goos is runtime.GOOS: where VS Code keeps its settings.
	goos string
	// sbx returns the reader of the sbx of the settings h, run with HOME
	// home.
	sbx func(h *spec.HostSettings, home string) sbxReader
	// gitFloor returns the floor check of the git of the settings h,
	// run with HOME home: nil, or the error git-floor (RJ-344).
	gitFloor func(h *spec.HostSettings, home string) func(ctx context.Context) error
	// secret runs one secret argv with exactly HOME and
	// PATH=/usr/bin:/bin and says whether it printed anything on its
	// standard output, which it never keeps (I25).
	secret func(ctx context.Context, argv []string, home string) (printed bool, err error)
}

// host is what the checks read, gathered once per run: HOME, the host
// settings, the paths of the settings file and the state directory, the
// state directory open to read, and sbx. A value that could not be read
// keeps its error, which each check that needs it reports as its own
// result error; doctor itself goes on (04 4.2, REQ-0331).
type host struct {
	seams
	env *cli.Env

	home         string
	settings     spec.HostSettings
	settingsPath string
	stateDir     string
	// store is the state directory open to read; nil when it does not
	// exist yet, which holds no record.
	store *state.Store
	// baseErr is the first of: HOME not absolute, the settings not
	// readable or invalid. A check that needs the settings reports it.
	baseErr error
	// stateErr is the error of a state directory that exists and cannot
	// be opened, or whose path cannot be made.
	stateErr error
	sbxr     sbxReader

	floor    *memo[sbxdrv.Release]
	list     *memo[[]sbxdrv.Sandbox]
	records  *memo[records]
	findings *memo[stepFive]
}

// memo is a value read once, on first use, with its error.
type memo[T any] struct {
	done bool
	v    T
	err  error
}

// get returns the value, reading it with read the first time.
func (m *memo[T]) get(read func() (T, error)) (T, error) {
	if !m.done {
		m.v, m.err = read()
		m.done = true
	}
	return m.v, m.err
}

// newHost gathers what the checks read. It writes nothing: the state
// directory is opened to read (state.OpenRead), so a mode doctor reports
// is the one it found.
func newHost(env *cli.Env, s seams) *host {
	h := &host{
		seams:    s,
		env:      env,
		floor:    &memo[sbxdrv.Release]{},
		list:     &memo[[]sbxdrv.Sandbox]{},
		records:  &memo[records]{},
		findings: &memo[stepFive]{},
	}
	h.home = s.getenv("HOME")
	if !filepath.IsAbs(h.home) {
		h.baseErr = usage("home", "HOME is not an absolute path")
	}
	if p, err := hostpaths.SettingsPath(env, s.getenv); err != nil {
		h.baseErr = cmpErr(h.baseErr, usage("settings-path", err.Error()))
	} else {
		h.settingsPath = p
	}
	settings, err := hostpaths.LoadSettings(env, s.getenv, usage)
	h.baseErr = cmpErr(h.baseErr, err)
	if err == nil {
		h.settings = settings
	}
	if h.baseErr == nil {
		h.sbxr = s.sbx(&h.settings, h.home)
	}
	h.stateDir, err = hostpaths.StateDir(s.getenv)
	if err != nil {
		h.stateErr = usage("state", err.Error())
		return h
	}
	h.store, err = state.OpenRead(h.stateDir)
	if errors.Is(err, fs.ErrNotExist) {
		h.store, err = nil, nil
	}
	h.stateErr = err
	return h
}

// close releases the state directory.
func (h *host) close() {
	if h.store != nil {
		_ = h.store.Close()
	}
}

// cmpErr returns a, or b when a is nil: the first error found.
func cmpErr(a, b error) error {
	if a != nil {
		return a
	}
	return b
}

// usage returns the error usage (RJ-349) for a precondition a check
// stops at, named by the slug of the rule it breaks, with the hint of
// the help of the command. doctor reports it as the error of each check
// that needs what it names, and exits 1, not 2: its exits are 0, 1 and
// 6 (04 4.2, REQ-0331).
func usage(slug, message string) error {
	e := cli.Usage.New(slug + ": " + message)
	e.Fill = map[string]string{"<binary>": "romeu", " <command>": " doctor"}
	return e
}

// requireFloor returns the sbx version, read once, with the error of the
// floor check: sbx-floor (RJ-343) with the version below the floor, or
// the error that kept the version from being read.
func (h *host) requireFloor(ctx context.Context) (sbxdrv.Release, error) {
	if h.baseErr != nil {
		return sbxdrv.Release{}, h.baseErr
	}
	return h.floor.get(func() (sbxdrv.Release, error) { return h.sbxr.RequireFloor(ctx) })
}

// sandboxes returns the sandboxes sbx lists, read once after the floor
// check, which a call of sbx needs first.
func (h *host) sandboxes(ctx context.Context) ([]sbxdrv.Sandbox, error) {
	if _, err := h.requireFloor(ctx); err != nil {
		return nil, err
	}
	return h.list.get(func() ([]sbxdrv.Sandbox, error) { return h.sbxr.List(ctx) })
}

// runnerSbx is the sbxReader of a sbxdrv.Runner.
type runnerSbx struct{ r *sbxdrv.Runner }

// opts are the options of every sbx call of doctor.
var opts = sbxdrv.Opts{Step: "doctor"}

func (s runnerSbx) RequireFloor(ctx context.Context) (sbxdrv.Release, error) {
	return s.r.RequireFloor(ctx, opts)
}

func (s runnerSbx) List(ctx context.Context) ([]sbxdrv.Sandbox, error) {
	return s.r.List(ctx, opts)
}

func (s runnerSbx) PolicyList(ctx context.Context) ([]byte, error) {
	return s.output(ctx, sbxdrv.PolicyList())
}

func (s runnerSbx) SecretList(ctx context.Context) ([]byte, error) {
	return s.output(ctx, sbxdrv.SecretList())
}

// output runs the read c and returns its standard output; one cut short
// is not a listing doctor can judge.
func (s runnerSbx) output(ctx context.Context, c sbxdrv.Call) ([]byte, error) {
	res, err := s.r.Run(ctx, c, opts)
	if err != nil {
		return nil, err
	}
	if res.Truncated {
		return nil, cli.UpstreamShape.New(fmt.Sprintf("the output of sbx %s is longer than romeu reads", strings.Join(c.Argv, " ")))
	}
	return res.Stdout, nil
}
