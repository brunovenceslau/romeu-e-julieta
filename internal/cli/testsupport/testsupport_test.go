// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package testsupport_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
)

// recorder is a testing.TB that records a failure instead of failing
// the test that runs it, so the helper's own failures can be asserted.
type recorder struct {
	testing.TB
	failed bool
}

func (r *recorder) Helper()               {}
func (r *recorder) Errorf(string, ...any) { r.failed = true }
func (r *recorder) FailNow()              { r.failed = true; panic(r) }

// requireIDFails reports whether RequireID fails for want and err; the
// methods the recorder does not replace go to t.
func requireIDFails(t *testing.T, want string, err error) (failed bool) {
	t.Helper()
	r := &recorder{TB: t}
	defer func() {
		if p := recover(); p != nil && p != r {
			panic(p)
		}
		failed = r.failed
	}()
	testsupport.RequireID(r, want, err)
	return false
}

// TestRequireID passes on the id of the outermost error of the table
// and fails on another id, a chain with no id and a nil error.
func TestRequireID(t *testing.T) {
	usage := fmt.Errorf("wrapped: %w", cli.Usage.New("bad flag"))
	assert.False(t, requireIDFails(t, "RJ-349", usage))
	assert.False(t, requireIDFails(t, "RJ-348", errors.New("no id")))
	assert.True(t, requireIDFails(t, "RJ-348", usage))
	assert.True(t, requireIDFails(t, "RJ-349", errors.New("no id")))
	assert.True(t, requireIDFails(t, "RJ-349", nil))
}
