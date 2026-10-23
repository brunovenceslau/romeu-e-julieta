// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli_test

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
	// The assembled command sets of both binaries, so the tests over the
	// registered definitions (the lock field, the command lines of the
	// hints and of the examples) read every real command; until the first
	// command lands they read the fixtures alone.
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// fixedNow is the clock of the fixture command.
var fixedNow = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

// failing is a row the fixture command raises; it is no row of 04 4.4,
// so it is declared here and not in an errors.go.
var failing = cli.Row{ID: "RJ-999", Slug: "fixture-failed", Exit: 4, Fix: "decide: <what>", RecoveredIn: "none: a fixture"}

// The fixture command of the package's tests, romeu fixture: it prints
// the clock and its arguments; with --fail it returns an error of the
// table wrapped twice, whose detail is --detail, with --plain an error
// with no id, with --context the error of its context, wrapped, and
// with both an error of the table joined to it.
var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.JSON = "romeu", "fixture", true
	c.New = func(env *cli.Env) *cobra.Command {
		var fail, plain, withContext bool
		var detail string
		cmd := &cobra.Command{
			Use:     "fixture [args]",
			Short:   "a command of the tests",
			Example: "romeu fixture --fail\nromeu fixture --detail=x a",
			Args:    cobra.ArbitraryArgs,
			RunE: func(c *cobra.Command, args []string) error {
				switch {
				case withContext && fail:
					return errors.Join(failing.New("it failed"), c.Context().Err())
				case withContext:
					return fmt.Errorf("stopped: %w", cmp.Or(c.Context().Err(), context.Canceled))
				case fail:
					e := failing.New("it failed", cli.Detail{Key: "step", Value: detail})
					e.Fill = map[string]string{"<what>": "retry"}
					return fmt.Errorf("outer: %w", e)
				case plain:
					return errors.New("no id")
				}
				_, err := fmt.Fprintf(env.Stdout, "now %s args %s\n", env.Now().Format(time.RFC3339), strings.Join(args, ","))
				return err
			},
		}
		cmd.Flags().BoolVar(&fail, "fail", false, "fail with a fixture error")
		cmd.Flags().BoolVar(&plain, "plain", false, "fail with an error with no id")
		cmd.Flags().BoolVar(&withContext, "context", false, "fail with the error of the context")
		cmd.Flags().StringVarP(&detail, "detail", "d", "2", "the `value` of the detail")
		return cmd
	}
	return c
}())

// The fixture command with subcommands: julieta fixturegroup one, which
// takes --repo with a default and one argument at most, and julieta
// fixturegroup two.
var (
	_ = cli.Register(func() cli.Command {
		c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
		c.Binary, c.Name, c.Sub = "julieta", "fixturegroup", "one"
		c.New = func(env *cli.Env) *cobra.Command {
			var repo string
			cmd := &cobra.Command{
				Use:     "one [arg]",
				Short:   "the first subcommand",
				Example: "julieta fixturegroup one --repo /x",
				Args:    cobra.MaximumNArgs(1),
				RunE: func(_ *cobra.Command, args []string) error {
					_, err := fmt.Fprintf(env.Stdout, "one repo %s args %s\n", repo, strings.Join(args, ","))
					return err
				},
			}
			cmd.Flags().StringVar(&repo, "repo", "/work", "the `dir` of the repository")
			cmd.Flags().Int("count", 3, "how many")
			return cmd
		}
		return c
	}())
	_ = cli.Register(func() cli.Command {
		c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
		c.Binary, c.Name, c.Sub = "julieta", "fixturegroup", "two"
		c.New = func(env *cli.Env) *cobra.Command {
			return &cobra.Command{
				Use:   "two",
				Short: "the second subcommand",
				Args:  cobra.NoArgs,
				RunE: func(*cobra.Command, []string) error {
					_, err := fmt.Fprintln(env.Stdout, "two")
					return err
				},
			}
		}
		return c
	}())
)

// run runs Main for the binary with args and returns the exit status,
// stdout and stderr.
func run(t *testing.T, binary string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: binary, Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), Now: func() time.Time { return fixedNow }}
	code := cli.Main(t.Context(), args, env)
	return code, stdout.String(), stderr.String()
}

// romeu runs Main for romeu with args.
func romeu(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return run(t, "romeu", args...)
}

// julieta runs Main for julieta with args.
func julieta(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return run(t, "julieta", args...)
}

