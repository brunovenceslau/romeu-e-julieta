// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

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
)

// workflowsDir holds the workflows, and workflowExt is the one name
// ending a file there may have.
const (
	workflowsDir = ".github/workflows/"
	workflowExt  = ".yml"
)

// Threat model of the workflows check. It guards against two things:
//
//  1. A hosted run that differs from a local one: check logic written
//     in a workflow (an if, a shell, continue-on-error, a container, a
//     called workflow, an expression that computes), a step that starts
//     a program other than tools/ci or tools/release, an action named
//     by a tag or a branch, which can move, a local action (a path in
//     the repository, which runs any shell), a runner label that is not
//     a pinned GitHub-hosted one (*-latest, a self-hosted label, a label
//     read from a variable), and an env table, whose keys could make a
//     tools/ci run start other code (BASH_ENV, LD_PRELOAD, PATH,
//     GOFLAGS=-toolexec). An action takes only the with keys listed
//     for it (actionInputs), each a literal or an exact matrix
//     reference whose values are literals, so no value an event can
//     write reaches an input, and no expression reads the token or a
//     secret. A matrix is a written mapping of the keys
//     os and include, and an include entry holds os and mise_sha256
//     only, so the label a job runs on is always one that is checked.
//  2. A file that reads as one thing and runs as another: an anchor,
//     an alias, a merge key, a tag YAML does not resolve on its own, a
//     key written twice, and a second document.
//
// Out of scope, and left to the review of the ask-first surface
// .github/workflows/** (05 5.3): the values the grammar leaves free
// (the owner of an action, the filters of an event, the strategy values
// outside the matrix), what a pinned
// action's code does, and a pull
// request that edits the workflow or this check itself, since the run
// it starts uses the files of that pull request.

// The grammar of 10 10.2: the keys each level may hold. A key outside
// its list is a finding, and so is any value that the rules below do
// not admit. Everything a workflow does lives in tools/ci, so that a
// developer's shell and a runner run the same code at the same commit.
var (
	fileKeys     = []string{"name", "on", "permissions", "concurrency", "jobs"}
	eventNames   = []string{"pull_request", "push", "schedule", "workflow_dispatch"}
	jobKeys      = []string{"name", "runs-on", "needs", "strategy", "permissions", "timeout-minutes", "steps"}
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
	"jdx/mise-action":  {"version", "sha256", "cache", "env"},
}

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

// workflows applies the grammar to every file below .github/workflows
// in the tree at head, as hygiene does: the tree that is committed is
// the one a runner checks out.
func workflows(ctx context.Context, repo git.Repo, head string) ([]finding, error) {
	files, err := headTree(ctx, repo, head)
	if err != nil {
		return nil, err
	}
	var findings []finding
	for _, f := range files {
		if strings.HasPrefix(f.path, workflowsDir) {
			findings = append(findings, workflowFindings(f.path, f.content)...)
		}
	}
	return findings, nil
}

// workflowFindings returns what one file breaks of the grammar. It
// reads the YAML as a tree of nodes, so a key is seen as it is
// written, with its line, and no value is converted.
func workflowFindings(path string, src []byte) []finding {
	where := git.Printable(path)
	if !strings.HasSuffix(path, workflowExt) {
		return []finding{{where, "workflows", "a file here ends in " + workflowExt}}
	}
	doc, err := parseWorkflow(src)
	if err != nil {
		return []finding{{where, "workflows", err.Error()}}
	}
	c := &grammar{where: where}
	c.walk(doc)
	c.file(doc)
	if strings.TrimPrefix(path, workflowsDir) == "ci.yml" {
		c.edited(doc)
	}
	return c.findings
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
	findings []finding
}

