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
//     by a tag or a branch, which can move, and a runner label that
//     floats (*-latest).
//  2. A file that reads as one thing and runs as another: an anchor,
//     an alias, a merge key, a tag YAML does not resolve on its own, a
//     key written twice, and a second document.
//
// Out of scope, and left to the review of the ask-first surface
// .github/workflows/** (05 5.3): the values the grammar leaves free
// (permissions, the owner of an action, with and env values, the
// filters of an event), what a pinned action's code does, and a pull
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
	runStepKeys  = []string{"name", "run", "env"}
)

var (
	// usesForm is an action pinned to a commit: owner, repository, an
	// optional path inside it, and 40 hex digits after the "@".
	usesForm = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*@[0-9a-f]{40}$`)
	// runWord is one word of a run line, and runVar the other form a
	// word may take: a variable of the step's env, quoted.
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
		if !contextPath.MatchString(inner) {
			c.add(n, "an expression is one context path, as matrix.os, with no operator, call or literal")
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
		case key.Value == "jobs":
			c.jobs(value)
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
	for key, value := range pairs(n) {
		switch key.Value {
		case "runs-on", "strategy":
			c.latest(value)
		case "steps":
			c.steps(value)
		}
		if !slices.Contains(jobKeys, key.Value) {
			c.add(key, "the key "+strconv.Quote(key.Value)+" is outside the grammar of a job")
		}
	}
}

// latest fails a runner label that floats: runs-on, and the strategy a
// label may come from through an expression, hold no scalar ending in
// "-latest" (10 10.2, Runners).
func (c *grammar) latest(n *yaml.Node) {
	if n.Kind == yaml.ScalarNode && strings.HasSuffix(n.Value, "-latest") {
		c.add(n, "a runner label is pinned, never *-latest")
	}
	for _, child := range n.Content {
		c.latest(child)
	}
}

func (c *grammar) steps(n *yaml.Node) {
	if n.Kind != yaml.SequenceNode {
		c.add(n, "steps is a list")
		return
	}
	for _, s := range n.Content {
		c.step(s)
	}
}

// step checks one step: a uses step or a run step, never both, each
// with its own keys and the form of its one command.
func (c *grammar) step(n *yaml.Node) {
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
		case "run":
			if value.Kind != yaml.ScalarNode || !runLine(value.Value) {
				c.add(value, "run is one line: go run ./tools/ci or ./tools/release, a subcommand, and plain words")
			}
		}
	}
	if kind == "run" && field(n, "run") == nil {
		c.add(n, "a step has uses or run")
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
