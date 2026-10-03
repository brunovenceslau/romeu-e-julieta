// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package git runs the git commands the checks read a repository with.
// The checks only read: nothing here writes a ref, the index or the
// working tree.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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
//
// Two settings hold for every command, whatever the configuration.
// Replace refs are off: a push sends the objects themselves, so a local
// replace ref must not show the checks a different object from the one
// that leaves the machine. core.quotePath is on: a path that git quotes
// then has every byte outside ASCII escaped, so unquoting gives back its
// bytes exactly.
func (r Repo) Run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	fixed := []string{"--no-replace-objects", "-c", "core.quotePath=true"}
	cmd := exec.CommandContext(ctx, "git", append(fixed, args...)...)
	cmd.Dir = r.Dir
	cmd.Env = r.Env
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, Printable(strings.TrimSpace(stderr.String())))
	}
	return out, nil
}

// Printable returns s as it is when it is UTF-8 and every character of
// it prints, and quoted in Go syntax with only ASCII otherwise. A path
// and a message of git pass through it before they are printed: a
// control character or a newline in them would move the terminal or
// add a line of its own to the output, such as a forged "ok" line.
func Printable(s string) string {
	if !utf8.ValidString(s) || strings.ContainsFunc(s, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return strconv.QuoteToASCII(s)
	}
	return s
}
