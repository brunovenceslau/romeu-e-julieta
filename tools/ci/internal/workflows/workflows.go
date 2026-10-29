// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package workflows is the workflows check of 10 10.2: the closed
// grammar every file of .github/workflows/ is held to, so that a
// workflow holds no check logic of its own. It judges the commit fast
// and all read before any step, and is the workflows subcommand.
package workflows

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// Dir holds the workflows, and workflowExt is the one name
// ending a file there may have.
const (
	Dir         = ".github/workflows/"
	workflowExt = ".yml"
)

// Threat model of the workflows check. It guards against two things:
//
//  1. A hosted run that differs from a local one: check logic written
//     in a workflow (an if other than the aggregator's, a shell, continue-on-error, a container, a
//     called workflow, an expression that computes), a step that starts
//     a program other than tools/ci or tools/release, an action named
//     by a tag or a branch, which can move, a local action (a path in
//     the repository, which runs any shell), a runner label that is not
//     a pinned GitHub-hosted one (*-latest, a self-hosted label, a label
//     read from a variable), and an env table other than the one at the
//     top of the file, which every workflow holds and which holds
//     mise.MiseEnv and nothing else, since other keys could make a tools/ci
//     run start other code (BASH_ENV, LD_PRELOAD, PATH,
//     GOFLAGS=-toolexec), and without it the mise of the workflow would
//     read every mise file of the change (tools/ci/internal/mise/misefiles.go). An
//     action takes only the with keys listed for it (actionInputs),
//     each a literal or an exact matrix reference whose values are
//     literals, so no value an event can write reaches an input, and
//     no expression reads the token or a secret. A matrix is a written
//     mapping of the keys os and include, and an include entry holds
//     os and mise_sha256 only, so the label a job runs on is always
//     one that is checked.
//  2. A file that reads as one thing and runs as another: an anchor,
//     an alias, a merge key, a tag YAML does not resolve on its own, a
//     key written twice, and a second document.
//
// Out of scope, and left to the review of the ask-first surface
// .github/workflows/** (05 5.3): the values the grammar leaves free
// (the owner of an action, the filters of an event, the strategy values
// outside the matrix), what a pinned
// action's code does, and a pull
// request that citest.Edits the workflow or this check itself, since the run
// it starts uses the files of that pull request.

// The grammar of 10 10.2: the keys each level may hold. A key outside
// its list is a finding, and so is any value that the rules below do
// not admit. Everything a workflow does lives in tools/ci, so that a
// developer's shell and a runner run the same code at the same commit.
var (
	fileKeys     = []string{"name", "on", "permissions", "concurrency", "env", "jobs"}
	eventNames   = []string{"pull_request", "push", "schedule", "workflow_dispatch"}
	jobKeys      = []string{"name", "runs-on", "needs", "if", "strategy", "permissions", "timeout-minutes", "steps"}
	usesStepKeys = []string{"name", "uses", "with"}
	runStepKeys  = []string{"name", "run"}
	strategyKeys = []string{"matrix", "fail-fast", "max-parallel"}
)

// The grammar is made of allowlists, and what a list does not name is
// refused: a denylist always misses a variant (10 10.2).
//
// allowedRunners are the hosted labels of 10 10.2, Runners: the only
// labels runs-on and the os of a matrix may hold.
//
// allowedSecrets are the secrets a workflow may read, as
// ${{ secrets.<name> }}, by lower-case name: none today. A secret joins
// the list with the operator's approval. The token of the run
// (${{ github.token }}) is read through no expression either.
var (
	allowedRunners = []string{"ubuntu-26.04", "ubuntu-26.04-arm", "macos-26", "macos-26-intel"}
	allowedSecrets []string
)

// checkoutAction is the action whose step must not keep the token.
const checkoutAction = "actions/checkout"

// actionInputs are the with keys each action may take, by its
// "<owner>/<repo>": the inputs the workflows of this repository use.
// An action that is not listed takes none. An input that changes what
// runs (mise-action's install_args, mise_toml, tool_versions) stays
// out of a workflow this way.
var actionInputs = map[string][]string{
	"actions/checkout": {"persist-credentials"},
	"jdx/mise-action":  {"version", "sha256", "cache", "env", "persist_github_token"},
	"actions/cache":    {"path", "key"},
	// The publish job of release.yml alone names this action (release.go).
	"actions/attest-build-provenance": {"subject-path", "subject-checksums"},
}

// miseAction is the action whose step must not hand the token on: its
// github_token input reads the job's token by default, and
// persist_github_token: false keeps it from every later step (10 10.2,
// the ask-first job row).
const miseAction = "jdx/mise-action"

// concurrencyKeys are the keys a concurrency mapping may hold: the group
// a run belongs to, and whether a new run cancels the one in progress.
var concurrencyKeys = []string{"group", "cancel-in-progress"}

// matrixRunner is the one expression runs-on may hold; the labels it
// takes come from the os key of the job's matrix, each one of
// allowedRunners.
const matrixRunner = "${{ matrix.os }}"

