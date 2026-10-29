// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// reservedDirs are the names romeu reserves inside <name>-env/ that are
// directories (03 3.1, the row of a repo dir, REQ-0176), compared
// case-insensitively: none is a repo dir, so none is orphaned. The
// other reserved names, .romeu, .git and sbxenv.yaml, start with a dot
// or are a file, and the walk skips those already.
var reservedDirs = []string{memoryDir, "review", "ledger"}

// reportOrphans is step 10 of 04 4.2 (REQ-0345, REQ-0334): it reports
// on stdout, and removes nothing, each <name>-env/ under the root that
// no spec of the config repo names, as "orphaned: <name>", and, in the
// env dir of each project a spec names, each repo dir its spec does
// not name, a host clone or a memory dir, as "orphaned repo dir:
// <name>-env/<path>" (01 1.7; J7, J13). It reads every spec, not the
// targets alone, and judges each entry by Lstat, so a link is no dir
// of either kind.
//
// The warning julieta-pin-mismatch, which REQ-0345 also places here,
// needs the pin of the config repo's validate.yml, whose format and
// reader julieta pin check (T055) defines, after this task.
func reportOrphans(_ context.Context, r *run) error {
	names, err := envDirs(r.settings.Root)
	if err != nil {
		return fmt.Errorf("the orphans of the root: %w", err)
	}
	for _, name := range names {
		i := slices.IndexFunc(r.specs, func(p spec.Project) bool { return p.Name == name })
		if i < 0 {
			if _, err := fmt.Fprintf(r.env.Stdout, "orphaned: %s\n", termsafe.Escape(name)); err != nil {
				return err
			}
			continue
		}
		env := name + "-env"
		dirs, err := orphanedDirs(filepath.Join(r.settings.Root, env), r.specs[i])
		if err != nil {
			return fmt.Errorf("the orphans of %s: %w", termsafe.Escape(env), err)
		}
		for _, d := range dirs {
			if _, err := fmt.Fprintf(r.env.Stdout, "orphaned repo dir: %s\n", termsafe.Escape(path.Join(env, d))); err != nil {
				return err
			}
		}
	}
	return nil
}

// orphanedDirs returns the repo dirs of the env dir env that the spec
// p does not name, in name order: each directory directly under env,
// and each one under its memory/, but the project's shared memory dir.
// A name that starts with a dot is no repo dir (03 3.1).
func orphanedDirs(env string, p spec.Project) ([]string, error) {
	named := func(dir string) bool {
		return slices.ContainsFunc(p.Repos, func(repo spec.Repo) bool { return repo.Dir == dir })
	}
	var out []string
	clones, err := subdirs(env)
	if err != nil {
		return nil, err
	}
	for _, d := range clones {
		reserved := slices.ContainsFunc(reservedDirs, func(n string) bool { return strings.EqualFold(n, d) })
		if !reserved && !named(d) {
			out = append(out, d)
		}
	}
	memories, err := subdirs(filepath.Join(env, memoryDir))
	if err != nil {
		return nil, err
	}
	for _, d := range memories {
		if d != sharedDir && !named(d) {
			out = append(out, path.Join(memoryDir, d))
		}
	}
	return out, nil
}

// subdirs returns the names of the directories directly under dir that
// do not start with a dot, links excluded, in name order; none when dir
// does not exist or is not a plain directory, a link among them, which
// is not read through (I12).
func subdirs(dir string) ([]string, error) {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) || err == nil && !info.IsDir() {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out, nil
}