// TestMainRunsTheFixture runs the fixture command through Main with a
// fixed clock, with GNU-style flags (04 4.1, Parsing): a flag after an
// argument, --name=value, a one-letter short form and "--", after which
// a word that starts with a dash is an argument.
func TestMainRunsTheFixture(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"fixture", "a", "b"}, "args a,b"},
		{[]string{"fixture", "a", "--detail", "x", "b"}, "args a,b"},
		{[]string{"fixture", "--detail=x", "a"}, "args a"},
		{[]string{"fixture", "-d", "x", "a"}, "args a"},
		{[]string{"fixture", "a", "--", "--fail", "-d"}, "args a,--fail,-d"},
	}
	for _, tt := range tests {
		code, stdout, stderr := romeu(t, tt.args...)
		assert.Equal(t, 0, code, "%v", tt.args)
		assert.Equal(t, "now 2026-10-17T12:00:00Z "+tt.want+"\n", stdout, "%v", tt.args)
		assert.Empty(t, stderr, "%v", tt.args)
	}
}

// TestMainReportsAnError prints the outermost error of the table in the
// text form and exits with its row's code; a flag value reaches the
// command after an argument as before one.
func TestMainReportsAnError(t *testing.T) {
	code, stdout, stderr := romeu(t, "fixture", "a", "--fail", "--detail=3")
	assert.Equal(t, 4, code)
	assert.Empty(t, stdout)
	assert.Equal(t, "error RJ-999 fixture-failed: it failed\nfix: decide: retry\nstep: 3\n", stderr)
}

// TestMainReportsJSON prints the --json form of the error: one object
// with id, slug, exit, message, fix and the details object.
func TestMainReportsJSON(t *testing.T) {
	code, _, stderr := romeu(t, "fixture", "--json", "--fail")
	assert.Equal(t, 4, code)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &got))
	assert.Equal(t, map[string]any{
		"id": "RJ-999", "slug": "fixture-failed", "exit": float64(4), "message": "it failed",
		"fix": "decide: retry", "details": map[string]any{"step": "2"},
	}, got)
}

// TestMainInternal prints an error with no id that a command returns as
// the error internal (RJ-348), exit 1.
func TestMainInternal(t *testing.T) {
	code, _, stderr := romeu(t, "fixture", "--plain")
	assert.Equal(t, 1, code)
	assert.True(t, strings.HasPrefix(stderr, "error RJ-348 internal: "), stderr)
}

// TestMainInterrupted prints nothing for an error that is the
// cancellation of the context, since procgroup.Main prints the one line
// of an interrupted command and maps the status 1 to 130 or 143 (04
// 4.1, Signals); context.Canceled from a context that is not canceled
// is still the error internal.
func TestMainInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), Now: func() time.Time { return fixedNow }}
	assert.Equal(t, 1, cli.Main(ctx, []string{"fixture", "--context"}, env))
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String(), "procgroup.Main prints the interrupted line")

	code, _, got := romeu(t, "fixture", "--context")
	assert.Equal(t, 1, code)
	assert.True(t, strings.HasPrefix(got, "error RJ-348 internal: "), got)

	stderr.Reset()
	assert.Equal(t, 4, cli.Main(ctx, []string{"fixture", "--context", "--fail"}, env))
	assert.True(t, strings.HasPrefix(stderr.String(), "error RJ-999 fixture-failed: "), "an error of the table joined to the cancellation is printed: %s", stderr.String())
}

// TestMainUsage exits 2 with the error usage (RJ-349) for every error of
// the parser, with the hint naming the help to read (04 4.1, Parsing):
// no command, an unknown command, subcommand or flag, a single-dash long
// flag, an argument count a command refuses, a group command with no
// subcommand and help of an unknown command.
func TestMainUsage(t *testing.T) {
	tests := []struct {
		binary string
		args   []string
		fix    string
	}{
		{"romeu", nil, "romeu --help"},
		{"romeu", []string{"nosuch"}, "romeu --help"},
		{"romeu", []string{"--nosuch"}, "romeu --help"},
		{"romeu", []string{"-json"}, "romeu --help"},
		{"romeu", []string{"fixture", "--nosuch"}, "romeu fixture --help"},
		{"romeu", []string{"fixture", "-json"}, "romeu fixture --help"},
		{"romeu", []string{"fixture", "--detail"}, "romeu fixture --help"},
		{"romeu", []string{"help", "nosuch"}, "romeu --help"},
		{"romeu", []string{"help", "fixture", "x"}, "romeu --help"},
		{"romeu", []string{"completion"}, "romeu completion --help"},
		{"romeu", []string{"completion", "powershell"}, "romeu completion --help"},
		{"romeu", []string{"Fixture"}, "romeu --help"},
		{"romeu", []string{"fix"}, "romeu --help"},
		{"julieta", []string{"fixturegroup"}, "julieta fixturegroup --help"},
		{"julieta", []string{"fixturegroup", "three"}, "julieta fixturegroup --help"},
		{"julieta", []string{"fixturegroup", "--json"}, "julieta fixturegroup --help"},
		{"julieta", []string{"fixturegroup", "one", "a", "b"}, "julieta fixturegroup one --help"},
		{"julieta", []string{"fixturegroup", "two", "--json"}, "julieta fixturegroup two --help"},
	}
	for _, tt := range tests {
		code, stdout, stderr := run(t, tt.binary, tt.args...)
		assert.Equal(t, 2, code, "%v", tt.args)
		assert.Empty(t, stdout, "%v", tt.args)
		assert.True(t, strings.HasPrefix(stderr, "error RJ-349 usage: "), "%v: %s", tt.args, stderr)
		assert.Contains(t, stderr, "\nfix: "+tt.fix+"\n", "%v", tt.args)
	}
}

