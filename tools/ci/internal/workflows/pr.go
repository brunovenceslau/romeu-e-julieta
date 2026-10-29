// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package workflows

import (
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst/approval"
)

// The ask-first job row of 10 10.2 (REQ-0807, REQ-0813) and 12 12.4 (REQ-1149,
// REQ-1150). Under pull_request_target GitHub runs the workflow file of
// the pull request's base branch with a token that may write, so prFile
// alone may name that event, and it is held to one shape:
//
//   - on is the one event pull_request_target, with no branches filter,
//     so a pull request into a stack branch is judged too (the ruleset
//     of main requires the check on every pull request of a stack;
//     decided on 2026-10-29), whose types list edited, so a change to
//     the title or the body runs ask-first again (REQ-0813), with opened,
//     synchronize and reopened and no other, and no other filter;
//   - permissions, at the top and in the job, is contents: read and
//     nothing else; no step names a secret or github.token (the
//     expression rules of every file);
//   - one job, approval.JobName, named so, with no strategy, no needs
//     and timeout-minutes: 10, whose steps are one checkout
//     of the base branch's commit, which names no ref and no
//     repository (actions/checkout then checks out github.sha, the
//     base branch's commit under this event: src/input-helper.ts at
//     the pinned commit 3d3c42e, lines 79 to 82, read on 2026-10-29),
//     with persist-credentials: false and the whole history of that
//     branch (fetch-depth: 0, so the base and the merge base are in the
//     clone), then the mise step, then the one run line PRLine, which
//     builds and runs the code of pr from that checkout. No other step
//     runs, so no code outside the approvals surface runs in the job
//     (tools/ci setup builds all of tools/ci).
//
// The run line PRLine and the fetch-depth input are refused in any other
// file or job.
const (
	prFile  = "ask-first.yml"
	prEvent = approval.PREvent
	// PRLine is the one command of the ask-first job (REQ-1150).
	PRLine = `go run ./tools/ci/internal/askfirst/cmd/ask-first "$GITHUB_EVENT_PATH"`
	// prDepth is the one fetch-depth of the ask-first job's checkout: the whole
	// history.
	prDepth     = "fetch-depth"
	prDepthNone = "0"
)

// prTypes are the types of the event, sorted, and prTimeout the
// timeout-minutes of the job.
var prTypes = []string{"edited", "opened", "reopened", "synchronize"}

const prTimeout = "10"

// prRow is what ask-first.yml grants its job. A nil prRow grants nothing,
// which is every other file.
type prRow struct {
	job   *yaml.Node
	steps map[*yaml.Node]bool
}

// readPR returns the grants of the workflow at path, or nil when the
// file is no ask-first.yml or holds no job approval.JobName.
func readPR(path string, doc *yaml.Node) *prRow {
	if strings.TrimPrefix(path, Dir) != prFile {
		return nil
	}
	job := field(field(doc, "jobs"), approval.JobName)
	if job == nil || job.Kind != yaml.MappingNode {
		return nil
	}
	r := &prRow{job: job, steps: map[*yaml.Node]bool{}}
	for _, s := range seqContent(field(job, "steps")) {
		r.steps[s] = true
	}
	return r
}

// inJob reports whether step is a step of the ask-first job.
func (r *prRow) inJob(step *yaml.Node) bool { return r != nil && r.steps[step] }

// prOnly refuses the event of the ask-first job in a file other than ask-first.yml.
func (c *grammar) prOnly(name *yaml.Node) {
	if !c.inPR {
		c.add(name, prEvent+" is the event of "+prFile+" alone (the ask-first job row)")
	}
}

// prRead holds the one permission the pr row admits, n, at the top of
// ask-first.yml or in its job, which owner holds: contents: read, and no other
// scope.
func (c *grammar) prRead(n, owner *yaml.Node, where string) {
	ok := n != nil && n.Kind == yaml.MappingNode && len(n.Content) == 2 &&
		n.Content[0].Value == "contents" && n.Content[1].Kind == yaml.ScalarNode && n.Content[1].Value == "read"
	if !ok {
		at := n
		if at == nil {
			at = owner
		}
		c.add(at, where+" of "+prFile+" is contents: read, and nothing else (the ask-first job row)")
	}
}

