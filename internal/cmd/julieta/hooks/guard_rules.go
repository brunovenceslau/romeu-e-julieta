// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// The three patterns of julieta hooks guard (06 6.4, REQ-0598), each a
// rule by name.
const (
	ruleMerge    = "gh-pr-merge"
	ruleTag      = "tag-push"
	ruleSettings = "gh-api-settings"
)

// refusal is the verdict of the guard on a command: the rule it hit and
// the text of 06 6.4, which says the action is the maintainer's and
// names the route.
type refusal struct {
	rule, text string
}

// The route the refusal text names: an issue, or the handoff.
const route = " Ask the maintainer for it in an issue, or write it into the handoff (julieta handoff write), and go on with the rest of the work."

// The refusals, by rule.
var refusals = map[string]refusal{
	ruleMerge:    {ruleMerge, "julieta hooks guard refused gh pr merge: merging a pull request is the maintainer's action, not the agent's." + route},
	ruleTag:      {ruleTag, "julieta hooks guard refused a push of a v* tag: tagging a release is the maintainer's action, not the agent's." + route},
	ruleSettings: {ruleSettings, "julieta hooks guard refused a gh api call that changes rulesets or repository settings: that change is the maintainer's action, not the agent's." + route},
}

// guardCheck returns the refusal of the first command of the shell text
// command that hits a rule, or nil.
//
// The patterns are matched on the words of each simple command, split
// by the quoting of sh, as a guard against mistakes and not against an
// agent that hides a command (05 5.4): a maintained shell parser,
// mvdan.cc/sh, would read more of the grammar, and is a dependency the
// three patterns do not need.
func guardCheck(command string) *refusal {
	return checkText(command, 0)
}

// maxDepth bounds the nesting of sh -c and eval the check follows.
const maxDepth = 4

// checkText checks each simple command of text.
func checkText(text string, depth int) *refusal {
	for _, words := range splitCommands(text) {
		if r := checkWords(words, depth); r != nil {
			return r
		}
	}
	return nil
}

// prefixes are the words that run the command after them: the
// reserved words a command can follow and the common wrappers, each
// with the flags of it that take a value.
var prefixes = map[string][]string{
	"{": nil, "!": nil, "then": nil, "do": nil, "else": nil, "if": nil, "elif": nil, "while": nil, "until": nil,
	"time": nil, "exec": nil, "command": nil, "nohup": nil, "builtin": nil,
	"sudo":    {"-u", "-g", "-C", "-D", "-h", "-p", "-U", "-r", "-t", "-T"},
	"env":     {"-u", "-C", "-S"},
	"nice":    {"-n"},
	"timeout": {"-s", "-k"},
	"xargs":   {"-n", "-I", "-L", "-P", "-d", "-E", "-s", "-a"},
}

// assignment is a word that sets a variable before a command.
var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// checkWords checks one simple command.
func checkWords(words []string, depth int) *refusal {
	for len(words) > 0 {
		w := words[0]
		valued, ok := prefixes[w]
		switch {
		case assignment.MatchString(w):
			words = words[1:]
			continue
		case !ok:
		default:
			words = skipFlags(words[1:], valued)
			if w == "timeout" && len(words) > 0 {
				words = words[1:] // the duration
			}
			continue
		}
		break
	}
	if len(words) == 0 {
		return nil
	}
	switch name := path.Base(words[0]); name {
	case "sh", "bash", "dash", "zsh", "ash":
		for i, w := range words[1:] {
			if strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.Contains(w, "c") && i+2 < len(words) && depth < maxDepth {
				return checkText(words[i+2], depth+1)
			}
		}
	case "eval":
		if depth < maxDepth {
			return checkText(strings.Join(words[1:], " "), depth+1)
		}
	case "gh":
		return checkGh(words[1:])
	case "git":
		return checkGit(words[1:])
	}
	return nil
}