// TestMainSubcommands runs a command's subcommand with its own flags.
func TestMainSubcommands(t *testing.T) {
	code, stdout, _ := julieta(t, "fixturegroup", "one", "a", "--repo", "/r")
	assert.Equal(t, 0, code)
	assert.Equal(t, "one repo /r args a\n", stdout)
	code, stdout, _ = julieta(t, "fixturegroup", "two")
	assert.Equal(t, 0, code)
	assert.Equal(t, "two\n", stdout)
}

// TestMainHelpOnEveryLevel prints, with --help, -h and help <command>,
// the help of the binary, of a command with subcommands and of a
// subcommand to stdout, each with the exit-code section, and the flag
// lines of a subcommand name each value and its default.
func TestMainHelpOnEveryLevel(t *testing.T) {
	levels := []struct {
		binary string
		args   [][]string
		want   string
	}{
		{"romeu", [][]string{{"--help"}, {"-h"}, {"help"}}, "Usage: romeu <command> [flags]\n\nthe commands that run on the host\n\nCommands:\n  approve     acknowledge the toolchain of this machine (gate 1)\n  completion  print the shell completion script\n  fixture     a command of the tests\n  help        print the help of a command\n"},
		{"julieta", [][]string{{"fixturegroup", "--help"}, {"fixturegroup", "-h"}, {"help", "fixturegroup"}}, "Usage: julieta fixturegroup <command> [flags]\n\nthe fixturegroup commands\n\nCommands:\n  one  the first subcommand\n  two  the second subcommand\n"},
		{"julieta", [][]string{{"fixturegroup", "one", "--help"}, {"fixturegroup", "one", "-h"}, {"help", "fixturegroup", "one"}}, "Usage: julieta fixturegroup one [arg] [flags]\n\nthe first subcommand\n\nFlags:\n      --count int   how many (default 3)\n  -h, --help        help for one\n      --repo dir    the dir of the repository (default \"/work\")\n\nExamples:\n  julieta fixturegroup one --repo /x\n"},
	}
	for _, level := range levels {
		for _, args := range level.args {
			code, stdout, stderr := run(t, level.binary, args...)
			assert.Equal(t, 0, code, "%v", args)
			assert.Empty(t, stderr, "%v", args)
			assert.True(t, strings.HasPrefix(stdout, level.want), "%v:\n%s", args, stdout)
			assert.True(t, strings.HasSuffix(stdout, "\n"+cli.ExitCodesSection()), "%v: the exit-code section ends the help", args)
		}
	}
	_, stdout, _ := romeu(t, "fixture", "--help")
	assert.Contains(t, stdout, "      --json           print one JSON document\n", "a command that takes --json lists it")
}

// TestCompletion prints the completion script of each shell raw, as
// cobra's generator writes it, and answers __complete with the names of
// the tree and real tabs, the raw output exception of 04 4.1.
func TestCompletion(t *testing.T) {
	tree := cli.Tree(&cli.Env{Binary: "romeu"})
	gens := map[string]func(io.Writer) error{
		"bash": func(w io.Writer) error { return tree.GenBashCompletionV2(w, true) },
		"zsh":  tree.GenZshCompletion,
		"fish": func(w io.Writer) error { return tree.GenFishCompletion(w, true) },
	}
	for shell, gen := range gens {
		var want bytes.Buffer
		require.NoError(t, gen(&want))
		code, stdout, stderr := romeu(t, "completion", shell)
		assert.Equal(t, 0, code, shell)
		assert.Empty(t, stderr, shell)
		assert.Equal(t, want.String(), stdout, shell)
	}
	code, stdout, stderr := romeu(t, "__complete", "fix")
	assert.Equal(t, 0, code)
	assert.Equal(t, "fixture\ta command of the tests\n:4\n", stdout)
	assert.Contains(t, stderr, "Completion ended with directive")
	code, stdout, _ = julieta(t, "__complete", "fixturegroup", "")
	assert.Equal(t, 0, code)
	assert.Equal(t, "one\tthe first subcommand\ntwo\tthe second subcommand\n:4\n", stdout)
}