// prWorkflow holds ask-first.yml to the ask-first job row: its event, its
// permissions, its one job and that job's steps.
func (c *grammar) prWorkflow(doc *yaml.Node) {
	c.prTrigger(doc)
	c.prRead(field(doc, "permissions"), doc, "the permissions")
	jobs := field(doc, "jobs")
	if c.pr == nil || len(jobs.Content) != 2 {
		at := jobs
		if at == nil {
			at = doc
		}
		c.add(at, prFile+" holds one job, "+approval.JobName+", and no other (the ask-first job row)")
	}
	if c.pr == nil {
		return
	}
	job := c.pr.job
	if name := field(job, "name"); name == nil || name.Kind != yaml.ScalarNode || name.Value != approval.JobName {
		c.add(job, "the ask-first job is named "+approval.JobName+", the name of its required check")
	}
	if s := field(job, "strategy"); s != nil {
		c.add(s, "the ask-first job has no strategy (the ask-first job row)")
	}
	if n := field(job, "needs"); n != nil {
		c.add(n, "the ask-first job needs no other job (the ask-first job row)")
	}
	if v := field(job, "timeout-minutes"); v == nil || v.Kind != yaml.ScalarNode || v.Value != prTimeout {
		c.add(job, "the ask-first job has timeout-minutes: "+prTimeout+", so a stuck run does not hold the check (the ask-first job row)")
	}
	c.prRead(field(job, "permissions"), job, "the permissions of the job")
	c.prSteps(job)
}

// prTrigger holds the on of ask-first.yml: the one event prEvent, with types
// that list edited, and no other filter: no branches, so a pull request
// into a stack branch is judged too.
func (c *grammar) prTrigger(doc *yaml.Node) {
	on := field(doc, "on")
	if on == nil || on.Kind != yaml.MappingNode || len(on.Content) != 2 || on.Content[0].Value != prEvent {
		at := on
		if at == nil {
			at = doc
		}
		c.add(at, prFile+" runs on "+prEvent+" alone, written as a mapping (the ask-first job row)")
		return
	}
	event := on.Content[1]
	for key := range pairs(event) {
		if key.Value != "types" {
			c.add(key, "the filter "+key.Value+" of "+prEvent+" is outside the ask-first job row: types alone, so ask-first runs for a pull request into any branch, a stack branch among them")
		}
	}
	var types []string
	for _, t := range seqContent(field(event, "types")) {
		types = append(types, t.Value)
	}
	if !slices.Contains(types, "edited") {
		c.add(on, prFile+" lists edited among its "+prEvent+" types, so a change to the title or body runs ask-first again (REQ-0813)")
	} else if slices.Sort(types); !slices.Equal(types, prTypes) {
		c.add(on, "the "+prEvent+" types of "+prFile+" are opened, synchronize, reopened and edited, so every push to the head is judged (the ask-first job row)")
	}
}

// prSteps holds the steps of the ask-first job: a checkout, the mise step and
// the run line PRLine, in this order, and no other.
func (c *grammar) prSteps(job *yaml.Node) {
	steps := seqContent(field(job, "steps"))
	kinds := make([]string, 0, len(steps))
	for _, s := range steps {
		switch uses, run := field(s, "uses"), field(s, "run"); {
		case uses != nil && uses.Kind == yaml.ScalarNode && actionName(uses) == checkoutAction:
			kinds = append(kinds, "checkout")
			c.prCheckout(s)
		case uses != nil && uses.Kind == yaml.ScalarNode && actionName(uses) == miseAction:
			kinds = append(kinds, "mise")
		case run != nil && run.Kind == yaml.ScalarNode && run.Value == PRLine:
			kinds = append(kinds, "pr")
		default:
			kinds = append(kinds, "other")
		}
	}
	if !slices.Equal(kinds, []string{"checkout", "mise", "pr"}) {
		c.add(job, "the steps of the ask-first job are a checkout, the mise step and the run line "+PRLine+", in this order, and no other (the ask-first job row)")
	}
}

// prCheckout holds the checkout of the ask-first job: it names no ref and no
// repository, so it checks out github.sha, and it fetches the whole
// history (fetch-depth: 0). persist-credentials: false is the rule of
// every checkout (checkout).
func (c *grammar) prCheckout(step *yaml.Node) {
	with := field(step, "with")
	for _, key := range []string{"ref", "repository"} {
		if v := field(with, key); v != nil {
			c.add(v, "the ask-first job checks out the base branch's commit, github.sha, so its checkout names no "+key+", never pull_request.base.sha or the head (the ask-first job row)")
		}
	}
	if v := field(with, prDepth); v == nil || v.Kind != yaml.ScalarNode || v.Tag != "!!int" || v.Value != prDepthNone {
		c.add(step, "the checkout of the ask-first job sets "+prDepth+": "+prDepthNone+", the whole history of the base branch (the ask-first job row)")
	}
}
