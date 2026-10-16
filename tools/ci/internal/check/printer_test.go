// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/stretchr/testify/assert"
)

// TestChecksPrintErrorsAsSafeText checks that the error of a verdict and
// the error of a step, which may hold a path of the repository, reach
// the log with no raw control character and no line that starts a
// workflow command, while the output of a step is printed as it is.
func TestChecksPrintErrorsAsSafeText(t *testing.T) {
	hostile := errors.New("x\x1b[2J\n\u00a0::error::forged\n::error::two ##[error]")
	var out bytes.Buffer
	c := check.NewPrinter(&out)
	c.Verdict(check.Verdict{Name: "v", Err: hostile})
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
