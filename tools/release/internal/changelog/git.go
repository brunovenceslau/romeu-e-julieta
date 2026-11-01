// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package changelog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// toolEnv is the environment of git and git-cliff: where programs and
// the home are, and nothing else, as the build package gives its git.
func toolEnv() []string {
	var env []string
	for _, key := range []string{"PATH", "HOME", "TMPDIR"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// output runs argv in dir under the class and returns its standard
// output. A failure carries the standard error, escaped (escape).
func output(ctx context.Context, class procgroup.Class, dir string, argv []string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	err := procgroup.Run(ctx, procgroup.Command{
		Class: class, Argv: argv, Dir: dir, Env: toolEnv(), Stdout: &stdout, Stderr: &stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", strings.Join(argv, " "), err, escape(bytes.TrimSpace(stderr.Bytes())))
	}
	return stdout.Bytes(), nil
}

// git runs git in dir with the keys that run a program from a
// repository's config switched off.
func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	argv := slices.Concat([]string{"git", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null"}, args)
	return output(ctx, procgroup.ToolGit, dir, argv)
}

// gitExit returns the exit status of a failed git run, or -1 when the
// error is not an exit.
func gitExit(err error) int {
	if exit, ok := errors.AsType[*procgroup.ExitError](err); ok {
		return exit.Code
	}
	return -1
}

// show returns the content of path at commit, and absent when the
// commit has no such file (git exits 128 with "does not exist" or
// "exists on disk, but not in"); any other failure is an error.
func show(ctx context.Context, dir, commit, path string) (content []byte, absent bool, err error) {
	out, err := git(ctx, dir, "show", commit+":"+path)
	if err == nil {
		return out, false, nil
	}
	if gitExit(err) == 128 && (strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "exists on disk, but not in")) {
		return nil, true, nil
	}
	return nil, false, err
}

// escapeMax is how much of a tool's output an error carries.
const escapeMax = 4096

// escape returns s with every byte outside printable ASCII, a newline
// and a tab excepted, written as a Go escape, and cut to escapeMax
// bytes of the source, so a tool's output can reach a terminal.
func escape(s []byte) string {
	if len(s) > escapeMax {
		s = s[:escapeMax]
	}
	var b strings.Builder
	for _, r := range string(s) {
		switch {
		case r == '\n' || r == '\t' || (r >= 0x20 && r <= 0x7e):
			b.WriteRune(r)
		default:
			q := strconv.QuoteToASCII(string(r))
			b.WriteString(q[1 : len(q)-1])
		}
	}
	return b.String()
}
