// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"
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

// stepFive is what preflight.HostFindings found, by the slug of the
// check each finding belongs to; a slug with no entry passed.
type stepFive struct {
	found     map[string]error
	unchecked []string
}

// preCheck returns the result of the pre check slug: the functions of
// T046, called once for the five checks (REQ-0365, one implementation
// serves both).
func (h *host) preCheck(ctx context.Context, slug string) Check {
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	s, _ := h.findings.get(func() (stepFive, error) { return h.stepFive(ctx), nil })
	err, found := s.found[slug]
	switch {
	case !found:
		return Check{Slug: slug, Result: resultPass}
	case errors.As(err, new(*listingError)), isUpstreamShape(err), !hasOwnRow(err, slug):
		return errorOf(slug, err)
	}
	return failOf(slug, err)
}

// stepFive runs the host checks of step 5 and sorts their findings by
// check. preflight.HostFindings returns the errors of the checks that
// failed, in their order, but not which check each came from, so an
// error that carries no row of the five is placed by this rule:
//   - an error of the five rows is its row's;
//   - the failure of an sbx listing doctor ran is the row that reads it;
//   - upstream-shape comes from a parser of the two I22 listings, which
//     no I23 check reads: two of them are the secret row's then the
//     policy row's, in step 5's order, and one is the row of the two
//     that has no finding of its own, or both when neither has one, its
//     message naming the listing;
//   - any other error is each I23 row's that has no finding of its own.
//
// A finding placed on a row it may not come from is an error of that
// row, never a pass, so the report errs toward incomplete.
func (h *host) stepFive(ctx context.Context) stepFive {
	d := preflight.Deps{Sbx: preSbx{h: h}}
	in := preflight.Input{
		// No project: doctor checks the host for every project, so the
		// hints keep their placeholders.
		Project:      "<name>",
		Primary:      "<primary>",
		Root:         h.settings.Root,
		Home:         h.home,
		OS:           h.goos,
		Getenv:       h.getenv,
		UsedServices: usedServices(h),
	}
	findings, unchecked := preflight.HostFindings(ctx, d, in)
	s := stepFive{found: map[string]error{}, unchecked: unchecked}
	set := func(slug string, err error) {
		if _, ok := s.found[slug]; !ok {
			s.found[slug] = err
		}
	}
	var shapes, others []error
	for _, f := range findings {
		if le, ok := errors.AsType[*listingError](f); ok {
			set(le.slug, f)
			continue
		}
		if e, ok := errors.AsType[*cli.Error](f); ok && slices.Contains(preSlugs, e.Slug) {
			set(e.Slug, f)
			continue
		}
		if isUpstreamShape(f) {
			shapes = append(shapes, f)
			continue
		}
		others = append(others, f)
	}
	i22 := []string{slugGlobalSecret, slugBroadAllow}
	if len(shapes) == 2 {
		for i, slug := range i22 {
			set(slug, shapes[i])
		}
		shapes = nil
	}
	for _, f := range shapes {
		for _, slug := range i22 {
			set(slug, f)
		}
	}
	for _, f := range others {
		for _, slug := range []string{slugHomeSymlink, slugAutoTrust, slugVSCodeTrust} {
			set(slug, f)
		}
	}
	return s
}

// preSlugs are the five slugs of step 5.
var preSlugs = []string{slugGlobalSecret, slugBroadAllow, slugHomeSymlink, slugAutoTrust, slugVSCodeTrust}

// isUpstreamShape reports whether err is upstream-shape (RJ-310).
func isUpstreamShape(err error) bool {
	e, ok := errors.AsType[*cli.Error](err)
	return ok && e.ID == cli.UpstreamShape.ID
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

// listingError is the failure of an sbx read doctor ran for step 5, with
// the slug of the check that reads it.
type listingError struct {
	slug string
	err  error
}

func (e *listingError) Error() string { return e.err.Error() }
func (e *listingError) Unwrap() error { return e.err }

// preSbx is the sbx of the preflight's step 5, over the host's reader:
// each read comes after the floor check, which a call of sbx needs
// first, and its failure is marked with the check that reads it.
type preSbx struct{ h *host }

func (p preSbx) List(ctx context.Context) ([]sbxdrv.Sandbox, error) {
	return p.h.sandboxes(ctx)
}

func (p preSbx) PolicyList(ctx context.Context) ([]byte, error) {
	return p.read(ctx, slugBroadAllow, p.h.sbxr.PolicyList)
}

func (p preSbx) SecretList(ctx context.Context) ([]byte, error) {
	return p.read(ctx, slugGlobalSecret, p.h.sbxr.SecretList)
}

// read runs the listing of the check slug after the floor check.
func (p preSbx) read(ctx context.Context, slug string, list func(context.Context) ([]byte, error)) ([]byte, error) {
	if _, err := p.h.requireFloor(ctx); err != nil {
		return nil, &listingError{slug: slug, err: err}
	}
	out, err := list(ctx)
	if err != nil {
		return nil, &listingError{slug: slug, err: err}
	}
	return out, nil
}
