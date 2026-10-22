// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// The validation rules of 03 3.1, 3.2 and 3.4 as data (REQ-0172). Each
// rule names a field path, its constraint, a message, its error and its
// scope. The validators of this package apply them (validate.go), the
// JSON Schemas carry the constraints JSON Schema can say (SchemaKeywords)
// and the reference pages print them as the rule table, so the four
// cannot drift.

// Format names a format of this package, as its schema name.
type Format string

// The formats of this package.
const (
	FormatProject      Format = "project"
	FormatHostSettings Format = "host-settings"
)

// Scope says which validators check a rule (03 3.1, REQ-0174).
type Scope string

// The scopes of a rule.
const (
	// Both: julieta spec validate in the config repo's CI and romeu.
	Both Scope = "both"
	// RomeuOnly: romeu alone, since the rule reads host settings, which
	// the config repo's CI cannot see.
	RomeuOnly Scope = "romeu only"
)

// Rule is one validation rule. Field is a path of JSON names from the
// top of the format: "a.b" is a member, "a[]" each element of a list,
// "a[0]" its first element, "a{}" each value of a map and "a{key}" each
// key; a path that reaches a type a second time, as the run layout's
// nodes do, is the path of its first place. The constraints are those
// set: a value is checked by each of them.
type Rule struct {
	// Slug names the rule in a finding; it is unique.
	Slug string
	// Format is the format the rule reads.
	Format Format
	// Field is the path of the values the rule checks.
	Field string
	// Source is the requirement id, or the section, the rule implements.
	Source string
	// Text says the constraint in words where the fields below do not
	// say it all, for the rule table.
	Text string
	// Pattern is a regular expression, anchored, a string must match.
	Pattern string
	// NotPattern is a regular expression no string may match.
	NotPattern string
	// MaxBytes is the most bytes a string may have, when it is not zero.
	MaxBytes int
	// Reserved are values a string may not be, compared
	// case-insensitively.
	Reserved []string
	// Enum are the values a string may be, when it is not empty.
	Enum []string
	// Min and Max bound a number, when Range is set; NaN is outside.
	Min, Max float64
	Range    bool
	// MinItems is the fewest elements a list may have.
	MinItems int
	// OneOf are groups of members of an object: exactly one group is
	// present whole, and no member of another group is.
	OneOf [][]string
	// value is a check of a string the fields above do not say.
	value func(string) error
	// inProject is a check across the values of one project spec; it
	// returns a finding for each value it refuses.
	inProject func(p *Project) []Finding
	// inSet is a check across the files of a config repo's projects/
	// directory, and the host settings for a rule of RomeuOnly.
	inSet func(s *set) []Finding
	// Message is the finding's message, after the field's path.
	Message string
	// Row is the error of 04 4.4 the rule raises, when it has one; a
	// rule without one is an invalid document, exit 2, whose finding
	// names the rule's slug.
	Row *cli.Row
	// Scope says which validators check the rule.
	Scope Scope
}

// The patterns 03 3.1 states, by the field they first name.
const (
	workloadPattern    = `^` + registryRepository + `@sha256:[0-9a-f]{64}$`
	projectNamePattern = `^[a-z][a-z0-9-]{0,38}[a-z0-9]$`
	repoDirPattern     = `^[a-z0-9][a-z0-9._-]{0,62}$`
	kitIDPattern       = `^[a-z0-9][a-z0-9-]{0,62}$`
	kitArgKeyPattern   = `^[a-z][a-zA-Z0-9_]{0,31}$`
	secretNamePattern  = `^[a-z0-9][a-z0-9_-]{0,62}$`
	paneIDPattern      = `^[a-z0-9][a-z0-9_-]{0,31}$`
	envKeyPattern      = `^[A-Z_][A-Z0-9_]*$`
	// absolutePattern is the part of an absolute path a schema can say;
	// absolutePath checks the rest.
	absolutePattern = `^/.*$`
	// tokenPattern finds the common token shapes in an argv element
	// (REQ-0206): a GitHub token prefix, or 40 hex digits in either
	// case, at the start or after whitespace or =, so --token=<t> and
	// Bearer <t> are caught too.
	tokenPattern = `(?:^|[\s=])(?:ghp_|gho_|ghs_|ghu_|ghr_|github_pat_)|(?:^|[\s=])[0-9a-fA-F]{40}(?:$|\s)`
	// registryRepository is <registry>/<repository>: a registry host
	// with a dot and an optional port, then path components of
	// lowercase letters and digits joined by separators.
	registryRepository = `[a-z0-9-]+(?:\.[a-z0-9-]+)+(?::[0-9]+)?(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)+`
)

