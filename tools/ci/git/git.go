// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-or-later

// Package git runs the git commands the checks read a repository with.
// The checks only read: nothing here writes a ref, the index or the
// working tree.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Timeout bounds one git command. The commands here read local objects,
// so a command that runs this long is stuck, and a stuck command must
// not hold a push open for ever.
const Timeout = 2 * time.Minute

// Repo is a repository on disk.
type Repo struct {
	// Dir is a directory inside the working tree.
	Dir string
	// Env is the environment of each git command. Nil means the
	// environment of this process, which is what a hook needs: git
	// tells a hook which repository it serves through it.
	Env []string
}

// Run runs git with args in the repository, gives it stdin, and returns
// its standard output. A command that exits non-zero is an error that
// carries what git wrote to standard error.
func (r Repo) Run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Dir
	cmd.Env = r.Env
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
