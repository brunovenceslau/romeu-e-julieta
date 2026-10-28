// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memory

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"syscall"
)

// The bounds of the memory-derivable check (04 4.3, REQ-1290).
const (
	// minDerivable is the fewest bytes of a trimmed body line the check
	// compares; a shorter line is too common to mean a copy.
	minDerivable = 40
	// maxInstruction is the most bytes of an instruction file it reads.
	maxInstruction = 1 << 20
	// maxDerivableLines is the most matches the warning names.
	maxDerivableLines = 10
)

// instructionFiles are the instruction files of a repo the check reads,
// beside .claude/rules/*.md and the files they import.
var instructionFiles = []string{"CLAUDE.md", "AGENTS.md", "CLAUDE.local.md"}

// rulesDir holds the rule files the pinned agent reads.
const rulesDir = ".claude/rules"

// derivable returns the messages of the warning memory-derivable for
// body, the body of an entry filed under the repo whose working tree is
// at repo (REQ-1290): one per line of an instruction file that equals,
// trimmed of white space on both sides, a body line of at least
// minDerivable bytes trimmed the same way, at most maxDerivableLines and
// then "<n> more", and one per instruction file over maxInstruction
// bytes, which is not compared. The files are CLAUDE.md, AGENTS.md,
// CLAUDE.local.md and .claude/rules/*.md at the working tree, then each
// file one of them imports with a line "@<path>" alone, resolved from
// the importing file's directory. Every read goes through an os.Root on
// the repo, so neither a symlink nor an import reaches out of it; an
// import out of the repo, a missing file and any file that cannot be
// read are skipped. A string comparison: no model call.
func derivable(repo, body string) []string {
	want := map[string]bool{}
	for line := range strings.Lines(body) {
		if l := strings.TrimSpace(line); len(l) >= minDerivable {
			want[l] = true
		}
	}
	if len(want) == 0 {
		return nil
	}
	root, err := os.OpenRoot(repo)
	if err != nil {
		return nil
	}
	defer func() { _ = root.Close() }()

	queue := slices.Clone(instructionFiles)
	queue = append(queue, ruleFiles(root)...)
	direct := len(queue)
	seen := map[string]bool{}
	var matches, notes []string
	for i := 0; i < len(queue); i++ {
		name := queue[i]
		if seen[name] {
			continue
		}
		seen[name] = true
		data, err := readInstruction(root, name)
		if errors.Is(err, errOversized) {
			notes = append(notes, fmt.Sprintf("%s is over %d bytes and was not compared", esc(name), maxInstruction))
			continue
		}
		if err != nil {
			continue
		}
		n := 0
		for line := range strings.Lines(string(data)) {
			n++
			l := strings.TrimSpace(line)
			if want[l] {
				matches = append(matches, fmt.Sprintf("%s:%d holds a line of the body; keep out what an instruction file already holds", esc(name), n))
			}
			if i < direct {
				if imp, ok := importOf(name, l); ok {
					queue = append(queue, imp)
				}
			}
		}
	}
	if len(matches) > maxDerivableLines {
		more := len(matches) - maxDerivableLines
		matches = append(matches[:maxDerivableLines], fmt.Sprintf("%d more", more))
	}
	return append(matches, notes...)
}

// ruleFiles returns the names of the *.md files of .claude/rules, sorted.
func ruleFiles(root *os.Root) []string {
	d, err := root.Open(rulesDir)
	if err != nil {
		return nil
	}
	defer func() { _ = d.Close() }()
	entries, err := d.ReadDir(-1)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			names = append(names, rulesDir+"/"+e.Name())
		}
	}
	slices.Sort(names)
	return names
}

// importOf returns the file the trimmed line l of the instruction file
// name imports, "@<path>" alone on a line, resolved from the directory
// of name; an absolute path, or one that leaves the repo, imports
// nothing.
func importOf(name, l string) (string, bool) {
	p, ok := strings.CutPrefix(l, "@")
	if !ok || p == "" || strings.ContainsAny(p, " \t") || path.IsAbs(p) || strings.HasPrefix(p, "~") {
		return "", false
	}
	p = path.Join(path.Dir(name), p)
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}

// errOversized is the error of an instruction file over maxInstruction
// bytes.
var errOversized = errors.New("over the size the check reads")

// readInstruction returns the content of the regular file name of root,
// at most maxInstruction bytes, opened without waiting on a FIFO; a
// larger one is errOversized.
func readInstruction(root *os.Root, name string) ([]byte, error) {
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fs.ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(f, maxInstruction+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxInstruction {
		return nil, errOversized
	}
	return data, nil
}
