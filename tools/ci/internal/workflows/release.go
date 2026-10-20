// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package workflows

import (
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The release publish job row of 10 10.2 (REQ-0808). In release.yml
// only, the job named publishJob (the publish job of 10 10.2, Release and
// bootstrap) is held to the rules below, and only it may:
//
//   - set contents, id-token and attestations to write in its
//     permissions;
//   - give a run step of tools/release build, notes, verify or publish
//     an env with the one key GH_TOKEN: ${{ github.token }}; and
//   - name actions/attest-build-provenance.
//
// Each is refused in any other file, job or step, and the tools/ci setup
// step of the job holds no env. The job runs on ubuntu-26.04, needs
// every other job of the file (the test jobs), and runs no test: its one
// tools/ci step is setup, and it runs before any tools/release step.
// GOPROXY=off needs no key here: the file's env is mise.MiseEnv alone and
// no step holds another, so the steps of the job get the environment
// tools/ci builds for them.
const (
	publishJob  = "publish"
	releaseFile = "release.yml"
	// attestAction is the action that attests the archives (REQ-0906).
	attestAction = "actions/attest-build-provenance"
	// tokenValue is the one value an env of the job may hold, for the one
	// key tokenKey.
	tokenKey   = "GH_TOKEN"
	tokenValue = "${{ github.token }}"
	// publishRunner is the one label the publish job runs on.
	publishRunner = "ubuntu-26.04"
)

// publishWrites are the scopes the publish job may set to write.
var publishWrites = []string{"contents", "id-token", "attestations"}

// publishActions are the only actions the publish job names, each a key
// of actionInputs.
var publishActions = []string{checkoutAction, miseAction, attestAction}

// tokenSubcommands are the tools/release subcommands whose step may hold
// the env of the row.
var tokenSubcommands = []string{"build", "notes", "verify", "publish"}

// setupLine is the one tools/ci command of the publish job.
const setupLine = "go run ./tools/ci setup"

// release is what a release.yml grants its publish job. A nil release
// grants nothing, which is every other file.
type release struct {
	job    *yaml.Node
	steps  map[*yaml.Node]bool // every step of the job
	tokens map[*yaml.Node]bool // the steps that may hold the env
	values map[*yaml.Node]bool // the GH_TOKEN values that are exactly tokenValue
}

// readRelease returns the grants of the workflow at path, or nil when
// the file is no release.yml or holds no publish job.
func readRelease(path string, doc *yaml.Node) *release {
	if strings.TrimPrefix(path, Dir) != releaseFile {
		return nil
	}
	job := field(field(doc, "jobs"), publishJob)
	if job == nil || job.Kind != yaml.MappingNode {
		return nil
	}
	r := &release{job: job, steps: map[*yaml.Node]bool{}, tokens: map[*yaml.Node]bool{}, values: map[*yaml.Node]bool{}}
	for _, s := range seqContent(field(job, "steps")) {
		r.steps[s] = true
		if sub, ok := releaseCommand(s); ok && slices.Contains(tokenSubcommands, sub) {
			r.tokens[s] = true
			if v := tokenNode(field(s, "env")); v != nil {
				r.values[v] = true
			}
		}
	}
	return r
}

// releaseCommand returns the subcommand of a run step whose line is a
// well formed go run ./tools/release line.
func releaseCommand(step *yaml.Node) (sub string, ok bool) {
	run := field(step, "run")
	if run == nil || run.Kind != yaml.ScalarNode || !runLine(run.Value) ||
		!strings.HasPrefix(run.Value, strings.Join(runPrograms[1], " ")+" ") {
		return "", false
	}
	// runLine holds at least four words, so the subcommand is there.
	return strings.Split(run.Value, " ")[3], true
}

// tokenNode returns the value of an env that is exactly
// GH_TOKEN: ${{ github.token }}, and nil for any other env.
func tokenNode(env *yaml.Node) *yaml.Node {
	if env == nil || env.Kind != yaml.MappingNode || len(env.Content) != 2 {
		return nil
	}
	k, v := env.Content[0], env.Content[1]
	if k.Kind == yaml.ScalarNode && k.Value == tokenKey && v.Kind == yaml.ScalarNode && v.Tag == "!!str" && v.Value == tokenValue {
		return v
	}
	return nil
}

// isJob reports whether n is the publish job.
func (r *release) isJob(n *yaml.Node) bool { return r != nil && n == r.job }

// inJob reports whether step is a step of the publish job.
func (r *release) inJob(step *yaml.Node) bool { return r != nil && r.steps[step] }

// mayHoldEnv reports whether step is a step that may hold the env.
func (r *release) mayHoldEnv(step *yaml.Node) bool { return r != nil && r.tokens[step] }

// isToken reports whether n is a GH_TOKEN value the row admits, which
// the expression rules then leave alone.
func (r *release) isToken(n *yaml.Node) bool { return r != nil && r.values[n] }

// stepEnv checks the env of a step that may hold one: the one key
// GH_TOKEN, valued ${{ github.token }}. Anything else, as a second key
// or a value read from the event, is a finding.
func (c *grammar) stepEnv(env *yaml.Node) {
	if tokenNode(env) == nil {
		c.add(env, "the env of a release step is the one key "+tokenKey+": "+tokenValue)
	}
}

// publishPermissions checks the permissions of the publish job: read or
// none for any scope, and write for the publishWrites alone.
func (c *grammar) publishPermissions(n *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		c.add(n, "permissions is a mapping of scopes, with no read-all or write-all")
		return
	}
	for scope, level := range pairs(n) {
		ok := level.Kind == yaml.ScalarNode && (level.Value == "read" || level.Value == "none" ||
			(level.Value == "write" && slices.Contains(publishWrites, scope.Value)))
		if !ok {
			c.add(level, "a permission of the publish job is read or none, or write for "+strings.Join(publishWrites, ", "))
		}
	}
}

