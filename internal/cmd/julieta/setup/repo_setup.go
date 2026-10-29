// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

// target returns the mise target of the repo with the dir dir.
func target(r *run, dir string) mise.Target {
	return mise.Target{Dir: repoPath(r, dir), Repo: dir, Sandbox: r.m.Project, Egress: r.m.Egress}
}

// trustRepos is the step trust of setup (04 4.3, REQ-1306): mise trust
// on the mise.toml of each repo that has one, before install, in the
// sandbox only. A failure is reported and leaves the sandbox usable.
func trustRepos(ctx context.Context, r *run) {
	dirs := miseRepos(r)
	if len(dirs) == 0 {
		return
	}
	d, err := mise.New(os.LookupEnv, r.home, repoPaths(r.m))
	if err != nil {
		r.report.Errors = append(r.report.Errors, "trust: "+Describe(err))
		return
	}
	for _, dir := range dirs {
		if err := d.Trust(ctx, target(r, dir)); err != nil {
			r.report.Errors = append(r.report.Errors, "trust: "+Describe(err))
		}
	}
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
	d, err := mise.New(os.LookupEnv, r.home, repoPaths(r.m))
	if err != nil {
		r.report.Errors = append(r.report.Errors, "repo setup: "+Describe(err))
		return
	}
	for _, dir := range dirs {
		t := target(r, dir)
		defined, err := d.HasTask(ctx, t, setupTask)
		if err != nil {
			r.warn(RepoSetupFailed, fmt.Sprintf("%s the tasks cannot be listed: %s", termsafe.Escape(dir), failure(err)))
			continue
		}
		if !defined {
			continue
		}
		if err := d.RunTask(ctx, t, setupTask); err != nil {
			r.warn(RepoSetupFailed, fmt.Sprintf("%s %s", termsafe.Escape(dir), failure(err)))
		}
	}
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
