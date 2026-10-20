// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli_test

import (
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	"github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
)

// skillsDir is skills/ seen from this package.
const skillsDir = "../../skills"

// TestSkillsNameRealCommands reads every skills/*/SKILL.md and fails on
// a julieta command or flag it names that the command definitions lack,
// and on headings of the handoff skill that differ from those
// `julieta handoff write` requires (12 12.3, REQ-1115). The headings
// come from handoff.Headings, the list CheckHeadings enforces.
func TestSkillsNameRealCommands(t *testing.T) {
	tree := cli.Tree(&cli.Env{Binary: "julieta"})
	files, err := os.ReadDir(skillsDir)
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		b, err := os.ReadFile(path.Join(skillsDir, f.Name(), "SKILL.md"))
		require.NoError(t, err, "skills/%s needs a SKILL.md", f.Name())
		seen[f.Name()] = true
		assert.Empty(t, checkSkill(tree, f.Name(), string(b)), "skills/%s/SKILL.md", f.Name())
	}
	assert.True(t, seen["julieta"], "skills/julieta/SKILL.md is missing")
	assert.True(t, seen["handoff"], "skills/handoff/SKILL.md is missing")
}

// TestSkillsFixtures holds checkSkill to what it promises: a whole skill
// passes, and each defect is named.
func TestSkillsFixtures(t *testing.T) {
	tree := cli.Tree(&cli.Env{Binary: "julieta"})
	clear := "## Goal\n## State\n## Decisions\n## Next steps\n## Open questions\n"
	final := clear + "## What would be lost\n"
	skill := func(body string) string { return "---\nname: handoff\ndescription: d\n---\n" + body }
	block := func(kind, headings string) string { return "```markdown kind=" + kind + "\n" + headings + "```\n" }
	whole := skill("`julieta memory add --title t --tag lesson`\n" +
		"```sh\njulieta handoff write --final <<'EOF'\nEOF\n```\n" + block("clear", clear) + block("final", final))

	tests := []struct {
		name string
		text string
		want string // a substring of the problem; empty means none
	}{
		{"a whole skill", whole, ""},
		{"an unknown flag", strings.Replace(whole, "--final <<", "--finale <<", 1), "--finale"},
		{"an unknown flag in a span", skill("`julieta memory add --nope`\n" + block("clear", clear) + block("final", final)), "--nope"},
		{"an unknown command", strings.Replace(whole, "julieta memory add", "julieta remember", 1), "remember"},
		{"an unknown subcommand", skill("`julieta memory zap`\n" + block("clear", clear) + block("final", final)), "zap"},
		{"a missing heading", strings.Replace(whole, "## State\n## Decisions\n## Next steps\n## Open questions\n## What", "## State\n## Next steps\n## Open questions\n## What", 1), "Decisions"},
		{"an extra heading", strings.Replace(whole, "## State\n", "## State\n## Risks\n", 1), "Risks"},
		{"a missing block", skill(block("clear", clear)), "final"},
		{"an unknown kind", whole + block("facts", ""), "facts"},
		{"no front matter", strings.TrimPrefix(whole, "---\nname: handoff\ndescription: d\n---\n"), "front matter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(checkSkill(tree, "handoff", tt.text), "\n")
			if tt.want == "" {
				assert.Empty(t, got)
				return
			}
			assert.Contains(t, got, tt.want)
		})
	}
}

// checkSkill returns the problems of the SKILL.md text of the skill
// named name: a missing front matter, a julieta command or flag that
// tree lacks, and, for the skill "handoff", kind blocks whose headings
// differ from handoff.Headings. A line scan, not a Markdown parser: a
// command is a line of a code block, or a code span, that starts with
// "julieta".
func checkSkill(tree *cobra.Command, name, text string) []string {
	var problems []string
	lines := strings.Split(text, "\n")
	if len(lines) < 4 || lines[0] != "---" || !slices.Contains(lines[1:], "---") || !slices.Contains(lines, "name: "+name) {
		problems = append(problems, "the front matter lacks name: "+name)
	}
	blocks := map[string][]string{} // kind -> headings of its block
	kind, fenced := "", false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if info, ok := strings.CutPrefix(trimmed, "```"); ok {
			fenced = !fenced
			kind = ""
			if fenced {
				for f := range strings.FieldsSeq(info) {
					if k, ok := strings.CutPrefix(f, "kind="); ok {
						kind = k
						blocks[k] = []string{}
					}
				}
			}
			continue
		}
		switch {
		case kind != "":
			if h, ok := strings.CutPrefix(trimmed, "## "); ok {
				blocks[kind] = append(blocks[kind], h)
			}
		case fenced:
			problems = append(problems, commandProblems(tree, trimmed)...)
		default:
			for span := range codeSpans(line) {
				problems = append(problems, commandProblems(tree, span)...)
			}
		}
	}
	if name != "handoff" {
		return problems
	}
	for _, k := range []handoff.Kind{handoff.Clear, handoff.Final} {
		got, ok := blocks[string(k)]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("no block kind=%s names the headings of a %s handoff", k, k))
		case !slices.Equal(got, handoff.Headings(k)):
			problems = append(problems, fmt.Sprintf("the headings for %s are %q, julieta handoff write requires %q", k, got, handoff.Headings(k)))
		}
	}
	for k := range blocks {
		if !handoff.Kind(k).Narrative() {
			problems = append(problems, fmt.Sprintf("a block kind=%s: only clear and final have headings", k))
		}
	}
	return problems
}

// codeSpans yields the text between backtick pairs of line.
func codeSpans(line string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		parts := strings.Split(line, "`")
		for i := 1; i < len(parts); i += 2 {
			if i < len(parts)-1 && !yield(parts[i]) {
				return
			}
		}
	}
}

// commandProblems returns the problems of one line or span if it is a
// julieta command line: the first word that is no command of the tree,
// and each flag the command does not have. Words after the command path
// are values and are skipped, as are placeholders such as <id>; the line
// ends at a pipe, a redirect or a list separator.
func commandProblems(tree *cobra.Command, line string) []string {
	words := strings.Fields(strings.TrimPrefix(line, "$ "))
	if len(words) < 2 || words[0] != "julieta" {
		return nil
	}
	cmd, descending := tree, true
	var problems []string
	for _, w := range words[1:] {
		switch {
		case w == "|" || w == "&&" || w == ";" || w == "<" || w == ">" || strings.HasPrefix(w, "<<") || strings.HasPrefix(w, ">>"):
			return problems
		case strings.HasPrefix(w, "<") && strings.HasSuffix(w, ">"):
			descending = false
		case strings.HasPrefix(w, "-"):
			descending = false
			name, _, _ := strings.Cut(strings.TrimLeft(w, "-"), "=")
			var found bool
			if strings.HasPrefix(w, "--") {
				found = cmd.Flag(name) != nil
			} else {
				found = len(name) == 1 && cmd.Flags().ShorthandLookup(name) != nil
			}
			if !found {
				problems = append(problems, fmt.Sprintf("%q: %s has no flag %s", line, cmd.CommandPath(), w))
			}
		case descending:
			next := find(cmd, w)
			switch {
			case next != nil:
				cmd = next
			case cmd.HasSubCommands():
				return append(problems, fmt.Sprintf("%q: %s has no command %s", line, cmd.CommandPath(), w))
			default:
				descending = false // a value of the command
			}
		}
	}
	return problems
}

// find returns the subcommand of cmd named name, or nil.
func find(cmd *cobra.Command, name string) *cobra.Command {
	for _, c := range cmd.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
