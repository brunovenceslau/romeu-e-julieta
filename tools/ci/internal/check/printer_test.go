// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestChecksPrintErrorsAsSafeText checks that the error of a verdict and
// the error of a step, which may hold a path of the repository, reach
// the log with no raw control character and no line that starts a
// workflow command, while the output of a step is printed as it is.
func TestChecksPrintErrorsAsSafeText(t *testing.T) {
	hostile := errors.New("x\x1b[2J\n\u00a0::error::forged\n::error::two ##[error]")
	var out bytes.Buffer
	c := check.NewPrinter(&out)
	c.Verdict(check.Verdict{Name: "v", Err: hostile}, 0)
	c.Run(t.Context(), t.TempDir(), []check.Step{{Name: "s", Unavailable: hostile}})
	got := out.String()
	assert.NotContains(t, got, "\x1b")
	assert.NotContains(t, got, "##[")
	for _, line := range strings.Split(got, "\n") {
		assert.False(t, strings.HasPrefix(strings.TrimSpace(line), "::"), "%q", line)
	}
	assert.Contains(t, got, `\x1b[2J`)
}

// TestQuietStepPrintsSafeText checks that the output of a quiet step,
// which lists files of the repository, reaches the log with no line that
// starts a workflow command, while the output of another step is
// printed as it is.
func TestQuietStepPrintsSafeText(t *testing.T) {
	hostile := "::error::x.go\n"
	var out bytes.Buffer
	c := check.NewPrinter(&out)
	c.Run(t.Context(), t.TempDir(), []check.Step{
		{Name: "escape", Argv: []string{"sh", "-c", `printf '\033[2J\nx.go\n'; exit 1`}, Quiet: true},
	})
	assert.NotContains(t, out.String(), "\x1b", "a control character of the output is escaped")
	assert.Contains(t, out.String(), `\x1b[2J`)
	out.Reset()
	c.Run(t.Context(), t.TempDir(), []check.Step{
		{Name: "listing", Argv: []string{"sh", "-c", "printf '%s' \"$0\"; exit 1", hostile}, Quiet: true},
		{Name: "code", Argv: []string{"sh", "-c", "printf '%s' \"$0\"; exit 1", hostile}},
	})
	var listing, code []string
	var in *[]string
	for _, line := range strings.Split(out.String(), "\n") {
		switch {
		case strings.HasPrefix(line, "FAIL  listing"):
			in = &listing
		case strings.HasPrefix(line, "FAIL  code"):
			in = &code
		case in != nil:
			*in = append(*in, line)
		}
	}
	for _, line := range listing {
		assert.False(t, strings.HasPrefix(strings.TrimSpace(line), "::"), "%q", line)
	}
	assert.Contains(t, listing, "./::error::x.go")
	assert.Contains(t, code, "::error::x.go", "the output of a step that runs code stays raw")
}

// TestRunReadsTheOutputOfAStep checks the Read hook of a step: what it
// returns is what the verdict prints, its error decides the verdict
// whatever the exit status, and an error that wraps ErrInfrastructure
// marks the step an infrastructure error. InfraOnly holds when every
// failed check is one.
func TestRunReadsTheOutputOfAStep(t *testing.T) {
	infra := func([]byte, error) ([]byte, error) {
		return []byte("read\n"), fmt.Errorf("the pull: %w", check.ErrInfrastructure)
	}
	pass := func([]byte, error) ([]byte, error) { return nil, nil }
	fail := func([]byte, error) ([]byte, error) { return []byte("no test ran\n"), errors.New("zero tests") }
	var out bytes.Buffer
	c := check.NewPrinter(&out)
	c.Run(t.Context(), t.TempDir(), []check.Step{
		{Name: "passes", Argv: []string{"sh", "-c", "exit 1"}, Read: pass},
		{Name: "e2e", Argv: []string{"sh", "-c", "echo raw; exit 1"}, Read: infra},
	})
	assert.Contains(t, out.String(), "ok    passes")
	assert.Contains(t, out.String(), "FAIL  e2e")
	assert.Contains(t, out.String(), "read\n")
	assert.NotContains(t, out.String(), "raw")
	assert.Equal(t, []string{"e2e"}, c.Infra)
	assert.True(t, c.InfraOnly())

	c.Run(t.Context(), t.TempDir(), []check.Step{{Name: "zero", Argv: []string{"true"}, Read: fail}})
	assert.Contains(t, out.String(), "FAIL  zero")
	assert.Contains(t, out.String(), "no test ran")
	assert.False(t, c.InfraOnly(), "a test failure besides the infrastructure error")
	assert.False(t, check.NewPrinter(&out).InfraOnly(), "nothing failed")
}

// TestFormatDuration holds the rounding of a duration on a verdict line
// to 100 ms, on both sides of the boundaries.
func TestFormatDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                       "0s",
		49 * time.Millisecond:   "0s",
		50 * time.Millisecond:   "100ms",
		1240 * time.Millisecond: "1.2s",
		1250 * time.Millisecond: "1.3s",
		3*time.Minute + 30*time.Second + 40*time.Millisecond: "3m30s",
	} {
		assert.Equal(t, want, check.FormatDuration(d), "%s", d)
	}
}

// TestLastNamesEachFailedCheckOnce holds the last line of a failed run.
func TestLastNamesEachFailedCheckOnce(t *testing.T) {
	var out bytes.Buffer
	c := check.NewPrinter(&out)
	c.Last("fast")
	assert.Empty(t, out.String(), "nothing failed, nothing to name")

	c.Report("a", true, "", 0)
	c.Report("b", false, "", 0)
	c.Report("c", false, "", 0)
	c.Report("b", false, "", 0)
	out.Reset()
	c.Last("all")
	assert.Equal(t, "all: failed: b, c\n", out.String())
}
