// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	// The assembled command sets of both binaries, so the tests over the
	// registered definitions (the lock field, the command lines of the
	// hints and of the examples) read every real command; until the first
	// command lands they read the fixture alone.
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// fixedNow is the clock of the fixture command.
var fixedNow = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

// failing is a row the fixture command raises; it is no row of 04 4.4,
// so it is declared here and not in an errors.go.
var failing = cli.Row{ID: "RJ-999", Slug: "fixture-failed", Exit: 4, Fix: "decide: <what>", RecoveredIn: "none: a fixture"}

// The fixture command of the package's tests: it prints the clock, and
// with --fail returns an error of the table wrapped twice, with
// --plain an error with no id.
var _ = cli.Register(func() cli.Command {
	var fail, plain bool
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.Summary, c.Usage, c.JSON = "romeu", "fixture", "a command of the tests", "[--fail] [--plain]", true
	c.Examples = []string{"romeu fixture --fail"}
	c.Flags = func(fs *flag.FlagSet) {
		fs.BoolVar(&fail, "fail", false, "fail with a fixture error")
		fs.BoolVar(&plain, "plain", false, "fail with an error with no id")
	}
	c.Run = func(_ context.Context, env *cli.Env, args []string) error {
		switch {
		case fail:
			e := failing.New("it failed", cli.Detail{Key: "step", Value: "2"})
			e.Fill = map[string]string{"<what>": "retry"}
			return fmt.Errorf("outer: %w", e)
		case plain:
			return errors.New("no id")
		}
		_, err := fmt.Fprintf(env.Stdout, "now %s args %s\n", env.Now().Format(time.RFC3339), strings.Join(args, ","))
		return err
	}
	return c
}())

// run runs Main for romeu with args and returns the exit status, stdout
// and stderr.
func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), Now: func() time.Time { return fixedNow }}
	code := cli.Main(t.Context(), args, env)
	return code, stdout.String(), stderr.String()
}

// TestMainRunsTheFixture runs the fixture command through Main with a
// fixed clock.
func TestMainRunsTheFixture(t *testing.T) {
	code, stdout, stderr := run(t, "fixture", "a", "b")
	assert.Equal(t, 0, code)
	assert.Equal(t, "now 2026-10-17T12:00:00Z args a,b\n", stdout)
	assert.Empty(t, stderr)
}

// TestMainReportsAnError prints the outermost error of the table in the
// text form and exits with its row's code.
func TestMainReportsAnError(t *testing.T) {
	code, stdout, stderr := run(t, "fixture", "--fail")
	assert.Equal(t, 4, code)
	assert.Empty(t, stdout)
	assert.Equal(t, "error RJ-999 fixture-failed: it failed\nfix: decide: retry\nstep: 2\n", stderr)
}

// TestMainReportsJSON prints the --json form of the error: one object
// with id, slug, exit, message, fix and the details object.
func TestMainReportsJSON(t *testing.T) {
	code, _, stderr := run(t, "fixture", "--json", "--fail")
	assert.Equal(t, 4, code)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &got))
	assert.Equal(t, map[string]any{
		"id": "RJ-999", "slug": "fixture-failed", "exit": float64(4), "message": "it failed",
		"fix": "decide: retry", "details": map[string]any{"step": "2"},
	}, got)
}

// TestMainInternal prints an error with no id as the error internal
// (RJ-348), exit 1.
func TestMainInternal(t *testing.T) {
	code, _, stderr := run(t, "fixture", "--plain")
	assert.Equal(t, 1, code)
	assert.True(t, strings.HasPrefix(stderr, "error RJ-348 internal: "), stderr)
}

// TestMainUsage exits 2 with the error usage (RJ-349) for no command, an
// unknown command and an unknown flag, with the hint filled.
func TestMainUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
		fix  string
	}{
		{"no command", nil, "fix: romeu --help\n"},
		{"an unknown command", []string{"nosuch"}, "fix: romeu --help\n"},
		{"an unknown flag", []string{"fixture", "--nosuch"}, "fix: romeu fixture --help\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, stderr := run(t, tt.args...)
			assert.Equal(t, 2, code)
			assert.True(t, strings.HasPrefix(stderr, "error RJ-349 usage: "), stderr)
			assert.Contains(t, stderr, tt.fix)
		})
	}
}