// skipFlags returns words after the flags at their start, each flag of
// valued taking the next word as its value.
func skipFlags(words, valued []string) []string {
	for len(words) > 0 && strings.HasPrefix(words[0], "-") {
		if words[0] == "--" {
			return words[1:]
		}
		if slices.Contains(valued, words[0]) && len(words) > 1 {
			words = words[1:]
		}
		words = words[1:]
	}
	return words
}

// positional returns the words of args that are not flags, a flag of
// valued taking the next word as its value.
func positional(args, valued []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return append(out, args[i+1:]...)
		case strings.HasPrefix(a, "-") && len(a) > 1:
			if slices.Contains(valued, a) {
				i++
			}
		default:
			out = append(out, a)
		}
	}
	return out
}

// checkGh checks the arguments of gh.
func checkGh(args []string) *refusal {
	pos := positional(args, ghValued)
	switch {
	case len(pos) >= 2 && pos[0] == "pr" && pos[1] == "merge":
		r := refusals[ruleMerge]
		return &r
	case len(pos) >= 1 && pos[0] == "api":
		return checkAPI(args)
	}
	return nil
}

// ghValued are the flags of gh and of gh api that take a value as the
// next word.
var ghValued = []string{"-R", "--repo", "-X", "--method", "-H", "--header", "-f", "--raw-field", "-F", "--field",
	"--input", "-q", "--jq", "-t", "--template", "--hostname", "--cache", "-p", "--preview"}

// The flags of gh api that send fields, which make the default method
// POST.
var ghFields = []string{"-f", "--raw-field", "-F", "--field", "--input"}

// checkAPI checks a call of gh api: one that changes a ruleset or a
// repository's settings, by a method other than GET, or a GraphQL
// mutation of them.
func checkAPI(args []string) *refusal {
	pos := positional(args, ghValued)
	if len(pos) < 2 {
		return nil
	}
	endpoint := strings.Trim(pos[1], "/")
	if endpoint == "graphql" {
		text := strings.Join(args, " ")
		if strings.Contains(text, "mutation") && graphqlSettings.MatchString(text) {
			r := refusals[ruleSettings]
			return &r
		}
		return nil
	}
	if apiMethod(args) == "GET" || !settingsPath(endpoint) {
		return nil
	}
	r := refusals[ruleSettings]
	return &r
}

// graphqlSettings names the GraphQL mutations of rulesets, branch
// protection and a repository's settings.
var graphqlSettings = regexp.MustCompile(`(?i)ruleset|BranchProtectionRule|updateRepository\b`)

// apiMethod returns the method of a gh api call: the one -X or
// --method names, else POST when it sends a field, else GET.
func apiMethod(args []string) string {
	method := ""
	fields := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case (a == "-X" || a == "--method") && i+1 < len(args):
			method = args[i+1]
			i++
		case strings.HasPrefix(a, "--method="):
			method = strings.TrimPrefix(a, "--method=")
		case strings.HasPrefix(a, "-X") && len(a) > 2:
			method = a[2:]
		case slices.Contains(ghFields, a) || slices.ContainsFunc(ghFields, func(f string) bool {
			return strings.HasPrefix(f, "--") && strings.HasPrefix(a, f+"=")
		}):
			fields = true
		}
	}
	switch {
	case method != "":
		return strings.ToUpper(method)
	case fields:
		return "POST"
	}
	return "GET"
}

// settingsSegments are the first segments, after repos/<owner>/<repo>,
// of the endpoints that hold a repository's settings.
var settingsSegments = []string{
	"rulesets", "actions", "environments", "collaborators", "hooks", "keys", "pages", "topics", "transfer",
	"vulnerability-alerts", "automated-security-fixes", "private-vulnerability-reporting", "interaction-limits",
	"autolinks", "dependabot", "code-scanning", "secret-scanning", "immutable-releases", "properties",
}

