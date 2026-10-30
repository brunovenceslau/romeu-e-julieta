// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// The results of a check (04 4.2, REQ-0331).
const (
	resultPass  = "pass"
	resultWarn  = "warn"
	resultFail  = "fail"
	resultError = "error"
)

// row is one check of 04 4.2, "Checks run by romeu doctor": its slug,
// a row of the table of 04 4.1 (REQ-0365), and the function that runs
// it over what the host holds.
type row struct {
	slug string
	run  func(ctx context.Context, h *host) Check
}

// rows is the registry of the checks, an unordered set (rule 14 of
// docs/plan.md, the seam of T049): each check registers itself from its
// own file with var _ = register(row{...}), and the tasks that add one
// add a file: the signing row with T080 (signing.go), the breadth of
// the table with T134. It is written during initialization, kept in
// slug order, and read-only after.
var rows []row

// register adds r to the rows and returns it. A slug registered already
// is a programming error.
func register(r row) row {
	if slices.ContainsFunc(rows, func(o row) bool { return o.slug == r.slug }) {
		panic(fmt.Sprintf("doctor: the check %s is registered twice", r.slug))
	}
	rows = append(rows, r)
	slices.SortFunc(rows, func(a, b row) int { return cmp.Compare(a.slug, b.slug) })
	return r
}

// pass returns the passing result of slug, with what it found.
func pass(slug, message string) Check {
	return Check{Slug: slug, Result: resultPass, Message: message}
}

// warn returns the warning result of slug.
func warn(slug, message string) Check {
	return Check{Slug: slug, Result: resultWarn, Message: message}
}

// fail returns the failing result of slug: the check ran and found what
// it checks for.
func fail(slug, message string) Check {
	return Check{Slug: slug, Result: resultFail, Message: message}
}

// failOf returns the failing result of slug from the error of its row
// that a shared check function returned.
func failOf(slug string, err error) Check {
	return fail(slug, err.Error())
}

// errorOf returns the result of slug when the check could not run
// because of err: the error's own id and slug, or doctor-incomplete's
// for an error that carries none of the table.
func errorOf(slug string, err error) Check {
	c := Check{Slug: slug, Result: resultError, ID: DoctorIncomplete.ID, Error: DoctorIncomplete.Slug, Message: err.Error()}
	if e, ok := errors.AsType[*cli.Error](err); ok && e != nil {
		c.ID, c.Error = e.ID, e.Slug
	}
	return c
}

// joined returns the items as one message, in their order.
func joined(items []string) string { return strings.Join(items, "; ") }