// publishUses refuses, in the publish job, an action that is not one of
// publishActions. The whole path before the "@" is compared, so a path
// inside an allowed action (actions/attest-build-provenance/predicate)
// is another action and does not borrow the inputs of the main one.
func (c *grammar) publishUses(step, uses *yaml.Node) {
	if !c.rel.inJob(step) {
		return
	}
	path, _, _ := strings.Cut(uses.Value, "@")
	if !slices.Contains(publishActions, strings.ToLower(path)) {
		c.add(uses, "the publish job names only the actions checkout, mise and attest-build-provenance, by their own path")
	}
}

// releaseTrigger holds release.yml to REQ-0899: it releases a v* tag, so
// on is a mapping of the one event push, which holds tags alone.
func (c *grammar) releaseTrigger(doc *yaml.Node) {
	on := field(doc, "on")
	push := field(on, "push")
	ok := on != nil && on.Kind == yaml.MappingNode && len(on.Content) == 2 && push != nil && push.Kind == yaml.MappingNode
	for key := range pairs(push) {
		ok = ok && key.Value == "tags"
	}
	if !ok {
		n := on
		if n == nil {
			n = doc
		}
		c.add(n, releaseFile+" runs on push of tags alone (REQ-0899): on holds the one event push, which holds tags only")
	}
}

// attest refuses actions/attest-build-provenance outside the publish job
// of release.yml.
func (c *grammar) attest(step, uses *yaml.Node) {
	if actionName(uses) == attestAction && !c.rel.inJob(step) {
		c.add(step, attestAction+" is used by the publish job of "+releaseFile+" only")
	}
}

// publish checks the publish job of release.yml as a whole: its runner,
// its needs, and the order and kind of its tools commands. A release.yml
// with no publish job is a finding.
func (c *grammar) publish(doc *yaml.Node) {
	c.releaseTrigger(doc)
	jobs := field(doc, "jobs")
	if c.rel == nil {
		if jobs == nil {
			jobs = doc
		}
		c.add(jobs, releaseFile+" holds a job named "+publishJob)
		return
	}
	job := c.rel.job
	if runsOn := field(job, "runs-on"); runsOn == nil || runsOn.Value != publishRunner {
		c.add(job, "the publish job runs on "+publishRunner)
	}
	var tests []string
	for key := range pairs(jobs) {
		if key.Value != publishJob {
			tests = append(tests, key.Value)
		}
	}
	needs := field(job, "needs")
	var got []string
	for _, n := range seqContent(needs) {
		got = append(got, n.Value)
	}
	slices.Sort(got)
	slices.Sort(tests)
	if needs == nil || needs.Kind != yaml.SequenceNode || !slices.Equal(got, tests) {
		c.add(job, "the publish job needs every other job of the file, the test jobs, as a list")
	}
	setups, released := 0, false
	for _, s := range seqContent(field(job, "steps")) {
		run := field(s, "run")
		if run == nil || run.Kind != yaml.ScalarNode {
			continue
		}
		switch _, isRelease := releaseCommand(s); {
		case isRelease:
			released = true
			if c.rel.mayHoldEnv(s) {
				c.tokenArgs(run)
			}
		case run.Value == setupLine:
			setups++
			if released {
				c.add(s, "tools/ci setup runs before any tools/release step")
			}
		default:
			c.add(s, "the publish job runs tools/release and the one tools/ci step, setup, and no test")
		}
	}
	if setups != 1 {
		c.add(job, "the publish job runs "+setupLine+" once")
	}
}

// tokenArgs refuses, in a step that holds the token, any variable word
// but "$GITHUB_REF_NAME": the token must not reach the command line.
func (c *grammar) tokenArgs(run *yaml.Node) {
	for _, w := range strings.Split(run.Value, " ")[4:] {
		if runVar.MatchString(w) && w != `"$GITHUB_REF_NAME"` {
			c.add(run, "a step that holds the token takes no variable word but \"$GITHUB_REF_NAME\"")
			return
		}
	}
}
