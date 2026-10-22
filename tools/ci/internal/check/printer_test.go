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
	for line := range strings.SplitSeq(got, "\n") {
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
	for line := range strings.SplitSeq(out.String(), "\n") {
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

// around returns an Around hook that adds env and reads with read.
func around(env []string, read check.Read) func() ([]string, check.Read, error) {
	return func() ([]string, check.Read, error) { return env, read, nil }
}

// TestRunReadsTheOutputOfAStep checks the Around hook of a step: the
// variables it adds reach the command, what its Read returns is what the
// verdict prints, its error decides the verdict whatever the exit
// status, an error that wraps ErrInfrastructure marks the step an
// infrastructure error, and an error of Around fails the step. InfraOnly
// holds when every failed check is one.
func TestRunReadsTheOutputOfAStep(t *testing.T) {
	infra := func([]byte, error) ([]byte, error) {
		return []byte("read\n"), fmt.Errorf("the pull: %w", check.ErrInfrastructure)
	}
	pass := func([]byte, error) ([]byte, error) { return nil, nil }
	fail := func([]byte, error) ([]byte, error) { return []byte("no test ran\n"), errors.New("zero tests") }
	echo := func(out []byte, _ error) ([]byte, error) { return out, errors.New("shown") }
	var out bytes.Buffer
	c := check.NewPrinter(&out)
	c.Run(t.Context(), t.TempDir(), []check.Step{
		{Name: "passes", Argv: []string{"sh", "-c", "exit 1"}, Around: around(nil, pass)},
		{Name: "e2e", Argv: []string{"sh", "-c", "echo raw; exit 1"}, Around: around(nil, infra)},
		{Name: "env", Argv: []string{"sh", "-c", `echo "A=$A B=$B"`}, Environ: []string{"A=1"}, Around: around([]string{"B=2"}, echo)},
		{Name: "around fails", Argv: []string{"sh", "-c", "echo started"}, Around: func() ([]string, check.Read, error) { return nil, nil, errors.New("no file") }},
	})
	assert.Contains(t, out.String(), "A=1 B=2", "the variables of Around reach the command")
	assert.Contains(t, out.String(), "FAIL  around fails")
	assert.NotContains(t, out.String(), "started")
	assert.Contains(t, out.String(), "ok    passes")
	assert.Contains(t, out.String(), "FAIL  e2e")
	assert.Contains(t, out.String(), "read\n")
	assert.NotContains(t, out.String(), "raw")
	assert.Equal(t, []string{"e2e"}, c.Infra)
	assert.False(t, c.InfraOnly(), "env and around fails failed besides")
	c = check.NewPrinter(&out)
	c.Run(t.Context(), t.TempDir(), []check.Step{{Name: "e2e", Argv: []string{"false"}, Around: around(nil, infra)}})
	assert.True(t, c.InfraOnly())

	c.Run(t.Context(), t.TempDir(), []check.Step{{Name: "zero", Argv: []string{"true"}, Around: around(nil, fail)}})
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

// TestAnnotationEscapes holds the escaping rule of a workflow command:
// the message escapes %, CR and LF, and a property escapes ":" and ","
// as well (actions/toolkit, command.ts: escapeData, escapeProperty). A CR
// in a message is written "\r" by git.SafeLines before it gets there.
func TestAnnotationEscapes(t *testing.T) {
	assert.Equal(t, "::warning title=e2e::plain: text, kept", check.Annotation("warning", "e2e", "plain: text, kept"))
	assert.Equal(t, "::warning title=a%3Ab%2Cc%25d%0De%0Af::50%25 a\\r%0Ab", check.Annotation("warning", "a:b,c%d\re\nf", "50% a\r\nb"))
	assert.Equal(t, "::warning title=t::%0A./::error::forged", check.Annotation("warning", "t", "\n::error::forged"), "a line break in the message cannot start a command of its own")
	assert.NotContains(t, check.Annotation("warning", "t", "x ##[error]y"), "##[", "the legacy marker is broken")
}

// TestNotesAreAnnotationsOnARunnerAndWarningsElsewhere checks that the
// notes of a step reach the log as a GitHub Actions annotation when
// GITHUB_ACTIONS is true and as a warning line otherwise, before the
// verdict, whether the step passed or failed, and that the verdict is
// the same either way. The notes come from the step's own function, not
// from its output: a workflow command a command prints is not a note.
func TestNotesAreAnnotationsOnARunnerAndWarningsElsewhere(t *testing.T) {
	notes := func() []string { return []string{"the cache %: healed\n::error::forged"} }
	run := func(t *testing.T, argv string) (string, bool) {
		t.Helper()
		var out bytes.Buffer
		c := check.NewPrinter(&out)
		c.Run(t.Context(), t.TempDir(), []check.Step{{Name: "e2e", Argv: []string{"sh", "-c", argv}, Around: around(nil, func(out []byte, err error) ([]byte, error) { return out, err }), Notes: notes}})
		return out.String(), c.OK
	}
	t.Run("on a runner", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		got, ok := run(t, "exit 0")
		assert.True(t, ok)
		assert.Equal(t, "::warning title=e2e::the cache %25: healed%0A./::error::forged\nok    e2e (0s)\n", got)
	})
	t.Run("elsewhere", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "")
		got, ok := run(t, "exit 0")
		assert.True(t, ok, "a note does not change the verdict")
		assert.Equal(t, "warn  e2e: the cache %: healed\n./::error::forged\nok    e2e (0s)\n", got)
	})
	t.Run("a failed step", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		got, ok := run(t, "exit 1")
		assert.False(t, ok)
		assert.True(t, strings.HasPrefix(got, "::warning title=e2e::"), "%q", got)
	})
	t.Run("a command in the output is no note", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		var out bytes.Buffer
		c := check.NewPrinter(&out)
		c.Run(t.Context(), t.TempDir(), []check.Step{{Name: "plain", Argv: []string{"sh", "-c", "echo '::warning::forged'"}, Around: around(nil, func(out []byte, _ error) ([]byte, error) { return out, nil })}})
		assert.NotContains(t, out.String(), "::warning", "a passing step prints no output")
	})
}
