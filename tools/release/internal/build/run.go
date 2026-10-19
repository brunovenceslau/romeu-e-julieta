// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/goenv"
)

// goEnv is the environment of every go command of a build, built from
// nothing as the steps of tools/ci are (10 10.2): PATH with the
// directory of the pinned go first, the variables that say where files
// are (goenv.PassThrough), the two lists that keep the go command on
// the pinned go and on the modules of the tree (goenv.Pin,
// goenv.Build), and GOPROXY=off, so a build reads no network and the
// module cache that "tools/ci setup" filled must hold go.sum's modules.
// The caller adds GOOS, GOARCH and CGO_ENABLED.
func goEnv(goBin string) []string {
	path := filepath.Dir(goBin)
	if value := os.Getenv("PATH"); value != "" {
		path += string(os.PathListSeparator) + value
	}
	var env []string
	for _, key := range goenv.PassThrough {
		if value := os.Getenv(key); value != "" && key != "PATH" {
			env = append(env, key+"="+value)
		}
	}
	return slices.Concat([]string{"PATH=" + path}, env, goenv.Pin, goenv.Build, []string{"GOPROXY=off"})
}

// ResolveGo returns the go command a build runs: the one "mise which go"
// resolves (12 12.1), or, where mise is not installed, the first go on
// PATH.
func ResolveGo(ctx context.Context) (string, error) {
	if _, err := procgroup.LookPath("mise"); err == nil {
		var env []string
		for _, key := range goenv.PassThrough {
			if value := os.Getenv(key); value != "" {
				env = append(env, key+"="+value)
			}
		}
		env = append(env, "MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES=none")
		out, err := output(ctx, procgroup.ToolProbe, "", env, []string{"mise", "which", "go"})
		if path := strings.TrimSpace(string(out)); err == nil && path != "" {
			return path, nil
		}
	}
	return procgroup.LookPath("go")
}

// gitEnv is the environment of a git command: where programs and the
// home are, and nothing else.
func gitEnv() []string {
	var env []string
	for _, key := range []string{"PATH", "HOME", "TMPDIR"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// output runs argv in dir under the class and returns its standard
// output. A failure carries the standard error.
func output(ctx context.Context, class procgroup.Class, dir string, env, argv []string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	err := procgroup.Run(ctx, procgroup.Command{
		Class: class, Argv: argv, Dir: dir, Env: env, Stdout: &stdout, Stderr: &stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", strings.Join(argv, " "), err, bytes.TrimSpace(stderr.Bytes()))
	}
	return stdout.Bytes(), nil
}

// git runs git in dir with the keys that run a program from a
// repository's config switched off.
func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	argv := slices.Concat([]string{"git", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null"}, args)
	return output(ctx, procgroup.ToolGit, dir, gitEnv(), argv)
}
