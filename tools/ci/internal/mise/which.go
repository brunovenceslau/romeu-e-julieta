// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// ResolveLintTools asks "mise which" for the paths of the versions that
// mise.toml pins and mise.lock locks, in the environment of
// check.PassThroughEnv with MiseEnv. "mise which" runs no tool and installs
// none: a tool that is not installed is an error that names the two
// commands a new machine runs first, so fast fails closed and never
// falls back to another golangci-lint or go on the search path.
//
// Each path must lie, once its symbolic links are resolved, in the
// directory where mise installs that tool (miseInstalls), and the error
// names that directory. A mise configuration may name a tool by a path
// ("path:" in [tools], in the repository or in the global
// configuration); such a tool is no pinned one, and it is an error here
// too.
func ResolveLintTools(ctx context.Context, root string) (check.Tools, error) {
	linter, err := WhichPinned(ctx, root, LintTool, LintTool, LintTool)
	if err != nil {
		return check.Tools{}, err
	}
	goBin, err := WhichPinned(ctx, root, GoTool, GoTool, GoTool)
	if err != nil {
		return check.Tools{}, err
	}
	return check.Tools{Linter: linter, GoDir: filepath.Dir(goBin)}, nil
}

// WhichPinned returns the path "mise which" resolves for the program
// name, once its symbolic links are resolved, and fails unless it lies
// in dir, the directory below the installs directory (miseInstalls)
// where mise installs that tool. The next path segment, the version
// directory, must equal the version mise.lock locks under key (the
// tool's name in the lock), so that a stale install left beside the
// locked one is refused; a tool locked at two different versions is an
// error, as the lock then holds no one version to hold an install to.
// It starts the mise of Path, in check.PassThroughEnv with MiseEnv, at
// the top of the tree, and refuses a key that is not one of PinnedTools
// before it starts.
func WhichPinned(ctx context.Context, root, name, key, dir string) (string, error) {
	if !slices.Contains(PinnedTools, key) {
		return "", fmt.Errorf("mise which %s: %s is not one of the pinned tools (PinnedTools)", name, key)
	}
	installsDir, err := miseInstalls()
	if err != nil {
		return "", err
	}
	installs, err := filepath.EvalSymlinks(installsDir)
	if err != nil {
		return "", fmt.Errorf("the mise installs directory %s: %w; run \"mise trust\" and \"mise install\" in %s", installsDir, err, root)
	}
	misePath, err := Path(root)
	if errors.Is(err, procgroup.ErrNotFound) {
		return "", fmt.Errorf("mise is not on the search path: run \"go run ./tools/ci setup\" in %s", root)
	}
	if err != nil {
		return "", err
	}
	var stdout, stderr bytes.Buffer
	err = procgroup.Run(ctx, procgroup.Command{
		Class: procgroup.ToolProbe,
		Argv:  []string{misePath, "-C", root, "which", name},
		// mise finds .miserc.toml from its working directory (misefiles.go).
		Dir:    root,
		Env:    RunEnviron(check.PassThroughEnv(), root),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		detail := ""
		if _, ok := errors.AsType[*procgroup.ExitError](err); ok {
			detail = "\n" + string(bytes.TrimSpace(stderr.Bytes()))
		}
		return "", fmt.Errorf("mise which %s: %w; the tool is not installed or the configuration is not trusted: run \"mise trust\" and \"mise install\" in %s%s", name, err, root, detail)
	}
	path, err := filepath.EvalSymlinks(string(bytes.TrimSpace(stdout.Bytes())))
	if err != nil {
		return "", fmt.Errorf("mise which %s: %w", name, err)
	}
	want := filepath.Join(installs, dir)
	if rel, err := filepath.Rel(want, path); err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("mise which %s: %s is outside %s, where mise installs it: the mise configuration names a tool mise did not install", name, path, want)
	} else if err := lockedInstall(root, name, key, filepath.ToSlash(rel)); err != nil {
		return "", fmt.Errorf("%w: %s", err, path)
	}
	return path, nil
}

// lockedInstall fails unless the first segment of rel, the path of a
// tool below its install directory, is the version mise.lock locks for
// key.
func lockedInstall(root, name, key, rel string) error {
	lock, err := os.ReadFile(filepath.Join(root, "mise.lock"))
	if err != nil {
		return fmt.Errorf("mise which %s: mise.lock: %w; the version a tool runs at is the one it locks", name, err)
	}
	versions, err := LockedVersions(lock)
	if err != nil {
		return fmt.Errorf("mise which %s: %w", name, err)
	}
	version, ok := versions[key]
	if !ok {
		return fmt.Errorf("mise which %s: mise.lock locks no version of %s", name, key)
	}
	if first, _, _ := strings.Cut(rel, "/"); first != version {
		return fmt.Errorf("mise which %s: the install is not the version %s that mise.lock locks: run \"mise install\" in %s", name, version, root)
	}
	return nil
}

// LockedVersions returns the version mise.lock locks for each tool: the
// "version" key that follows each "[[tools.<name>]]" header, until any
// other table header. A tool locked at two different versions is an
// error; the same version written twice is not.
func LockedVersions(data []byte) (map[string]string, error) {
	locked := map[string]string{}
	tool := ""
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if name, ok := strings.CutPrefix(line, "[[tools."); ok {
			tool = strings.Trim(strings.TrimSuffix(name, "]]"), `"`)
			continue
		}
		if strings.HasPrefix(line, "[") {
			tool = "" // a version below another table is no tool's
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && tool != "" && strings.TrimSpace(key) == "version" {
			version := strings.Trim(strings.TrimSpace(value), `"`)
			if prev, dup := locked[tool]; dup && prev != version {
				return nil, fmt.Errorf("mise.lock locks %s at two different versions", tool)
			}
			locked[tool] = version
			tool = ""
		}
	}
	return locked, nil
}

// miseInstalls returns the directory where mise installs tools, the way
// mise resolves its data directory (measured on linux with mise
// 2026.10.3, whose "mise doctor" reports it; TestMiseInstalls repeats
// that measurement on every machine that runs the tests):
// $MISE_DATA_DIR if it is set, else mise below $XDG_DATA_HOME if that
// is set, else .local/share/mise below HOME, and installs below it.
// Both variables are on the allowlist (goenv.PassThrough): a caller who
// points them at an installs directory of their own chooses the go and
// the linter the checks run, the same way a caller's PATH chooses mise
// itself, so a caller who controls the environment can bypass the
// checks (misefiles.go, the threat model).
func miseInstalls() (string, error) {
	if dir := os.Getenv("MISE_DATA_DIR"); dir != "" {
		return filepath.Join(dir, "installs"), nil
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "mise", "installs"), nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("HOME is not set: fast finds the tools mise installs below it")
	}
	return filepath.Join(home, ".local", "share", "mise", "installs"), nil
}