// TestMainHelp prints the help of the binary and of a command to stdout,
// each with the exit codes, and exits 0.
func TestMainHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"fixture", "--help"}} {
		code, stdout, stderr := run(t, args...)
		assert.Equal(t, 0, code)
		assert.Empty(t, stderr)
		assert.Contains(t, stdout, "fixture")
		assert.Contains(t, stdout, "143  ended by SIGTERM")
	}
}

// TestErrorIDs asserts ids through the one helper: the outermost error
// of the table, a wrapper that keeps the inner id, and the error
// internal for a chain with none.
func TestErrorIDs(t *testing.T) {
	inner := cli.Usage.New("inner")
	outer := cli.Internal.Wrap(inner, "outer")
	testsupport.RequireID(t, "RJ-349", inner)
	testsupport.RequireID(t, "RJ-349", fmt.Errorf("context: %w", inner))
	testsupport.RequireID(t, "RJ-348", outer)
	testsupport.RequireID(t, "RJ-348", errors.New("no id"))
	assert.Equal(t, 0, cli.ExitCode(nil))
	assert.Equal(t, 2, cli.ExitCode(fmt.Errorf("w: %w", inner)))
}

// TestWarn prints a warning by its slug, escaped by the CLI writer.
func TestWarn(t *testing.T) {
	var out bytes.Buffer
	cli.Warn(cli.NewWriter(&out), "sbx-untested", "sbx 9.9\u202e is newer than the tested range")
	assert.Equal(t, "warning sbx-untested: sbx 9.9\\u202e is newer than the tested range\n", out.String())
}

// TestReportKeepsEachValueOnOneLine writes a line feed in the message, a
// detail key and a detail value, and in a warning, as \n, so no value
// starts a line that forges an error, a fix or a CI annotation.
func TestReportKeepsEachValueOnOneLine(t *testing.T) {
	var stderr bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stderr: cli.NewWriter(&stderr)}
	e := failing.New("x\n::error::forged", cli.Detail{Key: "k\nfix: y", Value: "v\nerror RJ-000 ok: z"})
	assert.Equal(t, 4, cli.Report(env, e))
	assert.Equal(t, "error RJ-999 fixture-failed: x\\n::error::forged\nfix: decide: <what>\nk\\nfix: y: v\\nerror RJ-000 ok: z\n", stderr.String())
	stderr.Reset()
	cli.Warn(env.Stderr, "sbx-untested", "a\n::warning::b")
	assert.Equal(t, "warning sbx-untested: a\\n::warning::b\n", stderr.String())
}

