// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package approval

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// What pr reads of the head besides the approval lines. The head is the
// author's: each rule here reads its objects as data, from the code of
// the default branch, and fails closed on what it cannot read.

// workflowsDir is the directory GitHub reads workflow files from, and
// PRWorkflow the file of the pr job (10 10.2, the pr job row).
const (
	workflowsDir = ".github/workflows"
	// PRWorkflow is the file of the pr job, and PREvent its one event;
	// the grammar of tools/ci workflows reads the same names.
	PRWorkflow = workflowsDir + "/pr.yml"
	PREvent    = "pull_request_target"
)

// goModeFiles are the paths at the top of the tree that change what the
// go command builds in the pr job's checkout of the default branch: a
// workspace file and its sums, which take other modules in, and a vendor
// directory, which replaces the module cache. The hygiene rule of 10
// 10.2 refuses them, but it runs from the head's own tools/ci, which the
// head may change; pr refuses a head that holds one (invariant (c) of
// T005's acceptance), so none reaches the default branch.
var goModeFiles = []string{"go.work", "go.work.sum", "vendor"}

// headFindings applies the rules of the head that are not the approval
// lines: the files of goModeFiles (headGoFiles) and the jobs of the
// head's workflows (headJobs).
func headFindings(ctx context.Context, r git.Repo, head string) ([]Finding, error) {
	findings, err := headGoFiles(ctx, r, head)
	if err != nil {
		return nil, err
	}
	jobs, err := headJobs(ctx, r, head)
	if err != nil {
		return nil, err
	}
	return append(findings, jobs...), nil
}

// headGoFiles returns a finding for each path of goModeFiles the head's
// tree holds at its top, of any kind.
func headGoFiles(ctx context.Context, r git.Repo, head string) ([]Finding, error) {
	out, err := read(ctx, r, append([]string{"ls-tree", "-z", "--name-only", head, "--"}, goModeFiles...)...)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for name := range strings.SplitSeq(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if name != "" {
			findings = append(findings, Finding{Where: name, Msg: "the head holds " + name + ", which the go command reads from the checkout; it is refused at the top of the tree (10 10.2, hygiene)"})
		}
	}
	return findings, nil
}

// headJobs applies REQ-1154 and REQ-1155 to the workflow files of the
// head. A required check is matched by its name, and a workflow of the
// head other than pr.yml can run from the head (on pull_request, or on
// push to the head's branch) and post a check on the head commit, so:
//
//   - a job of such a file whose check name (its name, else its id)
//     equals JobName, without case and spaces around it, fails;
//   - a job of such a file whose name holds "${{" fails, since the check
//     name it posts is known only once the run expands the expression,
//     unless the literal text before the first "${{", without spaces
//     around it, is not empty and is no prefix of JobName: an expansion
//     only adds text after that prefix, so the name can never be
//     JobName (decided on 2026-10-29, provisional, so that a name such as
//     "static on ${{ matrix.os }}" stays; reopened when a measured check
//     name of GitHub differs from that reading, or when the maintainer
//     prefers the literal rule of REQ-1154 with jobs renamed);
//   - the head's pr.yml names no event but pull_request_target, under
//     which GitHub runs the default branch's file and never the head's;
//   - a workflow file that does not parse as one YAML mapping fails, as
//     what pr cannot read it cannot clear.
//
// Every file directly in .github/workflows that ends in .yml or .yaml, in
// any case, is read: the two endings GitHub runs.
func headJobs(ctx context.Context, r git.Repo, head string) ([]Finding, error) {
	out, err := read(ctx, r, "ls-tree", "-z", head, "--", workflowsDir+"/")
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for entry := range strings.SplitSeq(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		meta, name, ok := strings.Cut(entry, "\t")
		if ext := strings.ToLower(path.Ext(name)); !ok || (ext != ".yml" && ext != ".yaml") {
			continue
		}
		if f := strings.Fields(meta); len(f) != 3 || f[1] != "blob" {
			findings = append(findings, Finding{Where: name, Msg: "a workflow file of the head is not a file"})
			continue
		}
		src, err := read(ctx, r, "cat-file", "blob", head+":"+name)
		if err != nil {
			return nil, err
		}
		findings = append(findings, workflowFindings(name, src)...)
	}
	return findings, nil
}

