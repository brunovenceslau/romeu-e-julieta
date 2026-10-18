// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package clidoc hands tools/ci the two generators of the command line
// (ADR 0015, decision 7): the pages of the error table, which live in
// internal/cli with the table, and one reference page per command of
// each binary's tree, written here with github.com/spf13/cobra/doc. It is
// the one importer of cobra/doc, and only tools/ci imports it, so no
// binary links cobra/doc or what it pulls in (10 10.7, REQ-1247).
package clidoc

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// Page is one file a generator writes: its slash-separated path from the
// root of the tree and its bytes.
type Page = cli.Page

// pathWord matches a word of a command path, so a page path stays below
// docs/reference/.
var pathWord = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// GeneratedHeader returns the lines of the generated header that the
// generators of internal/cli write (cli.GeneratedHeader), so a test of
// tools/ci holds them to the header of tools/ci/internal/check.
func GeneratedHeader(source string) []string { return cli.GeneratedHeader(source) }

// binaries are the binaries whose trees CommandPages walks, in order.
var binaries = []string{"romeu", "julieta"}

// ErrorPages returns the pages of the error table of the tree at root
// (cli.ErrorPages): docs/reference/errors.md,
// docs/reference/exit-codes.md and internal/cli/exitcodes_gen.go.
func ErrorPages(root *os.Root) ([]Page, error) { return cli.ErrorPages(root.FS()) }

// CommandPages returns one page per command of each binary's tree, the
// root and the group commands included and the hidden and help commands
// left out (12 12.3, REQ-1109): docs/reference/<binary>/<command
// path>.md, the words of the path as directories, so the root's page is
// docs/reference/<binary>.md. It reads the definitions this binary
// registered, so it does not read root, and returns none for a tree with
// no internal/cli, such as a fixture of the generated check of tools/ci.
func CommandPages(root *os.Root) ([]Page, error) {
	if _, err := root.Stat("internal/cli"); err != nil {
		return nil, nil
	}
	var pages []Page
	for _, binary := range binaries {
		defs := map[string]cli.Command{}
		for _, d := range cli.Commands(binary) {
			defs[binary+" "+d.Line()] = d
		}
		tree := cli.Tree(&cli.Env{Binary: binary})
		var walk func(c *cobra.Command) error
		walk = func(c *cobra.Command) error {
			if c != tree && !c.IsAvailableCommand() {
				return nil
			}
			def, ok := defs[c.CommandPath()]
			page, err := Of(c, def, ok)
			if err != nil {
				return err
			}
			pages = append(pages, page)
			for _, s := range c.Commands() {
				if err := walk(s); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(tree); err != nil {
			return nil, err
		}
	}
	return pages, nil
}

// pagePath returns the path of the page of the command whose path is
// words.
func pagePath(words []string) string {
	return "docs/reference/" + strings.Join(words, "/") + ".md"
}

// Of returns the page of the command c: the generated-file header and
// the front matter of REQ-1117, the text GenMarkdownCustom writes with
// its date tag off, and, when def is the command's definition (hasDef),
// its side-effect fields, then the exit codes, which cobra/doc does not
// render (ADR 0015, decision 7).
func Of(c *cobra.Command, def cli.Command, hasDef bool) (Page, error) {
	words := strings.Fields(c.CommandPath())
	for _, w := range words {
		if !pathWord.MatchString(w) {
			return Page{}, fmt.Errorf("the page of %q: %q is no word of a command path", c.CommandPath(), w)
		}
	}
	p := pagePath(words)
	dir := path.Dir(p)
	link := func(name string) string {
		target := pagePath(strings.Split(strings.TrimSuffix(name, ".md"), "_"))
		return relative(dir, target)
	}
	c.DisableAutoGenTag = true
	var body bytes.Buffer
	if err := doc.GenMarkdownCustom(c, &body, link); err != nil {
		return Page{}, fmt.Errorf("the page of %s: %w", c.CommandPath(), err)
	}
	text := strings.ReplaceAll(body.String(), "\t - ", " - ")
	if c.HasAvailableSubCommands() {
		// A command with subcommands runs none of its own: alone it is the
		// error usage, so its synopsis names the command it needs.
		text = strings.Replace(text, "```\n"+c.UseLine()+"\n```", "```\n"+c.CommandPath()+" <command> [flags]\n```", 1)
	}
	text = strings.Replace(text, "## "+c.CommandPath()+"\n", "# "+c.CommandPath()+"\n", 1)
	var b strings.Builder
	fmt.Fprintf(&b, `---
type: reference
reader: a user of %s who looks up a command, its flags and its exit codes
---

%s
`, words[0], cli.HTMLComment(cli.GeneratedHeader("internal/cli and internal/cmd/"+words[0]+"/** (the command tree of "+words[0]+", written through internal/cli/clidoc)")))
	b.WriteString(strings.TrimRight(text, "\n") + "\n")
	if hasDef {
		b.WriteString("\n### Side effects\n\n")
		b.WriteString(effects(def.Effects()))
	}
	b.WriteString("\n### Exit codes\n\n| Exit | Meaning |\n|---|---|\n")
	for _, e := range cli.ExitCodes {
		fmt.Fprintf(&b, "| %d | %s |\n", e.Code, e.Meaning)
	}
	fmt.Fprintf(&b, "\n%s\n", cli.SignalPrecedence)
	return Page{Path: p, Content: []byte(b.String())}, nil
}

// effects returns the list of the five side-effect fields of a
// definition: the P marker of 04 4.2, the lock, the appending list, the
// snapshot and the idempotency (04 4.1).
func effects(e cli.Effects) string {
	preflight := "no"
	if e.Preflight == cli.PreflightRuns {
		preflight = "yes, the command runs the preflight before its sbx calls"
	}
	lock := "no"
	if e.Lock == cli.LockTaken {
		lock = "yes, the romeu lock, for the whole command"
	}
	appends := "nothing"
	if e.Appending.What != "" {
		appends = e.Appending.What
	}
	snapshot := "no"
	if e.Snapshot == cli.SnapshotWrites {
		snapshot = "yes"
	}
	idempotency := map[cli.Idempotency]string{
		cli.Converges:      "a second run with no input change leaves files and refs as they were",
		cli.AppendsEachRun: "each run adds one thing",
		cli.ReadsOnly:      "the command writes nothing",
	}[e.Idempotency]
	return fmt.Sprintf("- **P** (preflight): %s\n- Lock: %s\n- Appends on each run: %s\n- Writes a julieta snapshot: %s\n- Idempotency: %s\n",
		preflight, lock, appends, snapshot, idempotency)
}

// relative returns the link from a page in dir to the page at target.
func relative(dir, target string) string {
	from := strings.Split(dir, "/")
	to := strings.Split(target, "/")
	i := 0
	for i < len(from) && i < len(to)-1 && from[i] == to[i] {
		i++
	}
	return strings.Repeat("../", len(from)-i) + strings.Join(to[i:], "/")
}
