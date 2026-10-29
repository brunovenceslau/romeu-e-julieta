// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// Trust runs "mise trust" on the mise.toml of the repo (04 4.3,
// REQ-1306), with the environment of Install but without
// MISE_TRUSTED_CONFIG_PATHS and MISE_PARANOID, as the agent's shell has
// it, so the trust is recorded where that shell reads it. The errors are
// Install's but for the blocked download.
func (d *Driver) Trust(ctx context.Context, t Target) error {
	env := slices.DeleteFunc(slices.Clone(d.Env), func(kv string) bool {
		return strings.HasPrefix(kv, TrustedPathsEnv+"=") || strings.HasPrefix(kv, "MISE_PARANOID=")
	})
	args := []string{"trust", filepath.Join(t.Dir, ConfigFile)}
	out := &capture{}
	err := d.start(ctx, procgroup.Command{
		Class:  procgroup.Mise,
		Argv:   d.argv(args...),
		Dir:    t.Dir,
		Env:    env,
		Stdout: out,
		Stderr: out,
	})
	if err == nil {
		return nil
	}
	if timeout, ok := errors.AsType[*procgroup.TimeoutError](err); ok {
		return timeoutError(timeout, "trust", t, out)
	}
	return failure(err, d.argv(args...), "trust", t, out)
}

// listLimit bounds what HasTask keeps of the list of tasks.
const listLimit = 1 << 20

// limited keeps the first listLimit bytes written and drops the rest,
// and never fails a write, so mise never blocks on a full pipe.
type limited struct{ buf bytes.Buffer }

// Write keeps what fits and reports all of p written.
func (l *limited) Write(p []byte) (int, error) {
	if room := listLimit - l.buf.Len(); room > 0 {
		l.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

// listedTask is the part of a task of "mise tasks ls --json" HasTask
// reads: its name and the file that defines it.
type listedTask struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// HasTask reports whether "mise tasks ls --json" lists a task named name
// that the repo's mise.toml defines, with the environment of Install
// (04 4.3, REQ-1307). A file task of a directory such as mise-tasks is
// not the mise.toml's, which the guard of the repo's configuration
// judges (tools/ci/internal/lint), so it is not run. No output at all is
// no task. The error is the one of the run, as
// internal/procgroup returns it, or of reading the list.
func (d *Driver) HasTask(ctx context.Context, t Target, name string) (bool, error) {
	var stdout limited
	err := d.start(ctx, procgroup.Command{
		Class:  procgroup.Mise,
		Argv:   d.argv("tasks", "ls", "--json"),
		Dir:    t.Dir,
		Env:    d.Env,
		Stdout: &stdout,
	})
	if err != nil {
		return false, err
	}
	if len(bytes.TrimSpace(stdout.buf.Bytes())) == 0 {
		return false, nil
	}
	var tasks []listedTask
	if err := json.Unmarshal(stdout.buf.Bytes(), &tasks); err != nil {
		return false, err
	}
	source := filepath.Join(t.Dir, ConfigFile)
	return slices.ContainsFunc(tasks, func(task listedTask) bool {
		return task.Name == name && filepath.Clean(task.Source) == source
	}), nil
}

// RunTask runs "mise run <name>" in the repo, with the environment of
// Install and the limit of the class (04 4.3, REQ-1307). Its output is
// dropped, and the error is the one of the run, as internal/procgroup
// returns it: a *procgroup.ExitError or a *procgroup.TimeoutError.
func (d *Driver) RunTask(ctx context.Context, t Target, name string) error {
	return d.start(ctx, procgroup.Command{
		Class: procgroup.Mise,
		Argv:  d.argv("run", name),
		Dir:   t.Dir,
		Env:   d.Env,
	})
}