// TestReportJSONSurvivesTheWriter writes a message with a line feed and
// U+202E in the --json form through the CLI writer: the output is one
// line, holds neither raw, and decodes to the message as it was.
func TestReportJSONSurvivesTheWriter(t *testing.T) {
	var stderr bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stderr: cli.NewWriter(&stderr), JSON: true}
	msg := "a\nb\u202ec"
	assert.Equal(t, 4, cli.Report(env, failing.New(msg)))
	out := stderr.String()
	assert.Equal(t, 1, strings.Count(out, "\n"), out)
	assert.NotContains(t, out, "\u202e")
	var got struct {
		Message string            `json:"message"`
		Details map[string]string `json:"details"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, msg, got.Message)
	assert.NotNil(t, got.Details, "details is an object, not null")
}

// The fixture command with subcommands: julieta fixturegroup one, which
// takes --repo with a default, and julieta fixturegroup two.
var (
	_ = cli.Register(func() cli.Command {
		var repo string
		c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
		c.Binary, c.Name, c.Sub, c.Summary, c.Usage = "julieta", "fixturegroup", "one", "the first subcommand", "[--repo <dir>] [args]"
		c.Examples = []string{"julieta fixturegroup one --repo /x"}
		c.Flags = func(fs *flag.FlagSet) {
			fs.StringVar(&repo, "repo", "/work", "the `dir` of the repository")
			fs.Int("count", 3, "how many")
			fs.String("mode", "0", "the mode")
			fs.Duration("wait", 0, "how long")
			fs.Bool("verbose", false, "say more")
		}
		c.Run = func(_ context.Context, env *cli.Env, args []string) error {
			_, err := fmt.Fprintf(env.Stdout, "one repo %s args %s\n", repo, strings.Join(args, ","))
			return err
		}
		return c
	}())
	_ = cli.Register(func() cli.Command {
		c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
		c.Binary, c.Name, c.Sub, c.Summary = "julieta", "fixturegroup", "two", "the second subcommand"
		c.Run = func(_ context.Context, env *cli.Env, _ []string) error {
			_, err := fmt.Fprintln(env.Stdout, "two")
			return err
		}
		return c
	}())
)

// runJulieta runs Main for julieta with args.
func runJulieta(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), Now: func() time.Time { return fixedNow }}
	return cli.Main(t.Context(), args, env), stdout.String(), stderr.String()
}

// TestMainSubcommands dispatches a command's subcommand, with its own
// flags, and exits 2 with the error usage when the subcommand is missing
// or unknown, the hint naming the command's help.
func TestMainSubcommands(t *testing.T) {
	code, stdout, _ := runJulieta(t, "fixturegroup", "one", "--repo", "/r", "a")
	assert.Equal(t, 0, code)
	assert.Equal(t, "one repo /r args a\n", stdout)
	code, stdout, _ = runJulieta(t, "fixturegroup", "two")
	assert.Equal(t, 0, code)
	assert.Equal(t, "two\n", stdout)
	for _, args := range [][]string{{"fixturegroup"}, {"fixturegroup", "three"}} {
		code, _, stderr := runJulieta(t, args...)
		assert.Equal(t, 2, code, "%v", args)
		assert.True(t, strings.HasPrefix(stderr, "error RJ-349 usage: "), stderr)
		assert.Contains(t, stderr, "fix: julieta fixturegroup --help\n")
	}
	_, _, stderr := runJulieta(t, "fixturegroup", "one", "--nosuch")
	assert.Contains(t, stderr, "fix: julieta fixturegroup one --help\n")
	// A flag before the subcommand, --json among them, is a usage error
	// that says where the flags go.
	for _, args := range [][]string{{"fixturegroup", "--json"}, {"fixturegroup", "--repo", "x", "one"}} {
		code, _, stderr := runJulieta(t, args...)
		assert.Equal(t, 2, code, "%v", args)
		assert.Contains(t, stderr, "go after its subcommand", "%v", args)
	}
}

// TestMainHelpOnEveryLevel prints the help of the binary, of a command
// with subcommands and of a subcommand, whose flag line names its value
// and its default.
func TestMainHelpOnEveryLevel(t *testing.T) {
	code, stdout, _ := runJulieta(t, "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "  fixturegroup one ")
	for _, help := range []string{"--help", "-h", "help"} {
		code, stdout, _ = runJulieta(t, "fixturegroup", help)
		assert.Equal(t, 0, code, help)
		assert.Contains(t, stdout, "Usage: julieta fixturegroup <subcommand> [flags] [args]\n\nSubcommands:\n  one  the first subcommand\n  two  the second subcommand\n", help)
	}
	code, stdout, _ = runJulieta(t, "fixturegroup", "one", "-h")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "Usage: julieta fixturegroup one [--repo <dir>] [args]\n")
	for _, line := range []string{
		"  --repo <dir>  the dir of the repository (default \"/work\")\n",
		"  --count <int>  how many (default 3)\n",
		"  --mode <string>  the mode (default \"0\")\n",
		"  --wait <duration>  how long\n",
		"  --verbose  say more\n",
	} {
		assert.Contains(t, stdout, line)
	}
	assert.Contains(t, stdout, "  --help  print this help\n")
	assert.Contains(t, stdout, "143  ended by SIGTERM")
}
