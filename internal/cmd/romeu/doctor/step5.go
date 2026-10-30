// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/preflight"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// The slugs of the checks marked pre in 04 4.2 that step 5 of the
// preflight holds (internal/preflight/step5.go): the two I22 checks,
// which read sbx, and the three I23 checks, which read the host.
const (
	slugGlobalSecret = "sbx-global-secret"
	slugBroadAllow   = "sbx-broad-allow"
	slugHomeSymlink  = "home-symlink"
	slugAutoTrust    = "auto-trust"
	slugVSCodeTrust  = "vscode-trust"
)

// stepFive is what preflight.HostFindings found: the finding of each
// check that failed, by its slug, and the checks that ran in part.
type stepFive struct {
	found     map[string]error
	unchecked []string
}

// hostFindings runs the host checks of step 5 once and returns what they
// found (REQ-0365: one implementation serves the preflight and doctor).
func (h *host) hostFindings(ctx context.Context) stepFive {
	s, _ := h.findings.get(func() (stepFive, error) {
		in := preflight.Input{
			// No project: doctor checks the host for every project, so
			// the hints keep their placeholders.
			Project:      "<name>",
			Primary:      "<primary>",
			Root:         h.settings.Root,
			Home:         h.home,
			OS:           h.goos,
			Getenv:       h.getenv,
			UsedServices: usedServices(h),
		}
		findings, unchecked := h.preflight(ctx, preflight.Deps{Sbx: preSbx{h: h}}, in)
		s := stepFive{found: map[string]error{}, unchecked: unchecked}
		for _, f := range findings {
			s.found[f.Slug] = f.Err
		}
		return s, nil
	})
	return s
}

// preCheck returns the result of the pre check slug: fail with the error
// of its own row, error with any other error, which kept the check from
// running (an sbx read that failed, upstream-shape), and pass otherwise.
func (h *host) preCheck(ctx context.Context, slug string) Check {
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	err, found := h.hostFindings(ctx).found[slug]
	switch {
	case !found:
		return Check{Slug: slug, Result: resultPass}
	case hasOwnRow(err, slug):
		return failOf(slug, err)
	}
	return errorOf(slug, err)
}

// unplaced returns, as results of the error, the findings of step 5
// whose check has no row registered here, such as a check the preflight
// gained before doctor: a finding is never dropped.
func (h *host) unplaced(ctx context.Context) []Check {
	if h.baseErr != nil {
		return nil
	}
	var out []Check
	s := h.hostFindings(ctx)
	for _, slug := range slices.Sorted(maps.Keys(s.found)) {
		if !slices.ContainsFunc(rows, func(r row) bool { return r.slug == slug }) {
			c := errorOf(slug, s.found[slug])
			c.Message = "the preflight's check " + slug + " has no row in doctor, so its finding is reported as is: " + c.Message
			out = append(out, c)
		}
	}
	return out
}

// hasOwnRow reports whether err is the error of the row slug.
func hasOwnRow(err error, slug string) bool {
	e, ok := errors.AsType[*cli.Error](err)
	return ok && e.Slug == slug
}

// usedServices returns the service names of the secret entries of the
// settings, sorted and unique: a global sbx secret for one of them would
// reach every sandbox (I22). It is never nil.
func usedServices(h *host) []string {
	out := []string{}
	for _, s := range h.settings.Secrets {
		if !slices.Contains(out, s.Service) {
			out = append(out, s.Service)
		}
	}
	slices.Sort(out)
	return out
}

// preSbx is the sbx of the preflight's step 5, over the host's reader:
// each read comes after the floor check, which a call of sbx needs
// first.
type preSbx struct{ h *host }

func (p preSbx) List(ctx context.Context) ([]sbxdrv.Sandbox, error) {
	return p.h.sandboxes(ctx)
}

func (p preSbx) PolicyList(ctx context.Context) ([]byte, error) {
	return p.read(ctx, p.h.sbxr.PolicyList)
}

func (p preSbx) SecretList(ctx context.Context) ([]byte, error) {
	return p.read(ctx, p.h.sbxr.SecretList)
}

// read runs the listing list after the floor check.
func (p preSbx) read(ctx context.Context, list func(context.Context) ([]byte, error)) ([]byte, error) {
	if _, err := p.h.requireFloor(ctx); err != nil {
		return nil, err
	}
	return list(ctx)
}
