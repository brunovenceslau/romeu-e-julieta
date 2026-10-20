// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli_test

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	"github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
)

// skillsDir is skills/ seen from this package.
const skillsDir = "../../skills"

// TestSkillsNameRealCommands reads every file of every skills/*/ and
// fails on a julieta command or flag one names that the command
// definitions lack, on a reference SKILL.md does not link or a link to
// no file, and on headings of the handoff skill that differ from those
// `julieta handoff write` requires (12 12.3, REQ-1115). The headings
// come from handoff.Headings, the list CheckHeadings enforces.
func TestSkillsNameRealCommands(t *testing.T) {
	tree := cli.Tree(&cli.Env{Binary: "julieta"})
	dirs, err := os.ReadDir(skillsDir)
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, f := range dirs {
		if !f.IsDir() {
			continue
		}
		files := map[string]string{}
		dir := path.Join(skillsDir, f.Name())
		require.NoError(t, fs.WalkDir(os.DirFS(dir), ".", func(p string, d fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case p != "." && strings.HasPrefix(d.Name(), "."):
				// an editor's or the OS's file (.DS_Store, a swap file), not the skill's
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			case d.IsDir():
				return nil
			}
			b, err := os.ReadFile(path.Join(dir, p))
			files[p] = string(b)
			return err
		}))
		require.Contains(t, files, "SKILL.md", "skills/%s needs a SKILL.md", f.Name())
		seen[f.Name()] = true
		assert.Empty(t, checkSkill(tree, f.Name(), files), "skills/%s", f.Name())
	}
	assert.True(t, seen["julieta"], "skills/julieta/SKILL.md is missing")
	assert.True(t, seen["handoff"], "skills/handoff/SKILL.md is missing")
}

// TestSkillsFixtures holds checkSkill to what it promises: a whole skill
// passes, and each defect is named.
func TestSkillsFixtures(t *testing.T) {
	tree := cli.Tree(&cli.Env{Binary: "julieta"})
	clearHeadings := "## Goal\n## State\n## Decisions\n## Next steps\n## Open questions\n"
	finalHeadings := clearHeadings + "## What would be lost\n"
	front := "---\nname: handoff\ndescription: d\n---\n"
	block := func(kind, headings string) string { return "```markdown kind=" + kind + "\n" + headings + "```\n" }
	blocks := block("clear", clearHeadings) + block("final", finalHeadings)
	skill := func(body string) string { return front + body + blocks }
	whole := skill("`julieta memory add --title t --tag lesson`\n" +
		"```sh\njulieta handoff write --final <<'EOF'\nEOF\n```\n")

	tests := []struct {
		name string
		text string
		want string            // a substring of the problem; empty means none
		refs map[string]string // files besides SKILL.md
	}{
		{"a whole skill", whole, "", nil},
		{"an unknown flag", strings.Replace(whole, "--final <<", "--finale <<", 1), "has no flag --finale", nil},
		{"an unknown flag in a span", skill("`julieta memory add --nope`\n"), "has no flag --nope", nil},
		{"an unknown flag in a double backtick span", skill("``julieta memory add --nope``\n"), "has no flag --nope", nil},
		{"an unknown command", strings.Replace(whole, "julieta memory add", "julieta remember", 1), "has no command remember", nil},
		{"an unknown subcommand", skill("`julieta memory zap`\n"), "has no command zap", nil},
		{"an unknown subcommand after a flag", skill("`julieta memory -h zap`\n"), "has no command zap", nil},
		{"an unknown flag after a quoted pipe", skill("`julieta memory add --body 'a | b' --nope`\n"), "has no flag --nope", nil},
		{"a quoted dash is a value", skill("`julieta memory add --title 'a - b' --tag lesson`\n"), "", nil},
		{"a flag value is skipped", skill("`julieta memory add --title -x --tag lesson`\n"), "", nil},
		{"a placeholder is a value", skill("`julieta memory show <id> --repo=<path>`\n"), "", nil},
		{"an unknown flag in brackets", skill("`julieta handoff write [--finale]`\n"), "has no flag --finale", nil},
		{"an unknown flag on a continued line", skill("```sh\njulieta memory add \\\n  --nope\n```\n"), "has no flag --nope", nil},
		{"an unknown flag in a tilde block", skill("~~~sh\njulieta memory add --nope\n~~~\n"), "has no flag --nope", nil},
		{"a longer fence holds a shorter one", skill("````markdown\n```sh\n````\n`julieta memory add --nope`\n"), "has no flag --nope", nil},
		{"an unclosed block", front + blocks + "```sh\njulieta memory add\n", "never closed", nil},
		{"a missing heading", strings.Replace(whole, "## State\n## Decisions\n## Next steps\n## Open questions\n## What", "## State\n## Next steps\n## Open questions\n## What", 1), "requires", nil},
		{"an extra heading", strings.Replace(whole, "## State\n", "## State\n## Risks\n", 1), "Risks\"", nil},
		{"a missing block", front + block("clear", clearHeadings), "final", nil},
		{"a repeated block", whole + block("clear", clearHeadings), "twice", nil},
		{"an unknown kind", whole + block("facts", ""), "facts", nil},
		{"no front matter", strings.TrimPrefix(whole, front), "front matter", nil},
		{"a name in the body only", strings.Replace(whole, "name: handoff\n", "", 1) + "name: handoff\n", "name", nil},
		{"another name", strings.Replace(whole, "name: handoff", "name: hand-off", 1), "hand-off", nil},
		{"no description", strings.Replace(whole, "description: d\n", "", 1), "description", nil},
		{"a block in a reference", front + "See [clear](references/clear.md).\n" + block("final", finalHeadings), "", map[string]string{"references/clear.md": block("clear", clearHeadings)}},
		{"a command in a reference", front + "See [m](references/m.md).\n" + blocks, "has no flag --nope", map[string]string{"references/m.md": "`julieta memory add --nope`\n"}},
		{"an unlinked reference", whole, "references/m.md is linked from no line of SKILL.md", map[string]string{"references/m.md": "text\n"}},
		{"a link to no file", skill("See [m](references/m.md).\n"), "links to references/m.md, which is not in the skill", nil},
		{"a body of 499 lines", front + blocks + strings.Repeat("text\n", 499-strings.Count(blocks, "\n")), "", nil},
		{"a body of 500 lines", front + blocks + strings.Repeat("text\n", 500-strings.Count(blocks, "\n")), "keep it under 500", nil},
		{"a link with ./ and an anchor", skill("See [m](./references/m.md#top).\n"), "", map[string]string{"references/m.md": "text\n"}},
		{"a link out of the skill", skill("See [j](../julieta/SKILL.md).\n"), "links to ../julieta/SKILL.md, which is not in the skill", nil},
		{"a link only in a code block", skill("```markdown\nSee [m](references/m.md).\n```\n"), "references/m.md is linked from no line of SKILL.md", map[string]string{"references/m.md": "text\n"}},
		{"a link only in a code span", skill("`[m](references/m.md)`\n"), "references/m.md is linked from no line of SKILL.md", map[string]string{"references/m.md": "text\n"}},
		{"a description too long", strings.Replace(whole, "description: d", "description: "+strings.Repeat("d", 1025), 1), "description", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := tt.refs
			if files == nil {
				files = map[string]string{}
			}
			files["SKILL.md"] = tt.text
			got := strings.Join(checkSkill(tree, "handoff", files), "\n")
			if tt.want == "" {
				assert.Empty(t, got)
				return
			}
			assert.Contains(t, got, tt.want)
		})
	}
}