// settingsPath reports whether the endpoint is a ruleset, of a
// repository or an organization, or a repository's settings: the
// repository itself, one of settingsSegments, or a branch's protection.
func settingsPath(endpoint string) bool {
	segs := strings.Split(endpoint, "/")
	if slices.Contains(segs, "rulesets") {
		return true
	}
	if len(segs) < 3 || segs[0] != "repos" {
		return false
	}
	rest := segs[3:]
	switch {
	case len(rest) == 0:
		return true
	case rest[0] == "branches":
		return len(rest) >= 3 && rest[2] == "protection"
	}
	return slices.Contains(settingsSegments, rest[0])
}

// gitValued are the options of git before its command that take the
// next word as their value.
var gitValued = []string{"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--super-prefix", "--config-env", "--exec-path"}

// pushValued are the options of git push that take the next word as
// their value.
var pushValued = []string{"-o", "--push-option", "--receive-pack", "--exec", "--repo"}

// checkGit checks the arguments of git: a push of a v* tag, named as a
// ref, after tag, or as a bare name of v and a digit, which reads as a
// tag; or of every tag, by --tags, --follow-tags or --mirror.
func checkGit(args []string) *refusal {
	rest := skipFlags(args, gitValued)
	if len(rest) == 0 || rest[0] != "push" {
		return nil
	}
	refused := refusals[ruleTag]
	push := rest[1:]
	for _, a := range push {
		if a == "--" {
			break
		}
		if a == "--tags" || a == "--follow-tags" || a == "--mirror" {
			return &refused
		}
	}
	pos := positional(push, pushValued)
	if len(pos) < 2 {
		return nil
	}
	refs := pos[1:] // the first is the remote
	for i := 0; i < len(refs); i++ {
		if refs[i] == "tag" && i+1 < len(refs) {
			if strings.HasPrefix(refs[i+1], "v") {
				return &refused
			}
			i++
			continue
		}
		src, dst, _ := strings.Cut(strings.TrimPrefix(refs[i], "+"), ":")
		if vTag(src) || vTag(dst) {
			return &refused
		}
	}
	return nil
}

// bareVTag is a bare name that reads as a release tag.
var bareVTag = regexp.MustCompile(`^v[0-9]`)

// vTag reports whether one side of a refspec names a v* tag.
func vTag(ref string) bool {
	return strings.HasPrefix(ref, "refs/tags/v") || bareVTag.MatchString(ref)
}

// splitCommands splits shell text into simple commands, each a list of
// words, by the quoting of sh: outside single quotes a backslash
// escapes the next character; ; & | ( ) and a newline end a command
// outside quotes; and a command substitution, $( or a backquote, starts
// one, inside double quotes too, since sh runs it there. An unclosed
// quote runs to the end of the text.
func splitCommands(text string) [][]string {
	var (
		cmds  [][]string
		words []string
		word  strings.Builder
		inW   bool // a word is open, maybe empty ("")
		dq    bool // inside double quotes
	)
	endWord := func() {
		if inW {
			words = append(words, word.String())
		}
		word.Reset()
		inW = false
	}
	endCmd := func() {
		endWord()
		if len(words) > 0 {
			cmds = append(cmds, words)
		}
		words = nil
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '\\' && i+1 < len(text):
			i++
			word.WriteByte(text[i])
			inW = true
		case c == '`' || (c == '$' && i+1 < len(text) && text[i+1] == '('):
			endCmd()
			dq = false
			if c == '$' {
				i++
			}
		case dq && c == '"':
			dq = false
		case dq:
			word.WriteByte(c)
		case c == '"':
			dq, inW = true, true
		case c == '\'':
			j := strings.IndexByte(text[i+1:], '\'')
			if j < 0 {
				j = len(text) - i - 1
			}
			word.WriteString(text[i+1 : i+1+j])
			inW = true
			i += j + 1
		case c == ';' || c == '&' || c == '|' || c == '(' || c == ')' || c == '\n':
			endCmd()
		case c == ' ' || c == '\t':
			endWord()
		default:
			word.WriteByte(c)
			inW = true
		}
	}
	endCmd()
	return cmds
}