// workflowFindings applies the rules of headJobs to one workflow file of
// the head, at path name.
func workflowFindings(name string, src []byte) []Finding {
	doc, err := parseWorkflow(src)
	if err != nil {
		return []Finding{{Where: name, Msg: "a workflow file of the head that pr cannot read: " + err.Error()}}
	}
	if name == PRWorkflow {
		if !onlyEvent(mappingValue(doc, "on"), PREvent) {
			return []Finding{{Where: name, Msg: "the head's " + PRWorkflow + " names an event other than " + PREvent + ", under which the head's file would run and post the check " + JobName + " (REQ-1155)"}}
		}
		return nil
	}
	var findings []Finding
	jobs := mappingValue(doc, "jobs")
	if jobs == nil {
		return nil
	}
	if jobs.Kind != yaml.MappingNode {
		return []Finding{{Where: name, Msg: "the jobs of a workflow file of the head are not a mapping pr can read"}}
	}
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		id, job := jobs.Content[i], jobs.Content[i+1]
		// A name that is absent, null or empty leaves the check named by
		// the id, so the id is judged then.
		check, label := id.Value, "id"
		switch n := mappingValue(job, "name"); {
		case n == nil:
		case n.Kind != yaml.ScalarNode:
			findings = append(findings, Finding{Where: name, Msg: "a job of a workflow file of the head has a name pr cannot read"})
			continue
		case n.Tag != "!!null" && n.Value != "":
			check, label = n.Value, "name"
		}
		if spoofs(check) {
			findings = append(findings, Finding{Where: name, Msg: "a job's " + label + ", quoted below, could post the check " + JobName + " of the pr job from the head (REQ-1154)", Quote: check})
		}
	}
	return findings
}

// spoofs reports whether a check name could be JobName: equal to it
// without case and spaces around it, or holding an expression that the
// literal text before it does not keep from it (headJobs).
func spoofs(check string) bool {
	before, _, expression := strings.Cut(check, "${{")
	if !expression {
		return strings.EqualFold(strings.TrimSpace(check), JobName)
	}
	prefix := strings.TrimSpace(before)
	return prefix == "" || (len(prefix) <= len(JobName) && strings.EqualFold(JobName[:len(prefix)], prefix))
}

// onlyEvent reports whether on names the one event want, in any form
// YAML gives it: one name, a list of one name, or a mapping of one key.
func onlyEvent(on *yaml.Node, want string) bool {
	if on == nil {
		return false
	}
	switch on.Kind {
	case yaml.ScalarNode:
		return on.Value == want
	case yaml.SequenceNode:
		return len(on.Content) == 1 && on.Content[0].Kind == yaml.ScalarNode && on.Content[0].Value == want
	case yaml.MappingNode:
		return len(on.Content) == 2 && on.Content[0].Value == want
	}
	return false
}

// parseWorkflow reads exactly one YAML document whose root is a mapping,
// with no alias, anchor or merge key, whose meaning a reader of the
// nodes would have to resolve.
func parseWorkflow(src []byte) (*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var doc yaml.Node
	// The library's error may quote the file, the author's text, so
	// only the kind of failure is returned.
	if err := dec.Decode(&doc); err != nil {
		return nil, errors.New("not YAML")
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return nil, errors.New("more than one YAML document")
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("not a mapping")
	}
	if indirect(doc.Content[0]) {
		return nil, errors.New("an anchor, an alias, a merge key or a key written twice")
	}
	return doc.Content[0], nil
}

// indirect reports whether a node, or one below it, is an alias, holds
// an anchor or is a merge key, or is a mapping with a key written twice,
// which loaders read in different ways.
func indirect(n *yaml.Node) bool {
	if n.Kind == yaml.AliasNode || n.Anchor != "" || n.Tag == "!!merge" {
		return true
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i].Value
			if seen[k] {
				return true
			}
			seen[k] = true
		}
	}
	return slices.ContainsFunc(n.Content, indirect)
}

// mappingValue returns the value of key in a mapping node, or nil;
// parseWorkflow refuses a key written twice.
func mappingValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