// skillName is the form Claude Code takes for a skill's name.
var skillName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// checkSkill returns the problems of the skill named name, whose files
// maps each path below its directory to its text: a front matter of
// SKILL.md without that name or without a description, a SKILL.md body
// of 500 lines or more, a file of the skill SKILL.md does not link, a
// link of SKILL.md to no file of the skill, and the problems scanFile
// finds in each file. For the skill "handoff", the kind blocks of all
// its files together must name the headings handoff.Headings gives.
// SKILL.md links each reference so the agent reads it only when needed
// (progressive disclosure), and a file it does not link is never read.
func checkSkill(tree *cobra.Command, name string, files map[string]string) []string {
	problems, body := frontMatterProblems(name, splitLines(files["SKILL.md"]))
	if len(body) >= 500 {
		problems = append(problems, fmt.Sprintf("the body of SKILL.md has %d lines, keep it under 500 and move detail to references/", len(body)))
	}
	blocks := map[string][]string{} // kind -> headings of its block
	scanned, links := scanFile(tree, "SKILL.md", body, blocks)
	problems = append(problems, scanned...)
	linked := map[string]bool{}
	for _, target := range links {
		linked[target] = true
		if _, ok := files[target]; !ok {
			problems = append(problems, fmt.Sprintf("SKILL.md links to %s, which is not in the skill", target))
		}
	}
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if p == "SKILL.md" {
			continue
		}
		if !linked[p] {
			problems = append(problems, p+" is linked from no line of SKILL.md")
		}
		scanned, _ := scanFile(tree, p, splitLines(files[p]), blocks)
		problems = append(problems, scanned...)
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

// splitLines returns the lines of text, without the empty element a
// final newline leaves.
func splitLines(text string) []string {
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// relativeLink matches a Markdown link to a path, not to a URL or an
// anchor of the same file; a link with a title does not match, so the
// file it names counts as not linked.
var relativeLink = regexp.MustCompile(`\]\(([^)#\s:]+)(?:#[^)]*)?\)`)

// scanFile returns the problems of the lines of the file p: a julieta
// command or flag that tree lacks, a code block never closed, and a kind
// block that comes twice in the skill; it adds the headings of each kind
// block to blocks, and returns the relative links of the prose, outside
// code blocks and code spans, as cleaned paths. A line scan, not a Markdown parser: a command is a
// line of a code block, joined with the lines a trailing backslash
// continues, or a code span, that starts with "julieta".
func scanFile(tree *cobra.Command, p string, lines []string, blocks map[string][]string) ([]string, []string) {
	var problems, links []string
	kind, fence := "", "" // fence: the run that opened the block, "" outside one
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if run := fenceRun(trimmed); run != "" {
			info := trimmed[len(run):]
			switch {
			case fence == "":
				fence, kind = run, ""
				for f := range strings.FieldsSeq(info) {
					if k, ok := strings.CutPrefix(f, "kind="); ok {
						if _, dup := blocks[k]; dup {
							problems = append(problems, fmt.Sprintf("a block kind=%s comes twice", k))
						}
						kind, blocks[k] = k, []string{}
					}
				}
				continue
			case run[0] == fence[0] && len(run) >= len(fence) && strings.TrimSpace(info) == "":
				fence, kind = "", ""
				continue
			}
		}
		switch {
		case kind != "":
			if h, ok := strings.CutPrefix(trimmed, "## "); ok {
				blocks[kind] = append(blocks[kind], h)
			}
		case fence != "":
			for strings.HasSuffix(trimmed, `\`) && i+1 < len(lines) {
				i++
				trimmed = strings.TrimSuffix(trimmed, `\`) + " " + strings.TrimSpace(lines[i])
			}
			problems = append(problems, commandProblems(tree, trimmed)...)
		default:
			prose := lines[i]
			for span := range codeSpans(lines[i]) {
				problems = append(problems, commandProblems(tree, span)...)
				prose = strings.Replace(prose, span, "", 1)
			}
			for _, m := range relativeLink.FindAllStringSubmatch(prose, -1) {
				links = append(links, path.Clean(m[1]))
			}
		}
	}
	if fence != "" {
		problems = append(problems, fmt.Sprintf("%s: a code block opened with %s is never closed", p, fence))
	}
	return problems, links
}

// frontMatterProblems decodes the front matter, the lines between the
// first line "---" and the next, and returns its problems and the lines
// after it. Claude Code needs a name, lowercase words joined by hyphens
// of at most 64 characters, and a description of at most 1024; the name
// is the skill's directory.
func frontMatterProblems(name string, lines []string) ([]string, []string) {
	end := -1
	if len(lines) > 0 && lines[0] == "---" {
		end = slices.Index(lines[1:], "---") + 1
	}
	if end < 1 {
		return []string{"no front matter between two lines ---"}, lines
	}
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &fm); err != nil {
		return []string{"the front matter: " + err.Error()}, lines[end+1:]
	}
	var problems []string
	if fm.Name != name || !skillName.MatchString(fm.Name) || len(fm.Name) > 64 {
		problems = append(problems, fmt.Sprintf("the front matter has name %q, wants %q", fm.Name, name))
	}
	if d := strings.TrimSpace(fm.Description); d == "" || len(d) > 1024 {
		problems = append(problems, fmt.Sprintf("the front matter needs a description of 1 to 1024 characters, has %d", len(d)))
	}
	return problems, lines[end+1:]
}

// fenceRun returns the run of three or more backticks or tildes that
// starts line, or "".
func fenceRun(line string) string {
	if line == "" || (line[0] != '`' && line[0] != '~') {
		return ""
	}
	n := len(line) - len(strings.TrimLeft(line, line[:1]))
	if n < 3 {
		return ""
	}
	return line[:n]
}

// codeSpans yields the code spans of line: a run of backticks opens one,
// and the next run of the same length closes it.
func codeSpans(line string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for {
			open := strings.IndexByte(line, '`')
			if open < 0 {
				return
			}
			n := len(line[open:]) - len(strings.TrimLeft(line[open:], "`"))
			rest := line[open+n:]
			closeAt := -1
			for i := 0; i < len(rest); {
				if rest[i] != '`' {
					i++
					continue
				}
				m := len(rest[i:]) - len(strings.TrimLeft(rest[i:], "`"))
				if m == n {
					closeAt = i
					break
				}
				i += m
			}
			if closeAt < 0 {
				return
			}
			if !yield(strings.TrimSpace(rest[:closeAt])) {
				return
			}
			line = rest[closeAt+n:]
		}
	}
}

// commandProblems returns the problems of one line or span if it is a
// julieta command line: the first word that is no command of the tree,
// and each flag the command does not have. A flag that takes a value
// skips the next word; other words after the command path are values, as
// are quoted words and placeholders such as <id>; a flag in brackets,
// [--final], is checked as the flag. The line ends at an unquoted pipe,
// redirect or list separator, and at "--".
func commandProblems(tree *cobra.Command, line string) []string {
	words := shellWords(strings.TrimPrefix(line, "$ "))
	if len(words) < 2 || words[0].quoted || words[0].text != "julieta" {
		return nil
	}
	cmd, descending, value := tree, true, false
	var problems []string
	for _, w := range words[1:] {
		text := w.text
		if !w.quoted && strings.HasPrefix(text, "[") {
			text = strings.TrimSuffix(strings.TrimPrefix(text, "["), "]")
		}
		switch {
		case value:
			value = false
		case w.quoted || strings.HasPrefix(text, "<"):
			descending = false
		case text == "--":
			return problems
		case strings.HasPrefix(text, "-"):
			f := lookupFlag(cmd, text)
			if f == nil {
				problems = append(problems, fmt.Sprintf("%q: %s has no flag %s", line, cmd.CommandPath(), text))
				continue
			}
			value = f.NoOptDefVal == "" && !strings.Contains(text, "=")
		case descending:
			next := find(cmd, text)
			switch {
			case next != nil:
				cmd = next
			case cmd.HasSubCommands():
				return append(problems, fmt.Sprintf("%q: %s has no command %s", line, cmd.CommandPath(), text))
			default:
				descending = false // a value of the command
			}
		}
	}
	return problems
}

// lookupFlag returns the flag of cmd, its own or inherited, that word
// names (--name, --name=value or -n), or nil.
func lookupFlag(cmd *cobra.Command, word string) *pflag.Flag {
	name, _, _ := strings.Cut(strings.TrimLeft(word, "-"), "=")
	if strings.HasPrefix(word, "--") {
		return cmd.Flag(name)
	}
	if len(name) != 1 {
		return nil
	}
	if f := cmd.Flags().ShorthandLookup(name); f != nil {
		return f
	}
	return cmd.InheritedFlags().ShorthandLookup(name)
}

// word is one word of a command line, and whether any of it was quoted.
type word struct {
	text   string
	quoted bool
}

// shellWords splits line into words the way a POSIX shell treats quotes
// and backslashes, and stops at the first unquoted |, ;, &, < or >, so a
// quoted value can neither end the command nor pass for a flag. A
// placeholder such as <id> or --repo=<path> is part of a word, not a
// redirect. Written here rather than imported: no shell-words package is
// a dependency, and the test needs only quotes and operators.
func shellWords(line string) []word {
	var words []word
	var cur strings.Builder
	in, quoted := false, false // inside a word; the word has a quoted part
	flush := func() {
		if in {
			words = append(words, word{cur.String(), quoted})
		}
		cur.Reset()
		in, quoted = false, false
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == ' ' || c == '\t':
			flush()
		case c == '\'':
			j := strings.IndexByte(line[i+1:], '\'')
			if j < 0 {
				j = len(line) - i - 1
			}
			cur.WriteString(line[i+1 : i+1+j])
			i += j + 1
			in, quoted = true, true
		case c == '"':
			for i++; i < len(line) && line[i] != '"'; i++ {
				if line[i] == '\\' && i+1 < len(line) {
					i++
				}
				cur.WriteByte(line[i])
			}
			in, quoted = true, true
		case c == '\\' && i+1 < len(line):
			i++
			cur.WriteByte(line[i])
			in = true
		case c == '<' && isPlaceholder(line[i:]):
			end := strings.IndexAny(line[i:], " \t")
			if end < 0 {
				end = len(line) - i
			}
			cur.WriteString(line[i : i+end])
			i += end - 1
			in = true
		case strings.IndexByte("|;&<>", c) >= 0:
			flush()
			return words
		default:
			cur.WriteByte(c)
			in = true
		}
	}
	flush()
	return words
}

// isPlaceholder reports whether s starts with a placeholder, <name>,
// that runs to the next blank or the end.
func isPlaceholder(s string) bool {
	if end := strings.IndexAny(s, " \t"); end >= 0 {
		s = s[:end]
	}
	return len(s) > 2 && s[1] != '<' && strings.HasSuffix(s, ">")
}

// find returns the subcommand of cmd named name, or one of its aliases,
// or nil.
func find(cmd *cobra.Command, name string) *cobra.Command {
	for _, c := range cmd.Commands() {
		if c.Name() == name || c.HasAlias(name) {
			return c
		}
	}
	return nil
}