// TestNoOutputOutsideTheWriter runs every path of the tests above with
// os.Stdout and os.Stderr swapped for pipes, and finds nothing written
// to them: the tree prints through the writers of Env alone (I29).
func TestNoOutputOutsideTheWriter(t *testing.T) {
	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutW, stderrW
	paths := [][]string{
		nil, {"--help"}, {"help"}, {"help", "fixture"}, {"help", "nosuch"}, {"nosuch"}, {"-json"},
		{"fixture"}, {"fixture", "--help"}, {"fixture", "--fail"}, {"fixture", "--plain"}, {"fixture", "--json", "--fail"}, {"fixture", "--nosuch"},
		{"completion"}, {"completion", "bash"}, {"completion", "zsh"}, {"completion", "fish"}, {"completion", "--help"},
		{"__complete", "fix"}, {"__complete", "fixture", "--"}, {"__completeNoDesc", ""},
	}
	for _, args := range paths {
		romeu(t, args...)
	}
	julieta(t, "fixturegroup")
	julieta(t, "fixturegroup", "one", "-h")
	os.Stdout, os.Stderr = oldOut, oldErr
	require.NoError(t, stdoutW.Close())
	require.NoError(t, stderrW.Close())
	out, err := io.ReadAll(stdoutR)
	require.NoError(t, err)
	errOut, err := io.ReadAll(stderrR)
	require.NoError(t, err)
	assert.Empty(t, string(out), "nothing reaches os.Stdout")
	assert.Empty(t, string(errOut), "nothing reaches os.Stderr")
}

// TestMainEscapes feeds a control character and a line feed through an
// argument, a flag error and a detail, and finds each escaped by the CLI
// writer and kept on one line (I29): the error line and the fix line,
// and the detail line of the fixture error.
func TestMainEscapes(t *testing.T) {
	hostile := "a\x1b[31m\nb"
	tests := []struct {
		args  []string
		lines int
	}{
		{[]string{"no" + hostile}, 2},
		{[]string{"fixture", "--no" + hostile}, 2},
		{[]string{"fixture", "--fail", "--detail", hostile}, 3},
	}
	for _, tt := range tests {
		_, _, stderr := romeu(t, tt.args...)
		assert.NotContains(t, stderr, "\x1b", "%q", tt.args)
		assert.Equal(t, tt.lines, strings.Count(stderr, "\n"), "%q: one value per line:\n%s", tt.args, stderr)
	}
	_, _, stderr := romeu(t, "fixture", "--fail", "--detail", hostile)
	assert.Contains(t, stderr, "\nstep: "+termsafe.Escape("a\x1b[31m")+`\nb`+"\n")
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

// TestCompletionErrorGoesThroughTheWriter completes a line whose flag
// does not parse, which makes cobra print "[Error] ..." with
// CompErrorln, and finds the line on the CLI writer, escaped, and
// nothing on os.Stderr (04 4.1, Output).
func TestCompletionErrorGoesThroughTheWriter(t *testing.T) {
	errR, errW, err := os.Pipe()
	require.NoError(t, err)
	oldErr := os.Stderr
	os.Stderr = errW
	t.Cleanup(func() {
		os.Stderr = oldErr
		_ = errW.Close()
		_ = errR.Close()
	})
	code, _, stderr := romeu(t, "__complete", "fixture", "--no\x1b[31msuch", "")
	assert.Same(t, errW, os.Stderr, "Main restores the os.Stderr it found")
	require.NoError(t, errW.Close())
	leaked, err := io.ReadAll(errR)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Empty(t, string(leaked), "nothing reaches os.Stderr")
	assert.Contains(t, stderr, "[Error] ")
	assert.NotContains(t, stderr, "\x1b")
}

// TestCompletionWritesNoDebugFile sets BASH_COMP_DEBUG_FILE and finds
// that cobra appends nothing to it: Main unsets the variable before
// cobra runs.
func TestCompletionWritesNoDebugFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug")
	t.Setenv("BASH_COMP_DEBUG_FILE", path)
	romeu(t, "__complete", "fixture", "--nosuch", "")
	_, err := os.Stat(path)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

// TestCompletionWithoutAStderrWriter completes with no stderr writer in
// Env: Main swaps nothing and leaves os.Stderr as it found it.
func TestCompletionWithoutAStderrWriter(t *testing.T) {
	errR, errW, err := os.Pipe()
	require.NoError(t, err)
	oldErr := os.Stderr
	os.Stderr = errW
	t.Cleanup(func() {
		os.Stderr = oldErr
		_ = errW.Close()
		_ = errR.Close()
	})
	var stdout bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(&stdout), Now: func() time.Time { return fixedNow }}
	code := cli.Main(t.Context(), []string{"__complete", "fix"}, env)
	assert.Same(t, errW, os.Stderr)
	assert.Equal(t, 0, code)
	assert.Equal(t, "fixture\ta command of the tests\n:4\n", stdout.String())
}
