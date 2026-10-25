// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// The patterns of julieta hooks guard (06 6.4, REQ-0598), each a rule
// by name, and the verdict on a command the parser refuses.
const (
	ruleMerge      = "gh-pr-merge"
	ruleTag        = "tag-push"
	ruleSettings   = "gh-api-settings"
	ruleRelease    = "gh-release"
	ruleUnparsable = "unparsable"
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
	ruleMerge:    {ruleMerge, "julieta hooks guard refused a merge of a pull request (gh pr merge, or gh api): merging is the maintainer's action, not the agent's." + route},
	ruleTag:      {ruleTag, "julieta hooks guard refused a push of a v* tag: tagging a release is the maintainer's action, not the agent's." + route},
	ruleSettings: {ruleSettings, "julieta hooks guard refused a gh api call that changes rulesets or repository settings: that change is the maintainer's action, not the agent's." + route},
	ruleRelease:  {ruleRelease, "julieta hooks guard refused the creation or deletion of a release: releasing is the maintainer's action, not the agent's." + route},
}

// refuse returns the refusal of rule.
func refuse(rule string) *refusal {
	r := refusals[rule]
	return &r
}

// guardCheck returns the refusal of the first simple command of the
// shell text command that hits a rule, or nil. A text the parser
// refuses is refused too: the guard fails closed.
//
// The text is parsed with mvdan.cc/sh/v3/syntax, as bash reads it, and
// every simple command of the tree is checked: those of a list, a
// pipeline, a subshell, a function body, a command substitution or
// backquotes, and one inside an unquoted heredoc's substitution, which
// runs; a heredoc's text and a comment are no command. The words of a
// wrapper (sudo, env, xargs, timeout, ...) are followed, and the string
// of sh -c and of eval is parsed again. It is a guard against mistakes,
// not against an agent that hides a command (05 5.4).
func guardCheck(command string) *refusal {
	return checkText(command, 0)
}

// maxDepth bounds the nesting of sh -c and eval the check follows.
const maxDepth = 4

// checkText parses text and checks each simple command of it.
func checkText(text string, depth int) *refusal {
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(text), "")
	if err != nil {
		return &refusal{ruleUnparsable, fmt.Sprintf("julieta hooks guard refused the tool call: its command cannot be parsed as a shell script (%v), and the guard fails closed.", err)}
	}
	var found *refusal
	syntax.Walk(f, func(n syntax.Node) bool {
		if found != nil {
			return false
		}
		if call, ok := n.(*syntax.CallExpr); ok && len(call.Args) > 0 {
			words := make([]string, len(call.Args))
			for i, w := range call.Args {
				words[i] = wordText(w)
			}
			found = checkWords(words, depth)
		}
		return found == nil
	})
	return found
}

// expansion stands in a word for a part the shell expands at run time:
// a parameter, a command substitution, an arithmetic expansion.
const expansion = "\x00"

// wordText returns the text of a word as the command receives it,
// quotes removed and escapes applied, each expansion written as
// expansion.
func wordText(w *syntax.Word) string {
	var b strings.Builder
	for _, p := range w.Parts {
		partText(&b, p, false)
	}
	return b.String()
}

// partText writes the text of one part of a word to b; dq says the part
// is inside double quotes.
func partText(b *strings.Builder, p syntax.WordPart, dq bool) {
	switch p := p.(type) {
	case *syntax.Lit:
		b.WriteString(unescape(p.Value, dq))
	case *syntax.SglQuoted:
		b.WriteString(p.Value)
	case *syntax.DblQuoted:
		for _, q := range p.Parts {
			partText(b, q, true)
		}
	default:
		b.WriteString(expansion)
	}
}

// unescape removes the backslashes the shell removes from a literal:
// before any character outside double quotes, and before $ ` " \ inside.
func unescape(s string, dq bool) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (!dq || strings.IndexByte("$`\"\\", s[i+1]) >= 0) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
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
		return refuse(ruleMerge)
	case len(pos) >= 2 && pos[0] == "release" && (pos[1] == "create" || pos[1] == "delete"):
		return refuse(ruleRelease)
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