var (
	// usesForm is an action pinned to a commit: owner, repository, an
	// optional path inside it, and 40 hex digits after the "@".
	// The owner and the repository start with a letter or a digit, and
	// so does each path segment or it starts with "_", so no "." or ".."
	// segment can turn the value into a path of the repository.
	usesForm = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9_.-]*(/[A-Za-z0-9_][A-Za-z0-9_.-]*)*@[0-9a-f]{40}$`)
	// matrixRef is the one expression a literal input may hold: a key
	// of the job's matrix, alone.
	matrixRef = regexp.MustCompile(`^\$\{\{ matrix\.([A-Za-z0-9_-]+) \}\}$`)
	// runWord is one word of a run line, and runVar the other form a
	// word may take: a variable of the runner's environment, quoted.
	runWord = regexp.MustCompile(`^[A-Za-z0-9._/=:-]+$`)
	runVar  = regexp.MustCompile(`^"\$[A-Za-z_][A-Za-z0-9_]*"$`)
	// contextPath is the one form the text between "${{" and "}}" may
	// take: names joined by dots, with spaces around them. Each name
	// starts with a letter, and there are two at least, so no literal
	// (true, null, 1) and no bare context fits, and neither does an
	// operator or a function call.
	contextPath = regexp.MustCompile(`^ *[A-Za-z][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*)+ *$`)
)

// runPrograms are the two programs a run line may start: the first
// three words of the line.
var runPrograms = [][]string{
	{"go", "run", "./tools/ci"},
	{"go", "run", "./tools/release"},
}

// The aggregator job (10 10.2): the required checks of main are two jobs,
// "all on <label>", that every other job of the file feeds. GitHub reports
// a skipped job as a success, also for a required check, and a job is
// skipped when a job it needs fails, so an aggregator with needs alone
// would let a red job merge. An aggregator therefore runs, and fails,
// unless every job it needs succeeded (always() makes it run after a
// failed need, and the if reads each result, the skipped and the cancelled
// ones included), and is skipped, which GitHub reports as a success, when
// all did. It holds the one if of the grammar, compared as a whole string,
// and the one step that fails.
const (
	// AggregatorIf is the if of an aggregator, byte for byte.
	AggregatorIf = "${{ always() && (contains(needs.*.result, 'failure') || contains(needs.*.result, 'cancelled') || contains(needs.*.result, 'skipped')) }}"
	// AggregatorStep is the run line of the one step of an aggregator.
	AggregatorStep = "exit 1"
	// AggregatorTimeout is the timeout-minutes of an aggregator.
	AggregatorTimeout = "5"
)

// workflows applies the grammar to every file below .github/workflows
// in the tree at head, as hygiene does: the tree that is committed is
// the one a runner checks out.
func workflows(ctx context.Context, repo git.Repo, head string) ([]check.Finding, error) {
	files, err := check.HeadTree(ctx, repo, head)
	if err != nil {
		return nil, err
	}
	return Of(files), nil
}

// Of applies the grammar to the workflows among files, the
// tree of one commit as check.HeadTree reads it.
func Of(files []check.TreeFile) []check.Finding {
	var findings []check.Finding
	for _, f := range files {
		if strings.HasPrefix(f.Path, Dir) {
			findings = append(findings, workflowFindings(f.Path, f.Content)...)
		}
	}
	return findings
}

// workflowFindings returns what one file breaks of the grammar. It
// reads the YAML as a tree of nodes, so a key is seen as it is
// written, with its line, and no value is converted.
func workflowFindings(path string, src []byte) []check.Finding {
	where := git.Printable(path)
	if !strings.HasSuffix(path, workflowExt) {
		return []check.Finding{{Where: where, Rule: "workflows", Msg: "a file here ends in " + workflowExt}}
	}
	doc, err := parseWorkflow(src)
	if err != nil {
		return []check.Finding{{Where: where, Rule: "workflows", Msg: err.Error()}}
	}
	name := strings.TrimPrefix(path, Dir)
	c := &grammar{where: where, exempt: exempted(doc), rel: readRelease(path, doc), pr: readPR(path, doc), inPR: name == prFile}
	c.walk(doc)
	c.file(doc)
	if name == releaseFile {
		c.publish(doc)
	}
	if c.inPR {
		c.prWorkflow(doc)
	}
	if labels, ok := hostedFiles[strings.TrimPrefix(path, Dir)]; ok {
		c.hosted(doc, labels)
	}
	return c.findings
}

// hostedFiles are the two files that run the jobs of 10 10.2, by name,
// with the runner labels each one runs on: the Linux label that the three
// jobs and the aggregators use, and the macOS one that the race job adds.
var hostedFiles = map[string]struct{ linux, mac string }{
	"ci.yml":  {"ubuntu-26.04-arm", "macos-26"},
	"x64.yml": {"ubuntu-26.04", "macos-26-intel"},
}

// parseWorkflow reads exactly one YAML document whose root is a
// mapping.
func parseWorkflow(src []byte) (*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("not YAML: %w", err)
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return nil, errors.New("a workflow is one YAML document")
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("a workflow is a mapping")
	}
	return doc.Content[0], nil
}

// grammar collects the findings of one file.
type grammar struct {
	where    string
	findings []check.Finding
	// exempt are the nodes whose expression is not a context path: the
	// cancel-in-progress of the file's concurrency, when it is
	// prOnlyExpression (prOnlyCancel), and the if of a job, which
	// aggregator compares as a whole string.
	exempt []*yaml.Node
	// plain are the ids of the jobs of the file that are not aggregators
	// (jobs): what the needs of an aggregator lists.
	plain []string
	// rel is what a release.yml grants its publish job (release.go), and
	// nil in every other file.
	rel *release
	// pr is what ask-first.yml grants its job (pr.go), and nil in every other
	// file; inPR says the file is ask-first.yml, with an ask-first job or without.
	pr   *prRow
	inPR bool
}

// prOnlyExpression is the one expression the grammar admits that is no
// context path, and only as the cancel-in-progress of the file's
// concurrency: a new run cancels the one in progress only for a pull
// request, so that a push to the default branch never cancels the run of
// the push before it.
const prOnlyExpression = "${{ github.event_name == 'pull_request' }}"

// prOnlyCancel returns the cancel-in-progress node of the file's
// concurrency when it is prOnlyExpression, and nil otherwise.
func prOnlyCancel(doc *yaml.Node) *yaml.Node {
	v := field(field(doc, "concurrency"), "cancel-in-progress")
	if v != nil && v.Kind == yaml.ScalarNode && v.Tag == "!!str" && v.Value == prOnlyExpression {
		return v
	}
	return nil
}

// exempted returns the nodes whose expression walk does not hold to a
// context path (grammar.exempt): the cancel-in-progress that is
// prOnlyExpression, and the if of every job, which the aggregator rule
// reads as a whole string.
func exempted(doc *yaml.Node) []*yaml.Node {
	var out []*yaml.Node
	if n := prOnlyCancel(doc); n != nil {
		out = append(out, n)
	}
	for _, job := range pairs(field(doc, "jobs")) {
		if v := field(job, "if"); v != nil && v.Kind == yaml.ScalarNode {
			out = append(out, v)
		}
	}
	return out
}

func (c *grammar) add(n *yaml.Node, msg string) {
	c.findings = append(c.findings, check.Finding{Where: c.where + ":" + strconv.Itoa(n.Line), Rule: "workflows", Msg: msg})
}

// plainTags are the tags YAML resolves on its own. Any other tag, as
// "!!binary", a custom one or "!!merge", the tag of a "<<" key that
// copies another mapping's keys in, would make a value mean something
// the grammar does not read.
var plainTags = []string{"!!str", "!!int", "!!float", "!!bool", "!!null", "!!timestamp", "!!map", "!!seq"}

// walk visits every node once, for the rules that hold at any depth:
// no anchor, alias, merge key or unusual tag, no key written twice, and
// each expression a context path.
func (c *grammar) walk(n *yaml.Node) {
	switch {
	case n.Kind == yaml.AliasNode || n.Anchor != "":
		c.add(n, "an anchor or an alias is outside the grammar")
		return
	case !slices.Contains(plainTags, n.Tag):
		c.add(n, "the tag "+strconv.Quote(n.Tag)+" is outside the grammar")
	case n.Kind == yaml.ScalarNode:
		if !slices.Contains(c.exempt, n) && !c.rel.isToken(n) {
			c.expressions(n)
		}
	}
	if n.Kind == yaml.MappingNode {
		var seen []string
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Kind != yaml.ScalarNode {
				c.add(key, "a key is a plain name")
				continue
			}
			// A tag outside plainTags is already a finding of its own.
			if key.Tag != "!!str" && slices.Contains(plainTags, key.Tag) {
				c.add(key, "a key is written as a plain string, never as another kind")
			}
			if slices.Contains(seen, key.Value) {
				c.add(key, "the key "+strconv.Quote(key.Value)+" is written twice")
			}
			seen = append(seen, key.Value)
		}
	}
	for _, child := range n.Content {
		c.walk(child)
	}
}

// expressions checks each "${{ ... }}" of a scalar: the text between
// the braces is one context path.
func (c *grammar) expressions(n *yaml.Node) {
	rest := n.Value
	for {
		_, after, found := strings.Cut(rest, "${{")
		if !found {
			return
		}
		inner, tail, closed := strings.Cut(after, "}}")
		if !closed {
			c.add(n, "an expression is not closed")
			return
		}
		path := strings.ToLower(strings.TrimSpace(inner))
		switch name, isSecret := strings.CutPrefix(path, "secrets."); {
		case !contextPath.MatchString(inner):
			c.add(n, "an expression is one context path, as matrix.os, with no operator, call or literal")
		case isSecret && !slices.Contains(allowedSecrets, name):
			c.add(n, "a secret is read only from the list of allowed secrets, which is empty")
		case path == "github.token":
			c.add(n, "the token is read through no expression, as the list of allowed secrets is empty")
		}
		rest = tail
	}
}

// file checks the keys of the file, the env table, the events, and
// each job.
func (c *grammar) file(doc *yaml.Node) {
	for key, value := range pairs(doc) {
		switch {
		case !slices.Contains(fileKeys, key.Value):
			c.add(key, "the key "+strconv.Quote(key.Value)+" is outside the grammar of a file")
		case key.Value == "env":
			c.env(value)
		case key.Value == "on":
			c.events(value)
		case key.Value == "permissions":
			c.permissions(value)
		case key.Value == "concurrency":
			c.concurrency(value)
		case key.Value == "jobs":
			c.jobs(value)
		}
	}
	if field(doc, "env") == nil {
		c.add(doc, envRule)
	}
	c.permissionsSet(doc)
}

// envRule is the finding of a file whose env table is not mise.MiseEnv.
var envRule = "env holds exactly the mise environment of tools/ci, each value a YAML string, in this order: " + strings.Join(mise.MiseEnv, " ")

// env checks the env table of the file: the variables of mise.MiseEnv, in
// its order, each a YAML string with the value of the list, and no
// other. Every workflow sets it, since every step of one reaches go
// through the mise shims, and mise-action runs mise itself.
func (c *grammar) env(n *yaml.Node) {
	ok := n.Kind == yaml.MappingNode && len(n.Content) == 2*len(mise.MiseEnv)
	for i, kv := range mise.MiseEnv {
		if !ok {
			break
		}
		key, value, _ := strings.Cut(kv, "=")
		k, v := n.Content[2*i], n.Content[2*i+1]
		ok = k.Value == key && v.Kind == yaml.ScalarNode && v.Tag == "!!str" && v.Value == value
	}
	if !ok {
		c.add(n, envRule)
	}
}

// permissionsSet requires a permissions mapping at the top of the file,
// or in every job: the token of a run then never has the default scopes
// of the repository's settings.
func (c *grammar) permissionsSet(doc *yaml.Node) {
	if field(doc, "permissions") != nil {
		return
	}
	for key, job := range pairs(field(doc, "jobs")) {
		if job.Kind == yaml.MappingNode && field(job, "permissions") == nil {
			c.add(key, "permissions is set at the top of the file or in every job, so that no scope is left to the default")
		}
	}
}

// permissions checks the scopes of a permissions key, at the top or in
// a job: a mapping whose values are read or none. A word (read-all,
// write-all) and any write, id-token included, are outside the grammar.
func (c *grammar) permissions(n *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		c.add(n, "permissions is a mapping of scopes, with no read-all or write-all")
		return
	}
	for _, scope := range pairs(n) {
		if scope.Kind != yaml.ScalarNode || (scope.Value != "read" && scope.Value != "none") {
			c.add(scope, "a permission is read or none, never a write")
		}
	}
}

// events checks the names under "on", in each form YAML allows: one
// name, a list of names, or a mapping of names to their filters.
func (c *grammar) events(n *yaml.Node) {
	var names []*yaml.Node
	switch n.Kind {
	case yaml.ScalarNode:
		names = []*yaml.Node{n}
	case yaml.SequenceNode:
		names = n.Content
	case yaml.MappingNode:
		for key := range pairs(n) {
			names = append(names, key)
		}
	}
	for _, name := range names {
		switch {
		case name.Kind == yaml.ScalarNode && name.Value == prEvent:
			c.prOnly(name)
		case name.Kind != yaml.ScalarNode || !slices.Contains(eventNames, name.Value):
			c.add(name, "the event "+strconv.Quote(name.Value)+" is outside the grammar")
		}
	}
}

func (c *grammar) jobs(n *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		c.add(n, "jobs is a mapping")
		return
	}
	c.plain = nil
	for id, job := range pairs(n) {
		if field(job, "if") == nil {
			c.plain = append(c.plain, id.Value)
		}
	}
	for _, job := range pairs(n) {
		c.job(job)
	}
}

func (c *grammar) job(n *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		c.add(n, "a job is a mapping")
		return
	}
	osKnown := false
	aggregator := field(n, "if") != nil
	for key, value := range pairs(n) {
		switch key.Value {
		case "runs-on":
			if value.Kind != yaml.ScalarNode || (value.Value != matrixRunner && !slices.Contains(allowedRunners, value.Value)) {
				c.add(value, "runs-on is "+matrixRunner+" or one of "+strings.Join(allowedRunners, ", ")+", never *-latest")
			}
		case "strategy":
			c.strategy(value)
			osKnown = c.matrix(field(value, "matrix"))
		case "permissions":
			if c.rel.isJob(n) {
				c.publishPermissions(value)
			} else {
				c.permissions(value)
			}
		case "steps":
			c.steps(value, field(n, "strategy"), aggregator && len(seqContent(value)) == 1)
		}
		if !slices.Contains(jobKeys, key.Value) {
			c.add(key, "the key "+strconv.Quote(key.Value)+" is outside the grammar of a job")
		}
	}
	if runsOn := field(n, "runs-on"); runsOn != nil && runsOn.Value == matrixRunner && !osKnown {
		c.add(runsOn, "runs-on is "+matrixRunner+", so the matrix names os, in every include entry or as a key")
	}
	if aggregator {
		c.aggregator(n)
	}
}

// aggregator holds a job with an if to the shape of the aggregator job
// (AggregatorIf): a literal name, one literal runner label, no strategy,
// a needs that lists every job of the file that is not an aggregator, the
// if of AggregatorIf byte for byte, and the one step AggregatorStep. A job
// with an if that is not this is a finding for each part it breaks, so an
// if on any other job fails.
func (c *grammar) aggregator(n *yaml.Node) {
	const shape = "an aggregator job (the only job with an if) "
	if name := field(n, "name"); name == nil || !literal(name) {
		c.add(n, shape+"has a literal name, with no ${{ }}: a required check is matched by its name")
	}
	if runsOn := field(n, "runs-on"); runsOn == nil || runsOn.Value == matrixRunner {
		c.add(n, shape+"runs on one literal runner label, not "+matrixRunner)
	}
	if s := field(n, "strategy"); s != nil {
		c.add(s, shape+"has no strategy: a matrix job is named with its expression unexpanded when skipped")
	}
	c.aggregatorNeeds(n)
	if v := field(n, "if"); v.Kind != yaml.ScalarNode || v.Tag != "!!str" || v.Value != AggregatorIf {
		c.add(v, shape+"holds this if, byte for byte: "+AggregatorIf)
	}
	if v := field(n, "timeout-minutes"); v == nil || v.Kind != yaml.ScalarNode || v.Value != AggregatorTimeout {
		c.add(n, shape+"has timeout-minutes: "+AggregatorTimeout+", so a stuck runner does not hold a required check")
	}
	steps := seqContent(field(n, "steps"))
	if len(steps) != 1 || field(steps[0], "uses") != nil || field(steps[0], "run") == nil || field(steps[0], "run").Value != AggregatorStep {
		c.add(n, shape+"has one step, run: "+AggregatorStep)
	}
}

// aggregatorNeeds holds the needs of an aggregator to the jobs of the
// file that are not aggregators (grammar.plain): every one of them, and
// no other name.
func (c *grammar) aggregatorNeeds(n *yaml.Node) {
	needs := field(n, "needs")
	var listed []string
	switch {
	case needs == nil:
	case needs.Kind == yaml.ScalarNode:
		listed = []string{needs.Value}
	case needs.Kind == yaml.SequenceNode:
		for _, item := range needs.Content {
			listed = append(listed, item.Value)
		}
	}
	var wrong []string
	for _, id := range c.plain {
		if !slices.Contains(listed, id) {
			wrong = append(wrong, "missing "+strconv.Quote(id))
		}
	}
	for _, id := range listed {
		if !slices.Contains(c.plain, id) {
			wrong = append(wrong, "unknown "+strconv.Quote(id))
		}
	}
	if len(wrong) > 0 {
		at := needs
		if at == nil {
			at = n
		}
		c.add(at, "an aggregator job (the only job with an if) needs every other job of the file that is not an aggregator, no more and no fewer: "+strings.Join(wrong, ", "))
	}
}

// matrixKeys and includeKeys are the only keys a matrix and an entry of
// its include may hold, matched exactly: the runner label is read from
// os alone, so a key that differs in case from it (OS) is no label.
var (
	matrixKeys  = []string{"os", "include"}
	includeKeys = []string{"os", "mise_sha256"}
)

// matrix checks a job's matrix: a written mapping of the matrixKeys,
// never an expression that computes one, with the runner labels of os
// and of each include entry checked. It reports whether every runner
// the matrix makes has an os.
func (c *grammar) matrix(n *yaml.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind != yaml.MappingNode {
		c.add(n, "a matrix is a written mapping, not an expression")
		return true
	}
	osNode := field(n, "os")
	c.matrixLabels(osNode)
	for key := range pairs(n) {
		if !slices.Contains(matrixKeys, key.Value) {
			c.add(key, "the matrix key "+strconv.Quote(key.Value)+" is outside the grammar: os and include")
		}
	}
	include := field(n, "include")
	if include != nil && include.Kind != yaml.SequenceNode {
		c.add(include, "include is a written list, not an expression")
		return true
	}
	every := include != nil && len(include.Content) > 0
	for _, entry := range seqContent(include) {
		if entry.Kind != yaml.MappingNode {
			c.add(entry, "an include entry is a mapping")
			every = false
			continue
		}
		for key := range pairs(entry) {
			if !slices.Contains(includeKeys, key.Value) {
				c.add(key, "the include key "+strconv.Quote(key.Value)+" is outside the grammar: os and mise_sha256")
			}
		}
		// The os value is held to the runner labels by matrixLabels.
		for key, value := range pairs(entry) {
			if key.Value != "os" && !literal(value) {
				c.add(value, "a matrix value is a literal scalar")
			}
		}
		c.matrixLabels(field(entry, "os"))
		every = every && field(entry, "os") != nil
	}
	return osNode != nil || every
}

// strategy checks the keys of a job's strategy against strategyKeys.
func (c *grammar) strategy(n *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		c.add(n, "strategy is a mapping")
		return
	}
	for key := range pairs(n) {
		if !slices.Contains(strategyKeys, key.Value) {
			c.add(key, "the strategy key "+strconv.Quote(key.Value)+" is outside the grammar: matrix, fail-fast and max-parallel")
		}
	}
}

// matrixValues returns the values the matrix of strategy gives to key:
// the key itself, a scalar or a list, and the key of each include entry.
// ok is false when there is none, or when one is not a scalar free of
// expressions, since then the value an input receives is not a literal.
func matrixValues(strategy *yaml.Node, key string) (values []string, ok bool) {
	matrix := field(strategy, "matrix")
	ok = true
	add := func(v *yaml.Node) {
		if !literal(v) {
			ok = false
			return
		}
		values = append(values, v.Value)
	}
	if v := field(matrix, key); v != nil && key != "include" {
		if v.Kind == yaml.SequenceNode {
			for _, item := range v.Content {
				add(item)
			}
		} else {
			add(v)
		}
	}
	for _, entry := range seqContent(field(matrix, "include")) {
		if v := field(entry, key); v != nil {
			add(v)
		}
	}
	return values, ok && len(values) > 0
}

// matrixLabels checks the runner labels of a matrix's os key, one
// label or a list of them: each a pinned GitHub-hosted label.
func (c *grammar) matrixLabels(n *yaml.Node) {
	labels := []*yaml.Node{n}
	if n == nil {
		return
	}
	if n.Kind == yaml.SequenceNode {
		labels = n.Content
	}
	for _, l := range labels {
		if l.Kind != yaml.ScalarNode || !slices.Contains(allowedRunners, l.Value) {
			c.add(l, "a runner label of the matrix is one of "+strings.Join(allowedRunners, ", ")+", never *-latest")
		}
	}
}

// seqContent returns the items of a sequence node, and nothing for
// another node.
func seqContent(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

// steps checks the steps of a job. exitOK says that the job is an
// aggregator with one step, the only place the run line AggregatorStep
// is admitted.
func (c *grammar) steps(n, strategy *yaml.Node, exitOK bool) {
	if n.Kind != yaml.SequenceNode {
		c.add(n, "steps is a list")
		return
	}
	for _, s := range n.Content {
		c.step(s, strategy, exitOK)
	}
}

// step checks one step: a uses step or a run step, never both, each
// with its own keys and the form of its one command. exitOK is set for
// the one step of an aggregator job (steps).
func (c *grammar) step(n, strategy *yaml.Node, exitOK bool) {
	if n.Kind != yaml.MappingNode {
		c.add(n, "a step is a mapping")
		return
	}
	keys, kind := usesStepKeys, "uses"
	if field(n, "uses") == nil {
		keys, kind = runStepKeys, "run"
	}
	if c.rel.mayHoldEnv(n) {
		keys = slices.Concat(keys, []string{"env"})
	}
	for key, value := range pairs(n) {
		if !slices.Contains(keys, key.Value) {
			c.add(key, "the key "+strconv.Quote(key.Value)+" is outside the grammar of a "+kind+" step")
			continue
		}
		switch key.Value {
		case "uses":
			if value.Kind != yaml.ScalarNode || !usesForm.MatchString(value.Value) {
				c.add(value, "uses names <owner>/<repo>[/<path>]@<a 40-hex commit SHA>")
			}
		case "with":
			if value.Kind != yaml.MappingNode {
				c.add(value, "with is a mapping")
			} else if uses := field(n, "uses"); uses.Kind == yaml.ScalarNode && usesForm.MatchString(uses.Value) {
				c.inputs(n, uses, value, strategy)
			}
		case "env":
			c.stepEnv(value)
		case "run":
			allowed := value.Kind == yaml.ScalarNode && (runLine(value.Value) ||
				(exitOK && value.Value == AggregatorStep && value.Tag == "!!str") ||
				(c.pr.inJob(n) && value.Value == PRLine && value.Tag == "!!str"))
			if !allowed {
				c.add(value, "run is one line: go run ./tools/ci or ./tools/release, a subcommand, and plain words")
			}
		}
	}
	if uses := field(n, "uses"); uses != nil && uses.Kind == yaml.ScalarNode && usesForm.MatchString(uses.Value) {
		c.checkout(n, uses)
		c.miseToken(n, uses)
		c.attest(n, uses)
		c.publishUses(n, uses)
	}
	if kind == "run" && field(n, "run") == nil {
		c.add(n, "a step has uses or run")
	}
}

// actionName returns "<owner>/<repo>" of a uses value, in lower case:
// GitHub reads the owner and the repository without case.
func actionName(uses *yaml.Node) string {
	action, _, _ := strings.Cut(uses.Value, "@")
	if parts := strings.SplitN(action, "/", 3); len(parts) >= 2 {
		action = parts[0] + "/" + parts[1]
	}
	return strings.ToLower(action)
}

// inputs checks the with keys of a uses step against actionInputs (and,
// for the checkout of the ask-first job, prDepth), and
// each value: a literal, or one ${{ matrix.<key> }} alone whose values
// are literals, so that nothing an event can write reaches an input.
func (c *grammar) inputs(step, uses, with, strategy *yaml.Node) {
	action := actionName(uses)
	allowed := actionInputs[action]
	if action == checkoutAction && c.pr.inJob(step) {
		allowed = slices.Concat(allowed, []string{prDepth})
	}
	for key, value := range pairs(with) {
		if !slices.Contains(allowed, key.Value) {
			c.add(key, "the input "+strconv.Quote(key.Value)+" is not one this repository uses for "+action)
			continue
		}
		if value.Kind != yaml.ScalarNode {
			c.add(value, "a with value is a scalar")
		} else if withExpressionFails(runnerFree(action, key.Value, value.Value), strategy) {
			c.add(value, "this with input is a literal, or one ${{ matrix.<key> }} alone whose values are literals")
		}
	}
}

// reportedByWalk reports whether n holds an expression that walk
// already reports, as it is no context path, so that no second finding
// names it.
func reportedByWalk(n *yaml.Node) bool {
	rest := n.Value
	for {
		_, after, found := strings.Cut(rest, "${{")
		if !found {
			return false
		}
		inner, tail, closed := strings.Cut(after, "}}")
		if !closed || !contextPath.MatchString(inner) {
			return true
		}
		rest = tail
	}
}

// runnerExpressions are the expressions the key of an actions/cache step
// may hold beside its literal text: the runner's OS and architecture,
// which the runner sets and no event can write, so that each runner
// class keeps its own cache entry.
var runnerExpressions = []string{"${{ runner.os }}", "${{ runner.arch }}"}

// runnerFree returns value with each of runnerExpressions taken out when
// it is the key input of actions/cache, and value as it is otherwise.
func runnerFree(action, input, value string) string {
	if action != "actions/cache" || input != "key" {
		return value
	}
	for _, e := range runnerExpressions {
		value = strings.ReplaceAll(value, e, "")
	}
	return value
}

// withExpressionFails reports whether a with value holds an expression
// that the grammar does not allow: it has a ${{, it is not one matrix
// reference whose values are all literals, and it is not a matrix
// reference excused because the matrix is unreadable (matrixUnreadable,
// whose own finding says what is wrong).
func withExpressionFails(value string, strategy *yaml.Node) bool {
	if !strings.Contains(value, "${{") {
		return false
	}
	if literalMatrixRef(value, strategy) {
		return false
	}
	return !matrixRef.MatchString(value) || !matrixUnreadable(strategy)
}

// matrixUnreadable reports whether strategy has a matrix that matrix
// does not read, so that the value an input gets from it is unknown: a
// matrix that is not a written mapping, an include that is not a written
// list, or an include value that is not a literal. Each is a finding of
// its own already; it excuses only a matrix reference, never another
// expression. A job with no strategy, or a strategy with no matrix,
// is not unreadable: nothing there gives an input a literal, so a
// matrix reference or any other expression in with is a finding.
func matrixUnreadable(strategy *yaml.Node) bool {
	matrix := field(strategy, "matrix")
	if matrix == nil {
		return false
	}
	include := field(matrix, "include")
	if matrix.Kind != yaml.MappingNode || (include != nil && include.Kind != yaml.SequenceNode) {
		return true
	}
	for _, entry := range seqContent(include) {
		for key, value := range pairs(entry) {
			if key.Value != "os" && !literal(value) {
				return true
			}
		}
	}
	return false
}

// literal reports whether n is a scalar with no expression in it.
func literal(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && !strings.Contains(n.Value, "${{")
}

// literalMatrixRef reports whether s is exactly ${{ matrix.<key> }} and
// every value the matrix gives to key is a literal.
func literalMatrixRef(s string, strategy *yaml.Node) bool {
	m := matrixRef.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	_, ok := matrixValues(strategy, m[1])
	return ok
}

// checkout fails a checkout step that does not set persist-credentials:
// false, so that no later step holds the token. The action is matched
// without case, as GitHub reads it.
func (c *grammar) checkout(step, uses *yaml.Node) {
	if actionName(uses) != checkoutAction {
		return
	}
	if with := field(step, "with"); with != nil && with.Kind != yaml.MappingNode {
		return // "with is a mapping" is the finding
	}
	if v := field(field(step, "with"), "persist-credentials"); v == nil || v.Kind != yaml.ScalarNode || v.Value != "false" {
		c.add(step, "a checkout step sets persist-credentials: false, so that no later step holds the token")
	}
}

// miseToken fails a jdx/mise-action step that does not write
// persist_github_token: false, the YAML boolean, so that no later step
// reads the job's token from the environment the action exports. The
// action is matched without case, as GitHub reads it.
func (c *grammar) miseToken(step, uses *yaml.Node) {
	if actionName(uses) != miseAction {
		return
	}
	if with := field(step, "with"); with != nil && with.Kind != yaml.MappingNode {
		return // "with is a mapping" is the finding
	}
	if v := field(field(step, "with"), "persist_github_token"); v == nil || v.Kind != yaml.ScalarNode || v.Tag != "!!bool" || v.Value != "false" {
		c.add(step, "every jdx/mise-action step writes persist_github_token: false, so that no later step holds the token")
	}
}

// concurrency checks the concurrency of the file: a mapping of
// concurrencyKeys that names its group, with cancel-in-progress written
// true or false. The group may hold expressions, each a context path
// (walk), so that a run of a pull request's ref cancels the run of the
// same ref it replaces.
func (c *grammar) concurrency(n *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		c.add(n, "concurrency is a mapping of group and cancel-in-progress")
		return
	}
	for key, value := range pairs(n) {
		switch {
		case !slices.Contains(concurrencyKeys, key.Value):
			c.add(key, "the concurrency key "+strconv.Quote(key.Value)+" is outside the grammar: group and cancel-in-progress")
		case key.Value == "cancel-in-progress" && !slices.Contains(c.exempt, value) && !reportedByWalk(value) && (value.Kind != yaml.ScalarNode || value.Tag != "!!bool"):
			c.add(value, "cancel-in-progress is true, false or "+prOnlyExpression)
		}
	}
	if field(n, "group") == nil {
		c.add(n, "concurrency names its group")
	}
}

// BuildLine is the one run line that builds a program: tools/ci into
// the runner's temporary directory, and BuiltCI is that program, which a
// run line may start in place of "go run ./tools/ci". A program started
// by "go run" reaches the job with every exit status other than 0 as 1;
// the built one returns its own, so that the job's log and status show
// the infrastructure status of tools/ci (3) as it is.
const (
	BuildLine = `go build -o "$RUNNER_TEMP/ci" ./tools/ci`
	BuiltCI   = `"$RUNNER_TEMP/ci"`
)

// runLine reports whether s is a run line of the grammar: one line of
// words split by single spaces, starting with one of runPrograms and a
// subcommand, or with BuiltCI and a subcommand, each word plain or a
// quoted variable; or BuildLine exactly.
func runLine(s string) bool {
	if s == BuildLine {
		return true
	}
	if rest, ok := strings.CutPrefix(s, BuiltCI+" "); ok {
		s = strings.Join(runPrograms[0], " ") + " " + rest
	}
	words := strings.Split(s, " ")
	if len(words) < 4 {
		return false
	}
	known := slices.ContainsFunc(runPrograms, func(p []string) bool { return slices.Equal(p, words[:3]) })
	if !known || !runWord.MatchString(words[3]) {
		return false
	}
	for _, w := range words[4:] {
		if !runWord.MatchString(w) && !runVar.MatchString(w) {
			return false
		}
	}
	return true
}

// pairs yields the keys and values of a mapping node, in order, and
// nothing for another node.
func pairs(n *yaml.Node) iter.Seq2[*yaml.Node, *yaml.Node] {
	return func(yield func(key, value *yaml.Node) bool) {
		if n == nil || n.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if !yield(n.Content[i], n.Content[i+1]) {
				return
			}
		}
	}
}

// field returns the value of a key of a mapping node, or nil.
func field(n *yaml.Node, key string) *yaml.Node {
	for k, v := range pairs(n) {
		if k.Value == key {
			return v
		}
	}
	return nil
}

// Run applies the grammar of 10 10.2 to the workflows of the
// tree at HEAD.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("workflows takes no argument\n%s", check.Usage())
	}
	findings, err := workflows(ctx, e.Repo(), "HEAD")
	if err != nil {
		return false, err
	}
	check.Say(e.Stdout, "%s%s\n", check.Lines(findings), check.Summary("workflows", len(findings)))
	return len(findings) == 0, nil
}

// Judge is the workflows check of the commit fast and all read before
// any step (Of).
func Judge(_ context.Context, c check.Commit) ([]check.Verdict, error) {
	return []check.Verdict{{Name: "workflows", Findings: Of(c.Files)}}, nil
}

// hosted holds ci.yml and x64.yml to the jobs of 10 10.2, which the rest
// of the grammar would let a change delete together with its needs: the
// jobs that are not aggregators are exactly check.Jobs, each runs the
// built tools/ci with its own subcommand, the matrix of the race job
// holds the Linux and the macOS label and the other two the Linux label
// alone, and the aggregators are exactly two, named for the two labels.
func (c *grammar) hosted(doc *yaml.Node, labels struct{ linux, mac string }) {
	jobs := field(doc, "jobs")
	var missing, unknown []string
	for _, id := range check.Jobs {
		if !slices.Contains(c.plain, id) {
			missing = append(missing, id)
		}
	}
	for _, id := range c.plain {
		if !slices.Contains(check.Jobs, id) {
			unknown = append(unknown, id)
		}
	}
	if len(missing) > 0 || len(unknown) > 0 {
		c.add(jobs, "the jobs of this file are "+strings.Join(check.Jobs, ", ")+" besides the aggregators"+
			listed(", missing ", missing)+listed(", unknown ", unknown))
	}
	for _, id := range check.Jobs {
		job := field(jobs, id)
		if job == nil {
			continue
		}
		want := BuiltCI + " " + id
		if !slices.ContainsFunc(seqContent(field(job, "steps")), func(s *yaml.Node) bool {
			run := field(s, "run")
			return run != nil && run.Value == want
		}) {
			c.add(job, "the job "+id+" runs the built tools/ci with its own subcommand: "+want)
		}
		wantLabels := []string{labels.linux}
		if id == check.JobRace {
			wantLabels = []string{labels.linux, labels.mac}
		}
		if got := matrixLabelsOf(job); !slices.Equal(got, wantLabels) {
			if id == check.JobRace {
				c.add(job, "the matrix of the job race holds "+labels.linux+" and "+labels.mac)
			} else {
				c.add(job, "the matrix of the job "+id+" holds "+labels.linux+" alone")
			}
		}
	}
	var names []string
	for _, job := range pairs(jobs) {
		if field(job, "if") != nil {
			names = append(names, field(job, "name").Value)
		}
	}
	wantNames := []string{"all on " + labels.linux, "all on " + labels.mac}
	if len(names) != 2 {
		c.add(jobs, "this file has exactly two aggregators, named "+strconv.Quote(wantNames[0])+" and "+strconv.Quote(wantNames[1]))
	} else if slices.Sort(names); !slices.Equal(names, slices.Sorted(slices.Values(wantNames))) {
		c.add(jobs, "the aggregators are named "+strconv.Quote(wantNames[0])+" and "+strconv.Quote(wantNames[1]))
	}
}

// listed returns prefix and the names joined, or nothing for none.
func listed(prefix string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	return prefix + strings.Join(names, ", ")
}

// matrixLabelsOf returns the os values of the include entries of a job's
// matrix, in order.
func matrixLabelsOf(job *yaml.Node) []string {
	var labels []string
	for _, entry := range seqContent(field(field(field(job, "strategy"), "matrix"), "include")) {
		if os := field(entry, "os"); os != nil {
			labels = append(labels, os.Value)
		}
	}
	return labels
}
