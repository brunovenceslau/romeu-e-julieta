// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"errors"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// TestCallErrorDetailsTable is the details table of T029 (04 4.1, Error
// ids, item 7): for each way a call ends badly, the escaped argv, the
// status, the head of the stderr with the home path, the user name, the
// host name and the root path replaced, the step and the three versions.
func TestCallErrorDetailsTable(t *testing.T) {
	scrub := Scrubber{Home: "/home/agent", User: "agent", Host: "agent-mbp", Root: "/home/agent/dev"}
	versions := Versions{Romeu: "v1.2.3", Julieta: "v1.2.4", Sbx: "0.48.0"}
	call := Call{Name: "stop", Argv: []string{"stop", "shop"}}
	exit := func(code int) error { return &procgroup.ExitError{Argv0: "sbx", Code: code} }
	tests := []struct {
		name   string
		call   Call
		err    error
		stderr string
		step   string
		want   []cli.Detail
	}{
		{
			name: "an exit status", call: call, err: exit(1), stderr: "Error: no sandbox shop\n", step: "stop step 2",
			want: []cli.Detail{
				{Key: "argv", Value: "sbx stop shop"}, {Key: "status", Value: "exit 1"}, {Key: "stderr", Value: `Error: no sandbox shop\n`},
				{Key: "step", Value: "stop step 2"},
				{Key: "romeu version", Value: "v1.2.3"}, {Key: "julieta version", Value: "v1.2.4"}, {Key: "sbx version", Value: "0.48.0"},
			},
		},
		{
			name: "the four names of the host", call: call, err: exit(2), step: "run step 3",
			stderr: "mkdir /home/agent/dev/shop-env/.romeu: denied; user agent on agent-mbp, config /home/agent/.config/sbx",
			want: []cli.Detail{
				{Key: "argv", Value: "sbx stop shop"}, {Key: "status", Value: "exit 2"},
				{Key: "stderr", Value: "mkdir <root>/shop-env/.romeu: denied; user <user> on <host>, config <home>/.config/sbx"},
				{Key: "step", Value: "run step 3"},
				{Key: "romeu version", Value: "v1.2.3"}, {Key: "julieta version", Value: "v1.2.4"}, {Key: "sbx version", Value: "0.48.0"},
			},
		},
		{
			name: "a signal", call: call, err: &procgroup.ExitError{Argv0: "sbx", Code: -1, Signal: syscall.SIGKILL}, step: "stop step 2",
			want: []cli.Detail{
				{Key: "argv", Value: "sbx stop shop"}, {Key: "status", Value: "ended by signal 9 (killed)"}, {Key: "stderr", Value: ""},
				{Key: "step", Value: "stop step 2"},
				{Key: "romeu version", Value: "v1.2.3"}, {Key: "julieta version", Value: "v1.2.4"}, {Key: "sbx version", Value: "0.48.0"},
			},
		},
		{
			name: "a program that is not there", call: call, err: procgroup.ErrNotFound, step: "preflight step 1",
			want: []cli.Detail{
				{Key: "argv", Value: "sbx stop shop"}, {Key: "status", Value: "sbx was not found"}, {Key: "stderr", Value: ""},
				{Key: "step", Value: "preflight step 1"},
				{Key: "romeu version", Value: "v1.2.3"}, {Key: "julieta version", Value: "v1.2.4"}, {Key: "sbx version", Value: "0.48.0"},
			},
		},
		{
			name: "no step", call: call, err: exit(1),
			want: []cli.Detail{
				{Key: "argv", Value: "sbx stop shop"}, {Key: "status", Value: "exit 1"}, {Key: "stderr", Value: ""},
				{Key: "romeu version", Value: "v1.2.3"}, {Key: "julieta version", Value: "v1.2.4"}, {Key: "sbx version", Value: "0.48.0"},
			},
		},
		{
			name: "an argv with a control character and a space", call: Call{Name: "x", Argv: []string{"env", "rm", "/a b/\x1b[31m"}}, err: exit(1),
			want: []cli.Detail{
				{Key: "argv", Value: `sbx env rm "/a b/\u001b[31m"`}, {Key: "status", Value: "exit 1"}, {Key: "stderr", Value: ""},
				{Key: "romeu version", Value: "v1.2.3"}, {Key: "julieta version", Value: "v1.2.4"}, {Key: "sbx version", Value: "0.48.0"},
			},
		},
		{
			name: "an stderr with a control character", call: call, err: exit(1), stderr: "\x1b]0;title\x07 bell",
			want: []cli.Detail{
				{Key: "argv", Value: "sbx stop shop"}, {Key: "status", Value: "exit 1"}, {Key: "stderr", Value: `\u001b]0;title\u0007 bell`},
				{Key: "romeu version", Value: "v1.2.3"}, {Key: "julieta version", Value: "v1.2.4"}, {Key: "sbx version", Value: "0.48.0"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newCallError(tt.call, tt.err, []byte(tt.stderr), tt.step, versions, scrub)
			assert.Equal(t, tt.want, e.Details())
			assert.ErrorIs(t, e, tt.err, "the error of procgroup is in the chain")
		})
	}
}

// TestCallErrorTimeout names the class and the limit of a timeout, and
// keeps the timeout in the chain.
func TestCallErrorTimeout(t *testing.T) {
	te := &procgroup.TimeoutError{Argv0: "sbx", Class: procgroup.SbxRead, Limit: 30 * time.Second}
	e := newCallError(Version(), te, nil, "", Versions{}, Scrubber{})
	require.Contains(t, e.Details(), cli.Detail{Key: "status", Value: "timed out: class sbx-read, limit 30s"})
	got, ok := errors.AsType[*procgroup.TimeoutError](e)
	require.True(t, ok)
	assert.Equal(t, "RJ-323", got.ID())
	assert.Contains(t, e.Details(), cli.Detail{Key: "sbx version", Value: "unknown"})
}

// TestCallErrorOther says that a call that did not complete for another
// reason did not complete.
func TestCallErrorOther(t *testing.T) {
	e := newCallError(Version(), errors.New("boom"), nil, "", Versions{}, Scrubber{})
	assert.Contains(t, e.Details(), cli.Detail{Key: "status", Value: "did not complete"})
	assert.Equal(t, "sbx version failed: did not complete", e.Error())
}

// TestCallErrorArgv holds the argv of the details to the scrub of the
// stderr: the home path, the user name and the root path are replaced in
// it too, the value of --command and of --env is replaced whole, and a
// word is cut at argvWordMax.
func TestCallErrorArgv(t *testing.T) {
	scrub := Scrubber{Home: "/home/agent", User: "agent", Host: "agent-mbp", Root: "/home/agent/dev"}
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{"a root path", []string{"env", "rm", "--force", "/home/agent/dev/shop-env"}, "sbx env rm --force <root>/shop-env"},
		{"a home path and the user", []string{"stop", "agent", "/home/agent/x"}, "sbx stop <user> <home>/x"},
		{"the value of --command", []string{"secret", "set", "github", "--sandbox", "shop", "--command", "/home/agent/bin/token --secret=abc", "--refresh", "55m"},
			"sbx secret set github --sandbox shop --command <command> --refresh 55m"},
		{"the value of --command=", []string{"secret", "set", "github", "--command=/bin/get abc"}, "sbx secret set github --command=<command>"},
		{"the value of --env", []string{"env", "exec", "-it", "/d", "--env", "JULIETA_MANIFEST=eyJ", "--", "/x/julieta", "version"},
			"sbx env exec -it /d --env <env> -- /x/julieta version"},
		{"the value of --env=", []string{"env", "exec", "/d", "--env=A=b", "--", "x"}, "sbx env exec /d --env=<env> -- x"},
		{"a command after the double dash is scrubbed and not read as flags", []string{"env", "exec", "/d", "--", "cat", "/home/agent/.ssh/id", "--command", "secret"},
			"sbx env exec /d -- cat <home>/.ssh/id --command secret"},
		{"a long word", []string{"stop", strings.Repeat("a", 1000)}, "sbx stop " + strings.Repeat("a", argvWordMax) + "..."},
		{"a long word cut on a character", []string{"stop", strings.Repeat("a", argvWordMax-1) + "é"}, "sbx stop " + strings.Repeat("a", argvWordMax-1) + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newCallError(Call{Name: "x", Argv: tt.argv}, &procgroup.ExitError{Argv0: "sbx", Code: 1}, nil, "", Versions{}, scrub)
			assert.Equal(t, cli.Detail{Key: "argv", Value: tt.want}, e.Details()[0])
		})
	}
}

// TestStderrHeadIsFourKiB holds the head to the 4096 bytes of 04 4.1
// by the number, not by the constant that names it.
func TestStderrHeadIsFourKiB(t *testing.T) {
	assert.Equal(t, 4096, StderrHead)
	got := head([]byte(strings.Repeat("x", 5000)), Scrubber{})
	assert.Len(t, got, 4096)
	assert.Equal(t, strings.Repeat("x", 4095), head([]byte(strings.Repeat("x", 4095)), Scrubber{}))
}