// checkAPI checks a call of gh api, by a method other than GET: a merge
// of a pull request, a tag ref or a release created, changed or deleted,
// or a change of a ruleset or a repository's settings; or a GraphQL
// mutation of one of them.
func checkAPI(args []string) *refusal {
	pos := positional(args, ghValued)
	if len(pos) < 2 {
		return nil
	}
	endpoint, _, _ := strings.Cut(pos[1], "?")
	endpoint = strings.Trim(endpoint, "/")
	if endpoint == "graphql" {
		text := strings.Join(args, " ")
		switch {
		case !strings.Contains(text, "mutation"):
		case graphqlMerge.MatchString(text):
			return refuse(ruleMerge)
		case graphqlSettings.MatchString(text):
			return refuse(ruleSettings)
		}
		return nil
	}
	if apiMethod(args) == "GET" {
		return nil
	}
	segs := strings.Split(endpoint, "/")
	if len(segs) >= 4 && segs[0] == "repos" {
		switch rest := segs[3:]; {
		case len(rest) == 3 && rest[0] == "pulls" && rest[2] == "merge":
			return refuse(ruleMerge)
		case rest[0] == "releases":
			return refuse(ruleRelease)
		case len(rest) >= 2 && rest[0] == "git" && rest[1] == "refs" &&
			(len(rest) >= 4 && rest[2] == "tags" && strings.HasPrefix(rest[3], "v") ||
				slices.ContainsFunc(args, func(a string) bool { return strings.Contains(a, "refs/tags/v") })):
			return refuse(ruleTag)
		}
	}
	if settingsPath(endpoint) {
		return refuse(ruleSettings)
	}
	return nil
}

// graphqlSettings names the GraphQL mutations of rulesets, branch
// protection and a repository's settings; graphqlMerge those that merge
// a pull request or arm its auto-merge.
var (
	graphqlSettings = regexp.MustCompile(`(?i)ruleset|BranchProtectionRule|updateRepository\b`)
	graphqlMerge    = regexp.MustCompile(`mergePullRequest|enablePullRequestAutoMerge`)
)

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
			if strings.HasPrefix(f, "--") {
				return strings.HasPrefix(a, f+"=")
			}
			return strings.HasPrefix(a, f) // -fname=x, the value attached
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
	"rulesets", "environments", "collaborators", "hooks", "keys", "pages", "topics", "transfer",
	"vulnerability-alerts", "automated-security-fixes", "private-vulnerability-reporting", "interaction-limits",
	"autolinks", "dependabot", "code-scanning", "secret-scanning", "immutable-releases", "properties",
}

// actionsSettings are the segments after actions/ that hold settings,
// not runs: a rerun or a workflow dispatch is the agent's.
var actionsSettings = []string{"secrets", "variables", "permissions", "runners", "oidc"}

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
	case rest[0] == "actions":
		return len(rest) >= 2 && slices.Contains(actionsSettings, rest[1])
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
// ref of refs/tags/ (a glob there too), after tag, or as a bare name of
// the form of a release version; or of every tag, by --tags,
// --follow-tags or --mirror.
func checkGit(args []string) *refusal {
	rest := skipFlags(args, gitValued)
	if len(rest) == 0 || rest[0] != "push" {
		return nil
	}
	push := rest[1:]
	for _, a := range push {
		if a == "--" {
			break
		}
		if a == "--tags" || a == "--follow-tags" || a == "--mirror" {
			return refuse(ruleTag)
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
				return refuse(ruleTag)
			}
			i++
			continue
		}
		src, dst, _ := strings.Cut(strings.TrimPrefix(refs[i], "+"), ":")
		if vTag(src) || vTag(dst) {
			return refuse(ruleTag)
		}
	}
	return nil
}

// releaseVersion is a bare name of the form of a release tag: v and a
// version, v1, v1.2.3, v1.2.3-rc.1 or v1.2.3+build, and not a branch
// such as v2-fix.
var releaseVersion = regexp.MustCompile(`^v[0-9]+((\.[0-9]+)+([-+][0-9A-Za-z.-]+)?)?$`)

// vTag reports whether one side of a refspec names a v* tag: a ref of
// refs/tags/v, or a glob of refs/tags/, which may match one; a bare
// release version; or a bare v followed by an expansion, a version the
// shell fills in.
func vTag(ref string) bool {
	if tag, ok := strings.CutPrefix(ref, "refs/tags/"); ok {
		return strings.HasPrefix(tag, "v") || strings.ContainsAny(tag, "*?[")
	}
	return releaseVersion.MatchString(ref) ||
		strings.HasPrefix(ref, "v") && strings.Contains(ref, expansion)
}