func (c *grammar) add(n *yaml.Node, msg string) {
	c.findings = append(c.findings, finding{c.where + ":" + strconv.Itoa(n.Line), "workflows", msg})
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
		c.expressions(n)
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

// file checks the keys of the file, the events, and each job.
func (c *grammar) file(doc *yaml.Node) {
	for key, value := range pairs(doc) {
		switch {
		case !slices.Contains(fileKeys, key.Value):
			c.add(key, "the key "+strconv.Quote(key.Value)+" is outside the grammar of a file")
		case key.Value == "on":
			c.events(value)
		case key.Value == "permissions":
			c.permissions(value)
		case key.Value == "jobs":
			c.jobs(value)
		}
	}
	c.permissionsSet(doc)
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
		if name.Kind != yaml.ScalarNode || !slices.Contains(eventNames, name.Value) {
			c.add(name, "the event "+strconv.Quote(name.Value)+" is outside the grammar")
		}
	}
}

func (c *grammar) jobs(n *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		c.add(n, "jobs is a mapping")
		return
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
			c.permissions(value)
		case "steps":
			c.steps(value, field(n, "strategy"))
		}
		if !slices.Contains(jobKeys, key.Value) {
			c.add(key, "the key "+strconv.Quote(key.Value)+" is outside the grammar of a job")
		}
	}
	if runsOn := field(n, "runs-on"); runsOn != nil && runsOn.Value == matrixRunner && !osKnown {
		c.add(runsOn, "runs-on is "+matrixRunner+", so the matrix names os, in every include entry or as a key")
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

func (c *grammar) steps(n, strategy *yaml.Node) {
	if n.Kind != yaml.SequenceNode {
		c.add(n, "steps is a list")
		return
	}
	for _, s := range n.Content {
		c.step(s, strategy)
	}
}

// step checks one step: a uses step or a run step, never both, each
// with its own keys and the form of its one command.
func (c *grammar) step(n, strategy *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		c.add(n, "a step is a mapping")
		return
	}
	keys, kind := usesStepKeys, "uses"
	if field(n, "uses") == nil {
		keys, kind = runStepKeys, "run"
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
				c.inputs(uses, value, strategy)
			}
		case "run":
			if value.Kind != yaml.ScalarNode || !runLine(value.Value) {
				c.add(value, "run is one line: go run ./tools/ci or ./tools/release, a subcommand, and plain words")
			}
		}
	}
	if uses := field(n, "uses"); uses != nil && uses.Kind == yaml.ScalarNode && usesForm.MatchString(uses.Value) {
		c.checkout(n, uses)
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

// inputs checks the with keys of a uses step against actionInputs, and
// each value: a literal, or one ${{ matrix.<key> }} alone whose values
// are literals, so that nothing an event can write reaches an input.
func (c *grammar) inputs(uses, with, strategy *yaml.Node) {
	action := actionName(uses)
	allowed := actionInputs[action]
	for key, value := range pairs(with) {
		if !slices.Contains(allowed, key.Value) {
			c.add(key, "the input "+strconv.Quote(key.Value)+" is not one this repository uses for "+action)
			continue
		}
		if value.Kind != yaml.ScalarNode {
			c.add(value, "a with value is a scalar")
		} else if strings.Contains(value.Value, "${{") && !literalMatrixRef(value.Value, strategy) && (!matrixRef.MatchString(value.Value) || !matrixUnreadable(strategy)) {
			c.add(value, "this with input is a literal, or one ${{ matrix.<key> }} alone whose values are literals")
		}
	}
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

// runLine reports whether s is a run line of the grammar: one line of
// words split by single spaces, starting with one of runPrograms and a
// subcommand, each word plain or a quoted variable.
func runLine(s string) bool {
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

// edited fails a ci.yml that a change to a pull request's title or body
// does not run again: its pull_request event lists the type "edited".
func (c *grammar) edited(doc *yaml.Node) {
	on := field(doc, "on")
	if on == nil {
		c.add(doc, "ci.yml runs on pull_request, with the type edited")
		return
	}
	pr := field(on, "pull_request")
	types := field(pr, "types")
	if types != nil && types.Kind == yaml.SequenceNode {
		for _, t := range types.Content {
			if t.Value == "edited" {
				return
			}
		}
	}
	c.add(on, "ci.yml lists edited among its pull_request types, so a change to the title or body runs pr again")
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
