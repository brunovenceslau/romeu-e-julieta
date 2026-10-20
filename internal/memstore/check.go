// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memstore

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The rules of the findings of Check, each the slug of one way a memory
// dir breaks the layout of 08 8.1, which julieta memory check lists and
// romeu sync and run refuse (REQ-0651).
const (
	// RuleName is a name that is no element of a path, such as "..".
	RuleName = "name"
	// RuleTopLevel is a top-level name outside TopLevel.
	RuleTopLevel = "top-level"
	// RuleNotDir is a top-level name of TopLevel that is not a directory.
	RuleNotDir = "not-a-directory"
	// RuleDotfile is a name that starts with a dot and is not a
	// temporary name of julieta (REQ-0654).
	RuleDotfile = "dotfile"
	// RuleGit is a .git, a directory or a file.
	RuleGit = "git"
	// RuleSymlink, RuleFIFO, RuleDevice, RuleSocket and RuleIrregular
	// are a node that is neither a directory nor a regular file.
	RuleSymlink   = "symlink"
	RuleFIFO      = "fifo"
	RuleDevice    = "device"
	RuleSocket    = "socket"
	RuleIrregular = "irregular"
	// RuleEntryName is a member of entries/ that is not a file
	// <ULID>.md, which the reader could not read.
	RuleEntryName = "entry-name"
	// RuleFileSize is an entry or handoff file over MaxFileSize.
	RuleFileSize = "file-size"
	// RuleEntriesCap is an entries/ with more than MaxEntries entries.
	RuleEntriesCap = "entries-cap"
	// RuleNodesCap is a tree with more nodes than Check walks.
	RuleNodesCap = "nodes-cap"
)

// The rules of the warnings of Check: each is reported and refuses
// nothing.
const (
	// RuleTemp is a leftover temporary file of julieta (REQ-0654),
	// which julieta setup removes once it is older than its start.
	RuleTemp = "temp-file"
	// RuleHandoffCap is a handoff/ over MaxHandoffFiles (REQ-0650).
	RuleHandoffCap = "handoff-cap"
)

// Finding is one finding or warning of Check: the slash path of the
// node from the memory dir, "." for the dir itself, and its rule.
type Finding struct {
	Path string
	Rule string
}

// String returns "<rule> <path>", the path escaped with termsafe, so a
// name an agent wrote cannot forge a line of the output.
func (f Finding) String() string { return f.Rule + " " + termsafe.Escape(f.Path) }

// Report is what Check found in a memory dir: the findings, each a
// violation of the layout allowlist, and the warnings.
type Report struct {
	Findings []Finding
	Warnings []Finding
}

// classify returns the rule a node named name of mode mode breaks, or
// "" when the allowlist admits it, below the top level; top says it is
// at the top level. A temporary name of julieta that is a regular file
// is admitted here, and Check warns about it.
//
//romeu:invariant I12 guard
//romeu:invariant I24 guard
func classify(name string, mode fs.FileMode, top bool) string {
	switch {
	case !validName(name):
		return RuleName
	case name == ".git":
		return RuleGit
	case strings.HasPrefix(name, ".") && (!IsTemp(name) || !mode.IsRegular()):
		return RuleDotfile
	}
	switch t := mode.Type(); {
	case t&fs.ModeSymlink != 0:
		return RuleSymlink
	case t&fs.ModeNamedPipe != 0:
		return RuleFIFO
	case t&fs.ModeDevice != 0:
		return RuleDevice
	case t&fs.ModeSocket != 0:
		return RuleSocket
	case t&^fs.ModeDir != 0:
		return RuleIrregular
	}
	if top && !IsTemp(name) {
		if !slices.Contains(TopLevel, name) {
			return RuleTopLevel
		}
		if !mode.IsDir() {
			return RuleNotDir
		}
	}
	return ""
}

