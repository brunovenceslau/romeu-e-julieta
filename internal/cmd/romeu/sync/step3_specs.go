// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// projectsDir is the directory of the config repo that holds the specs
// (02 2.2).
const projectsDir = "projects"

// readSpecs is step 3 of 04 4.2 (REQ-0337, REQ-0338): git fsck --strict
// on the spec commit, the files of projects/ read from its tree, and
// every spec decoded and validated together with the host settings,
// every error listed in one exit 2: projects-stray-file (RJ-345) and
// config-project-missing (RJ-346) keep their rows, and the other rules
// are usage. Then it picks the targets, --all or the names given, and
// checks their secret bindings (checkSecrets). It fills specs and
// targets.
func readSpecs(ctx context.Context, r *run) error {
	if err := r.git.Fsck(ctx, r.configClone, r.specCommit); err != nil {
		return refused(err, "spec-commit-unsafe", "git fsck --strict refuses the spec commit "+r.specCommit)
	}
	read, err := r.git.ReadDir(ctx, r.configClone, r.specCommit, projectsDir)
	if err != nil {
		return refused(err, "projects-refused", "the files of "+projectsDir+"/ at the spec commit "+r.specCommit)
	}
	files := make([]spec.File, len(read))
	for i, f := range read {
		files[i] = spec.File{Path: projectsDir + "/" + f.Name, Data: f.Data}
	}
	specs, err := spec.LoadSet(files, &r.settings)
	if err != nil {
		return invalid(err, r.specCommit)
	}
	r.specs = specs
	if r.targets, err = targets(specs, r.flags); err != nil {
		return err
	}
	return checkSecrets(r)
}

// invalid returns the error of LoadSet with its text escaped, since a
// finding quotes the config repo's content, which is untrusted (I29):
// an error of a row (projects-stray-file, config-project-missing) keeps
// its row and details, and the other findings are usage with the slug
// spec-invalid. Either lists every finding.
func invalid(err error, commit string) error {
	text := termsafe.Escape(err.Error())
	if e, ok := errors.AsType[*cli.Error](err); ok {
		out := e.New(text, e.Details...)
		out.Fill = e.Fill
		return out
	}
	return usage("spec-invalid", "the specs at "+commit+" do not validate: "+text)
}

// refused returns usage, with slug and what, for an error of git on
// the config repo's content (a git that exits non-zero, or what gitsafe
// refuses in it), and err as it is otherwise.
func refused(err error, slug, what string) error {
	_, exited := errors.AsType[*procgroup.ExitError](err)
	if !exited && !errors.Is(err, gitsafe.ErrRefused) {
		return err
	}
	return usage(slug, what+": "+termsafe.Escape(err.Error()))
}

// targets returns the specs the command line names, in the order of
// specs: every spec with --all, else each one named, which must exist.
// The shape of the line was checked before step 1 (flags.check).
func targets(specs []spec.Project, f flags) ([]spec.Project, error) {
	if f.all {
		return specs, nil
	}
	var out []spec.Project
	for _, p := range specs {
		if slices.Contains(f.names, p.Name) {
			out = append(out, p)
		}
	}
	for _, n := range f.names {
		if !slices.ContainsFunc(out, func(p spec.Project) bool { return p.Name == n }) {
			return nil, usage("project-unknown", "the spec has no project "+termsafe.Escape(n))
		}
	}
	return out, nil
}

// checkSecrets refuses a target whose spec names a secret that the host
// settings bind to no <secret>@<project> entry of that project exactly
// (I8, REQ-1222): github@other does not satisfy project shop. It is the
// check REQ-0335 places in step 1, run here because it reads the specs,
// which step 3 reads; it runs before step 4, the first write, and on
// the targets alone, since nothing of another project is rendered. Each
// unbound secret is named; the hint names the first.
func checkSecrets(r *run) error {
	var missing []string
	var first [2]string
	for _, p := range r.targets {
		for _, s := range p.Secrets {
			key := s + "@" + p.Name
			if _, ok := r.settings.Secrets[key]; ok {
				continue
			}
			if missing == nil {
				first = [2]string{s, p.Name}
			}
			missing = append(missing, key)
		}
	}
	if missing == nil {
		return nil
	}
	e := SecretUnbound.New("the host settings bind no command to the spec secrets " + strings.Join(missing, ", "))
	e.Fill = map[string]string{"<secret>": first[0], "<name>": first[1]}
	return e
}