// reservedDirs are the names romeu reserves inside <name>-env/ (03 3.1,
// REQ-0176), with . and .., which no repo dir may be either.
var reservedDirs = []string{".", "..", "memory", "review", "ledger", ".romeu", "sbxenv.yaml", ".git"}

// Rules returns the validation rules, in the order of the rule table: a
// copy, so a caller cannot change what the validators apply.
func Rules() []Rule {
	out := slices.Clone(rules)
	for i := range out {
		out[i].Reserved = slices.Clone(out[i].Reserved)
		out[i].Enum = slices.Clone(out[i].Enum)
		out[i].OneOf = slices.Clone(out[i].OneOf)
		for j := range out[i].OneOf {
			out[i].OneOf[j] = slices.Clone(out[i].OneOf[j])
		}
		if out[i].Row != nil {
			row := *out[i].Row
			out[i].Row = &row
		}
	}
	return out
}

// HasCheck reports whether the rule checks something its exported
// fields do not say, which its Text then says.
func (r Rule) HasCheck() bool { return r.value != nil || r.inProject != nil || r.inSet != nil }

// rules are the validation rules, in the order of the rule table.
var rules = []Rule{
	// The project spec, value by value (03 3.1, 3.2).
	{Slug: "project-name", Format: FormatProject, Field: "name", Source: "REQ-0175", Pattern: projectNamePattern,
		Message: "is not a project name", Scope: Both},
	{Slug: "repos-primary", Format: FormatProject, Field: "repos", Source: "03 3.2", MinItems: 1,
		Text: "exactly one repo is primary", inProject: onePrimary, Message: "holds no repo, or not exactly one primary", Scope: Both},
	{Slug: "repo-url", Format: FormatProject, Field: "repos[].url", Source: "REQ-0177", value: repoURL,
		Text: "https://<host>/<owner>/<repo>, an optional .git; one owner segment", Message: "is not a GitHub-shaped https URL", Scope: Both},
	{Slug: "repo-dir", Format: FormatProject, Field: "repos[].dir", Source: "REQ-0176", Pattern: repoDirPattern, Reserved: reservedDirs,
		Message: "is not a repo dir, or is a name romeu reserves", Scope: Both},
	{Slug: "repo-dir-unique", Format: FormatProject, Field: "repos[].dir", Source: "REQ-0176", inProject: uniqueDirs,
		Text: "unique within the project, compared case-insensitively", Message: "is the dir of another repo of the project", Scope: Both},
	{Slug: "repo-ref", Format: FormatProject, Field: "repos[].ref", Source: "REQ-0181", value: branchName,
		Text: "a branch name valid under git check-ref-format --branch, @ refused", Message: "is not a branch name", Scope: Both},
	{Slug: "workload", Format: FormatProject, Field: "workload", Source: "REQ-0189",
		Pattern: workloadPattern, Message: "is not <registry>/<repository>@sha256:<64 hex>", Scope: Both},
	{Slug: "agent", Format: FormatProject, Field: "agent", Source: "REQ-1259", Pattern: `^[a-z0-9][a-z0-9._-]{0,62}$`,
		Message: "is not an agent handle of lowercase letters, digits, ., _ and -", Scope: Both},
	{Slug: "kit-source", Format: FormatProject, Field: "kits[]", Source: "03 3.2", OneOf: [][]string{{"product"}, {"personal"}},
		Message: "names not exactly one of product and personal", Scope: Both},
	{Slug: "kit-product", Format: FormatProject, Field: "kits[].product", Source: "REQ-0182", Pattern: kitIDPattern,
		Message: "is not a kit id", Scope: Both},
	{Slug: "kit-personal", Format: FormatProject, Field: "kits[].personal", Source: "REQ-0182", Pattern: kitIDPattern,
		Message: "is not a kit id", Scope: Both},
	{Slug: "kit-arg-key", Format: FormatProject, Field: "kits[].args{key}", Source: "REQ-0183", Pattern: kitArgKeyPattern,
		Message: "is not a kit argument name", Scope: Both},
	{Slug: "kit-arg-value", Format: FormatProject, Field: "kits[].args{}", Source: "REQ-0183", MaxBytes: 1024, value: printableArg,
		Text: "printable UTF-8, no newline", Message: "is not printable UTF-8 of at most 1 KiB", Scope: Both},
	{Slug: "secret-name", Format: FormatProject, Field: "secrets[]", Source: "REQ-0184", Pattern: secretNamePattern,
		Message: "is not a secret name", Scope: Both},
	{Slug: "egress-extra", Format: FormatProject, Field: "egress.extra[]", Source: "REQ-0188", value: domainName,
		Text: "lowercase host name, IDNA A-labels, no wildcard, port or IP literal, no label of 0x and hex digits; not under a special-use name: localhost, local, internal, localdomain, lan, home, corp, intranet, private, test, invalid, onion (RFC 6761, 6762, 7686, 8375, ICANN's private-use reservation of internal) or home.arpa", Message: "is not an egress domain", Scope: Both},
	{Slug: "egress-tool", Format: FormatProject, Field: "egress.tools{key}", Source: "REQ-1264",
		Pattern: `^[a-z0-9][a-z0-9-]*:[A-Za-z0-9@][A-Za-z0-9._/@-]*$`,
		Message: "is not a mise backend:tool", Scope: Both},
	{Slug: "egress-tool-domain", Format: FormatProject, Field: "egress.tools{}[]", Source: "REQ-0188", value: domainName,
		Text: "lowercase host name, IDNA A-labels, no wildcard, port or IP literal, no label of 0x and hex digits; not under a special-use name: localhost, local, internal, localdomain, lan, home, corp, intranet, private, test, invalid, onion (RFC 6761, 6762, 7686, 8375, ICANN's private-use reservation of internal) or home.arpa", Message: "is not an egress domain", Scope: Both},

	{Slug: "cpus", Format: FormatProject, Field: "sandboxOptions.cpus", Source: "REQ-1260", Range: true, Min: 0, Max: 256,
		Text:    "0 is sbx's default, the host's; that the host has that many CPUs is checked at run time, not here",
		Message: "is outside 0 to 256", Scope: Both},
	{Slug: "memory", Format: FormatProject, Field: "sandboxOptions.memory", Source: "REQ-1261", Pattern: `^[1-9][0-9]{0,6}[mg]$`, value: memoryLimit,
		Text:    "a whole number of binary units, m or g, at least 512m, as sbx documents it; the host's maximum is checked at run time",
		Message: "is not a memory limit of at least 512m, such as 8g", Scope: Both},
	{Slug: "port", Format: FormatProject, Field: "ports[]", Source: "REQ-1262",
		Pattern: `^(?:(?:127\.0\.0\.1:)?[1-9][0-9]{0,4}:)?[1-9][0-9]{0,4}(?:/(?:tcp|tcp4|tcp6|udp|udp4|udp6))?$`, value: portSpec,
		Text:    "[[HOST_IP:]HOST_PORT:]SANDBOX_PORT[/PROTOCOL] as sbx documents it, each port from 1 to 65535; no port has a leading zero; HOST_IP only 127.0.0.1, since another address would expose the sandbox's port to the network, and none is loopback; the IPv6 loopback form is deferred (03 3.1)",
		Message: "is not a port spec such as 8080:8080/tcp, bound on loopback", Scope: Both},
	{Slug: "salvage-exclude", Format: FormatProject, Field: "salvage.excludeIgnored[]", Source: "REQ-1263", value: excludePath,
		Text: "a clean relative path, an optional trailing /, no .. element, no leading -", Message: "is not a relative path inside the repo", Scope: Both},

	// The run layout (03 3.2).
	{Slug: "tab-label", Format: FormatProject, Field: "run.tabs[].label", Source: "REQ-0186", Pattern: paneIDPattern,
		Message: "is not a tab label", Scope: Both},
	{Slug: "node-shape", Format: FormatProject, Field: "run.tabs[].root", Source: "03 3.2",
		OneOf:   [][]string{{"pane"}, {"split", "ratio", "first", "second"}},
		Message: "is neither {pane} nor {split, ratio, first, second}", Scope: Both},
	{Slug: "node-pane", Format: FormatProject, Field: "run.tabs[].root.pane", Source: "REQ-0186", Pattern: paneIDPattern,
		Message: "is not a pane id", Scope: Both},
	{Slug: "node-split", Format: FormatProject, Field: "run.tabs[].root.split", Source: "03 3.2", Enum: []string{"row", "column"},
		Message: "is neither row nor column", Scope: Both},
	{Slug: "node-ratio", Format: FormatProject, Field: "run.tabs[].root.ratio", Source: "03 3.2", Range: true, Min: 0.1, Max: 0.9,
		Message: "is outside 0.1 to 0.9", Scope: Both},
	{Slug: "pane-id", Format: FormatProject, Field: "run.panes[].id", Source: "REQ-0186", Pattern: paneIDPattern,
		Message: "is not a pane id", Scope: Both},
	{Slug: "pane-placed-once", Format: FormatProject, Field: "run.panes[].id", Source: "REQ-0197", inProject: placedOnce,
		Text:    "every pane id is declared once and appears exactly once across all tabs, and every leaf names a declared pane",
		Message: "is declared twice, placed twice or not at all, or names no declared pane", Scope: Both},
	{Slug: "pane-repo", Format: FormatProject, Field: "run.panes[].repo", Source: "03 3.2", inProject: paneRepos,
		Text: "the dir of a repo of the project", Message: "is not the dir of a repo of the project", Scope: Both},
	{Slug: "pane-cwd", Format: FormatProject, Field: "run.panes[].cwd", Source: "REQ-0187", value: relativePath,
		Text: "clean, no leading /, no .. element", Message: "is not a clean path inside the repo", Scope: Both},
	{Slug: "pane-env-key", Format: FormatProject, Field: "run.panes[].env{key}", Source: "REQ-0193", Pattern: envKeyPattern,
		NotPattern: `^(?:PATH|HOME|BASH_ENV|ENV|LD_.*)$`,
		Message:    "is not an environment variable name, or is PATH, HOME, BASH_ENV, ENV or LD_*", Scope: Both},
	{Slug: "pane-step", Format: FormatProject, Field: "run.panes[].steps[]", Source: "03 3.2", MinItems: 1,
		Text: "an argv list, run with no shell", Message: "is an empty argv", Scope: Both},
	{Slug: "step-argv0", Format: FormatProject, Field: "run.panes[].steps[][0]", Source: "REQ-1265", Pattern: `^[^-].*$`,
		Message: "is an empty program name, or one that starts with -", Scope: Both},
	{Slug: "step-arg", Format: FormatProject, Field: "run.panes[].steps[][]", Source: "REQ-1265", value: noControl,
		Text: "no control character", Message: "holds a control character", Scope: Both},
	{Slug: "pane-env-value", Format: FormatProject, Field: "run.panes[].env{}", Source: "REQ-1265", value: noControl,
		Text: "no control character", Message: "holds a control character", Scope: Both},
	{Slug: "tab-focus", Format: FormatProject, Field: "run.panes[].focus", Source: "03 3.2", inProject: focusPerTab,
		Text: "at most one focused pane per tab", Message: "marks a second focused pane in a tab", Scope: Both},

	// The project specs of a config repo together (03 3.1).
	{Slug: "projects-yaml-only", Format: FormatProject, Field: "projects/", Source: "REQ-0178", inSet: yamlOnly,
		Text: "holds *.yaml files only", Message: "is a file under projects/ that is not *.yaml", Row: &ProjectsStrayFile, Scope: Both},
	{Slug: "project-name-stem", Format: FormatProject, Field: "name", Source: "REQ-0175", inSet: nameIsStem,
		Text: "equals the file stem", Message: "differs from the file stem", Scope: Both},
	{Slug: "project-name-unique", Format: FormatProject, Field: "name", Source: "REQ-0175", inSet: uniqueNames,
		Text: "unique across the config repo", Message: "is the name of another spec", Scope: Both},
	{Slug: "repo-url-unique", Format: FormatProject, Field: "repos[].url", Source: "REQ-0177", inSet: uniqueURLs,
		Text: "in at most one project, compared lowercased without a trailing .git", Message: "is a repo of another project", Scope: Both},
	{Slug: "config-project", Format: FormatProject, Field: "repos[].url", Source: "REQ-0179", inSet: configProject,
		Text:    "exactly one spec's primary repo URL equals config.url of host settings",
		Message: "names config.url as no spec's primary repo, or as several", Row: &ConfigProjectMissing, Scope: RomeuOnly},
	{Slug: "repo-url-host", Format: FormatProject, Field: "repos[].url", Source: "REQ-0180", inSet: knownHosts,
		Text: "the host is a gitHosts entry of host settings", Message: "names a host that no gitHosts entry names", Scope: RomeuOnly},
	{Slug: "workload-repository", Format: FormatProject, Field: "workload", Source: "REQ-0190", inSet: allowedWorkloads,
		Text:    "<registry>/<repository> is in workloadRepositories of host settings",
		Message: "names a repository workloadRepositories does not hold", Scope: RomeuOnly},

	// Host settings (03 3.4).
	{Slug: "root", Format: FormatHostSettings, Field: "root", Source: "03 3.4", Pattern: absolutePattern, value: absolutePath,
		Text: "an absolute, clean path", Message: "is not an absolute, clean path", Scope: RomeuOnly},
	{Slug: "config-name", Format: FormatHostSettings, Field: "config.project", Source: "REQ-0175", Pattern: projectNamePattern,
		Message: "is not a project name", Scope: RomeuOnly},
	{Slug: "config-url", Format: FormatHostSettings, Field: "config.url", Source: "REQ-0177", value: repoURL,
		Text: "https://<host>/<owner>/<repo>, an optional .git; one owner segment", Message: "is not a GitHub-shaped https URL", Scope: RomeuOnly},
	{Slug: "config-ref", Format: FormatHostSettings, Field: "config.ref", Source: "REQ-0181", value: branchName,
		Text: "a branch name valid under git check-ref-format --branch, @ refused", Message: "is not a branch name", Scope: RomeuOnly},
	{Slug: "tool-sbx", Format: FormatHostSettings, Field: "tools.sbx", Source: "03 3.4", Pattern: absolutePattern, value: absolutePath,
		Text: "an absolute, clean path", Message: "is not an absolute, clean path", Scope: RomeuOnly},
	{Slug: "tool-git", Format: FormatHostSettings, Field: "tools.git", Source: "03 3.4", Pattern: absolutePattern, value: absolutePath,
		Text: "an absolute, clean path", Message: "is not an absolute, clean path", Scope: RomeuOnly},
	{Slug: "credential-helper", Format: FormatHostSettings, Field: "tools.gitCredentialHelper", Source: "REQ-1267",
		Pattern: `^(?:[a-z0-9][a-z0-9-]{0,62}|/[A-Za-z0-9._/+-]+)$`, value: credentialHelper,
		Text:    "a git-credential-<name> helper's name, or an absolute, clean path with no space or shell character, since git runs it through sh",
		Message: "is neither a helper name nor a plain absolute path", Scope: RomeuOnly},
	{Slug: "git-host", Format: FormatHostSettings, Field: "gitHosts[].host", Source: "REQ-1268", value: hostName,
		Text: "a lowercase host name, IDNA A-labels, no port or IP literal, no label of 0x and hex digits; a private suffix such as git.corp.internal is valid, unlike an egress domain", Message: "is not a host name", Scope: RomeuOnly},
	{Slug: "ca-file", Format: FormatHostSettings, Field: "gitHosts[].caFile", Source: "03 3.4", Pattern: absolutePattern, value: absolutePath,
		Text: "an absolute, clean path", Message: "is not an absolute, clean path", Scope: RomeuOnly},
	{Slug: "workload-allowlist", Format: FormatHostSettings, Field: "workloadRepositories[]", Source: "REQ-0190",
		Pattern: `^` + registryRepository + `$`, Message: "is not <registry>/<repository>", Scope: RomeuOnly},
	{Slug: "agent-socket", Format: FormatHostSettings, Field: "signing.agentSocket", Source: "03 3.4", Pattern: absolutePattern, value: absolutePath,
		Text: "an absolute, clean path", Message: "is not an absolute, clean path", Scope: RomeuOnly},
	{Slug: "secret-key", Format: FormatHostSettings, Field: "secrets{key}", Source: "REQ-0054",
		Pattern: `^[a-z0-9][a-z0-9_-]{0,62}@[a-z][a-z0-9-]{0,38}[a-z0-9]$`,
		Message: "is not <secret name>@<project>", Scope: RomeuOnly},
	{Slug: "secret-service", Format: FormatHostSettings, Field: "secrets{}.service", Source: "REQ-1266", Pattern: secretNamePattern,
		Message: "is not an sbx service name", Scope: RomeuOnly},
	{Slug: "secret-argv", Format: FormatHostSettings, Field: "secrets{}.argv", Source: "REQ-0205", MinItems: 1,
		Text: "a list, never a shell string", Message: "is an empty argv", Scope: RomeuOnly},
	{Slug: "secret-argv0", Format: FormatHostSettings, Field: "secrets{}.argv[0]", Source: "REQ-0205", Pattern: absolutePattern, value: absolutePath,
		Text: "an absolute, clean path; doctor checks that it exists", Message: "is not an absolute, clean path", Scope: RomeuOnly},
	{Slug: "secret-in-argv", Format: FormatHostSettings, Field: "secrets{}.argv[]", Source: "REQ-0206",
		NotPattern: tokenPattern,
		Text:       "a best-effort guard on the common token shapes, at the start, after whitespace or after =, not a detector", Message: "has the shape of a token",
		Row: &SecretInArgv, Scope: RomeuOnly},
}