// Check walks the memory dir of root and returns its findings and
// warnings (08 8.1, REQ-0645, REQ-0648): every node is judged with
// Lstat by classify, a member of entries/ must be a file <ULID>.md, an
// entry or handoff file is at most MaxFileSize, entries/ holds at most
// MaxEntries entries, and a handoff/ over MaxHandoffFiles is a warning.
// A violating node is not descended into. The error is a read that
// failed.
func Check(root *os.Root) (Report, error) {
	var rep Report
	nodes, handoff, entries := 0, 0, 0
	stack := []string{"."}
	for len(stack) > 0 {
		dir := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		names, more, err := readNames(root, dir, maxNodes-nodes)
		if err != nil {
			return Report{}, err
		}
		if more {
			rep.Findings = append(rep.Findings, Finding{Path: ".", Rule: RuleNodesCap})
			return rep, nil
		}
		for _, name := range names {
			nodes++
			p := path.Join(dir, name)
			if !validName(name) {
				rep.Findings = append(rep.Findings, Finding{Path: p, Rule: RuleName})
				continue
			}
			info, err := root.Lstat(p)
			if err != nil {
				return Report{}, err
			}
			mode := info.Mode()
			if rule := classify(name, mode, dir == "."); rule != "" {
				rep.Findings = append(rep.Findings, Finding{Path: p, Rule: rule})
				continue
			}
			top := strings.SplitN(p, "/", 2)[0]
			if IsTemp(name) {
				rep.Warnings = append(rep.Warnings, Finding{Path: p, Rule: RuleTemp})
				if (top == EntriesDir || top == HandoffDir) && info.Size() > MaxFileSize {
					rep.Findings = append(rep.Findings, Finding{Path: p, Rule: RuleFileSize})
				}
				continue
			}
			if top == EntriesDir && dir != "." {
				if _, ok := entryID(name); !ok || dir != EntriesDir || !mode.IsRegular() {
					rep.Findings = append(rep.Findings, Finding{Path: p, Rule: RuleEntryName})
					continue
				}
				entries++
			}
			if top == HandoffDir && dir != "." {
				handoff++
			}
			if mode.IsRegular() && (top == EntriesDir || top == HandoffDir) && info.Size() > MaxFileSize {
				rep.Findings = append(rep.Findings, Finding{Path: p, Rule: RuleFileSize})
			}
			if mode.IsDir() {
				stack = append(stack, p)
			}
		}
	}
	if entries > MaxEntries {
		rep.Findings = append(rep.Findings, Finding{Path: EntriesDir, Rule: RuleEntriesCap})
	}
	if handoff > MaxHandoffFiles {
		rep.Warnings = append(rep.Warnings, Finding{Path: HandoffDir, Rule: RuleHandoffCap})
	}
	sortFindings(rep.Findings)
	sortFindings(rep.Warnings)
	return rep, nil
}

// sortFindings sorts fs by path, then rule.
func sortFindings(fs []Finding) {
	slices.SortFunc(fs, func(a, b Finding) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(a.Rule, b.Rule)
	})
}

// readNames returns the names of the directory dir of root, sorted,
// and more set when it holds more than limit names, in which case it
// returns none: a caller never sees a silent part of a directory.
func readNames(root *os.Root, dir string, limit int) (names []string, more bool, err error) {
	f, err := root.Open(dir)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()
	for len(names) <= limit {
		batch, err := f.Readdirnames(256)
		names = append(names, batch...)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, false, err
		}
	}
	if len(names) > limit {
		return nil, true, nil
	}
	slices.Sort(names)
	return names, false, nil
}

// ErrUnmounted is the error of a memory dir that is not there to check:
// absent, or not a directory (julieta memory check, exit 1).
var ErrUnmounted = errors.New("memstore: the memory dir is not mounted")

// ErrUnwritable is the error of a memory dir julieta cannot write
// (julieta memory check, exit 1).
var ErrUnwritable = errors.New("memstore: the memory dir is not writable")

// CheckDir checks the memory dir at dir, an absolute and clean path
// (04 4.3, REQ-0412): CheckMounted, then Check on it; the findings are
// in the report.
func CheckDir(dir string) (Report, error) {
	if err := CheckMounted(dir); err != nil {
		return Report{}, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = root.Close() }()
	return Check(root)
}

// CheckMounted returns why the memory dir at dir cannot be used: it is
// not an absolute and clean path, it is absent or no directory, a
// symlink among them (ErrUnmounted), or julieta cannot write it
// (ErrUnwritable).
func CheckMounted(dir string) error {
	if err := checkPath(dir); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %s is absent", ErrUnmounted, dir)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrUnmounted, dir)
	}
	if err := unix.Access(dir, unix.W_OK); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrUnwritable, dir, err)
	}
	return nil
}

// checkPath returns why dir cannot name a memory dir: it is not an
// absolute, clean path.
func checkPath(dir string) error {
	if !strings.HasPrefix(dir, "/") || path.Clean(dir) != dir {
		return fmt.Errorf("memstore: the memory dir %q is not an absolute, clean path", dir)
	}
	return nil
}
