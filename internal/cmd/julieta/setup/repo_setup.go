// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// RepoSetupFailed is the warning repo-setup-failed (04 4.1, Warnings;
// REQ-1307): the task setup of a repo's mise configuration failed or ran
// past its budget, "<repo> exit <n>" or "<repo> ran past ...".
const RepoSetupFailed = "repo-setup-failed"

// setupTask is the name of the mise task a repo defines to prepare what
// its own checks need (04 4.3, REQ-1307).
const setupTask = "setup"

// outputLimit bounds what one mise call of this file keeps of its
// output: the list of tasks of a repo, and the first words of an error.
const outputLimit = 1 << 20

// bounded keeps the first outputLimit bytes of what is written and drops
// the rest, so a call never fails a write and mise never blocks on a
// full pipe.
type bounded struct{ buf bytes.Buffer }

// Write keeps what fits and reports all of p written.
func (b *bounded) Write(p []byte) (int, error) {
	if room := outputLimit - b.buf.Len(); room > 0 {
		b.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

// miseRepos returns the repos of the manifest whose mise.toml is a
// regular file, the ones install reads (04 4.3, REQ-0400).
func miseRepos(r *run) []string {
	var dirs []string
	for _, repo := range r.m.Repos {
		info, err := os.Lstat(filepath.Join(repo.Path, mise.ConfigFile))
		if err == nil && info.Mode().IsRegular() {
			dirs = append(dirs, repo.Dir)
		}
	}
	return dirs
}

// repoPath returns the path of the repo with the dir dir.
func repoPath(r *run, dir string) string {
	for _, repo := range r.m.Repos {
		if repo.Dir == dir {
			return repo.Path
		}
	}
	return ""
}

// agentEnv returns the mise environment of install (REQ-0403) without
// MISE_TRUSTED_CONFIG_PATHS and MISE_PARANOID, as the agent's shell has
// it: a trust recorded under it is the one that shell reads (REQ-1306).
func agentEnv(r *run) ([]string, error) {
	env, err := mise.Environ(os.LookupEnv, r.home, repoPaths(r.m))
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(env, func(kv string) bool {
		return strings.HasPrefix(kv, mise.TrustedPathsEnv+"=") || strings.HasPrefix(kv, "MISE_PARANOID=")
	}), nil
}

// trustRepos is the step trust of setup (04 4.3, REQ-1306): mise trust
// on the mise.toml of each repo that has one, before install, in the
// sandbox only. A failure is reported and leaves the sandbox usable.
func trustRepos(ctx context.Context, r *run) {
	dirs := miseRepos(r)
	if len(dirs) == 0 {
		return
	}
	env, err := agentEnv(r)
	if err != nil {
		r.report.Errors = append(r.report.Errors, "trust: "+Describe(err))
		return
	}
	for _, dir := range dirs {
		path := repoPath(r, dir)
		var stderr bounded
		err := procgroup.Run(ctx, procgroup.Command{
			Class:  procgroup.Mise,
			Argv:   []string{"mise", "trust", filepath.Join(path, mise.ConfigFile)},
			Dir:    path,
			Env:    env,
			Stderr: &stderr,
		})
		if err != nil {
			r.report.Errors = append(r.report.Errors, fmt.Sprintf("trust: %s: %s: %s",
				termsafe.Escape(dir), termsafe.Escape(err.Error()), termsafe.Escape(strings.TrimSpace(firstBytes(stderr.buf.String(), 4<<10)))))
		}
	}
}

// firstBytes returns at most n bytes of s.
func firstBytes(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// runRepoTasks is the step repo-setup of setup (04 4.3, REQ-1307): after
// install, mise run setup in each repo whose mise configuration defines
// the task setup, with the environment of install and the budget of its
// class. A task that fails or runs past the budget is the warning
// repo-setup-failed and setup goes on; a repo without the task is
// skipped silently.
func runRepoTasks(ctx context.Context, r *run) {
	dirs := miseRepos(r)
	if len(dirs) == 0 {
		return
	}
	env, err := mise.Environ(os.LookupEnv, r.home, repoPaths(r.m))
	if err != nil {
		r.report.Errors = append(r.report.Errors, "repo setup: "+Describe(err))
		return
	}
	for _, dir := range dirs {
		path := repoPath(r, dir)
		defined, err := definesSetup(ctx, env, path)
		if err != nil {
			r.warn(RepoSetupFailed, fmt.Sprintf("%s the tasks cannot be listed: %s", termsafe.Escape(dir), failure(err)))
			continue
		}
		if !defined {
			continue
		}
		err = procgroup.Run(ctx, procgroup.Command{
			Class: procgroup.Mise,
			Argv:  []string{"mise", "run", setupTask},
			Dir:   path,
			Env:   env,
		})
		if err != nil {
			r.warn(RepoSetupFailed, fmt.Sprintf("%s %s", termsafe.Escape(dir), failure(err)))
		}
	}
}

// definesSetup reports whether mise tasks ls --json lists a task named
// setup in the repo at path. An empty list, or none at all, is no task.
func definesSetup(ctx context.Context, env []string, path string) (bool, error) {
	var stdout bounded
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.Mise,
		Argv:   []string{"mise", "tasks", "ls", "--json"},
		Dir:    path,
		Env:    env,
		Stdout: &stdout,
	})
	if err != nil {
		return false, err
	}
	if len(bytes.TrimSpace(stdout.buf.Bytes())) == 0 {
		return false, nil
	}
	var tasks []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(stdout.buf.Bytes(), &tasks); err != nil {
		return false, err
	}
	return slices.ContainsFunc(tasks, func(t struct {
		Name string `json:"name"`
	}) bool {
		return t.Name == setupTask
	}), nil
}

// failure says how a mise call ended, in the words of the warning: "exit
// <n>", "ended by signal <n>", "ran past its limit of <d>" for a timeout.
func failure(err error) string {
	if timeout, ok := errors.AsType[*procgroup.TimeoutError](err); ok {
		return fmt.Sprintf("ran past its limit of %s (timeout)", timeout.Limit)
	}
	if exit, ok := errors.AsType[*procgroup.ExitError](err); ok {
		if exit.Signal != 0 {
			return fmt.Sprintf("ended by signal %d", int(exit.Signal))
		}
		return fmt.Sprintf("exit %d", exit.Code)
	}
	return termsafe.Escape(err.Error())
}
