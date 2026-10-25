// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
	"encoding/json/v2"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/agent/claude"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// ruleUndecodable is the verdict on a document the guard cannot decode.
const ruleUndecodable = "undecodable"

// GuardSchema is the value of the schema field of the --json document
// of julieta hooks guard.
const GuardSchema = "julieta-hooks-guard/v1"

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.Sub, c.JSON = "julieta", "hooks", "guard", true
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:   "guard",
			Short: "the PreToolUse hook: refuse a merge, a v* tag, a release, and gh api changes to rulesets or repository settings",
			Example: "julieta hooks guard <pre-tool-use.json\n" +
				"julieta hooks guard --json <pre-tool-use.json",
			Long: "The agent runs this as its PreToolUse hook, with the document of the tool call on stdin.\n" +
				"It exits 2, which refuses the call, when the Bash command merges a pull request (gh pr merge, gh api to pulls/<n>/merge, " +
				"or the GraphQL mergePullRequest or enablePullRequestAutoMerge), pushes a v* tag (a ref of refs/tags/v or a glob of refs/tags/, " +
				"a bare release version, after tag, or every tag by --tags, --follow-tags or --mirror, or a tag ref made through gh api), " +
				"creates or deletes a release (gh release create or delete, or gh api to releases), or changes a ruleset or a repository's " +
				"settings through gh api; the refusal on stderr says the action is the maintainer's and names the route, an issue or the handoff. " +
				"It exits 0 for any other call.\n\n" +
				"The command is parsed as bash parses it, so a command inside a list, a pipeline, a subshell, a function or a command " +
				"substitution counts, and a heredoc's text or a comment does not. A document it cannot decode, and a command it cannot parse, " +
				"exit 2 as well: the guard fails closed. It is a guard against mistakes, not against an agent that hides a command or edits its own hook.",
			Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				return runGuard(env)
			},
		}
	}
	return c
}())

// guardVerdict is the --json document of julieta hooks guard.
type guardVerdict struct {
	// Decision is allow or refuse.
	Decision string `json:"decision"`
	// Message is the refusal text, empty on allow.
	Message string `json:"message"`
	// Rule is the pattern that refused the call, or undecodable;
	// empty on allow.
	Rule string `json:"rule"`
	// Schema is GuardSchema.
	Schema string `json:"schema"`
}

// runGuard reads one PreToolUse document on stdin and ends the process
// with 2 on a refusal, which prints its line on stderr, where the agent
// reads why its call was refused (04 4.3, REQ-0410).
func runGuard(env *cli.Env) error {
	v := guardVerdict{Decision: "allow", Schema: GuardSchema}
	data, err := io.ReadAll(io.LimitReader(stdin(), claude.MaxPreToolUse+1))
	if err == nil {
		var doc claude.PreToolUseDoc
		if doc, err = claude.DecodePreToolUse(data); err == nil && doc.Tool == "Bash" {
			if r := guardCheck(doc.Command); r != nil {
				v = guardVerdict{Decision: "refuse", Message: r.text, Rule: r.rule, Schema: GuardSchema}
			}
		}
	}
	if err != nil {
		v = guardVerdict{Decision: "refuse", Rule: ruleUndecodable, Schema: GuardSchema,
			Message: fmt.Sprintf("julieta hooks guard refused the tool call: its PreToolUse document cannot be decoded (%v), and the guard fails closed.", err)}
	}
	if env.JSON {
		b, err := json.Marshal(v, json.Deterministic(true))
		if err != nil {
			return err
		}
		if _, err := env.Stdout.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	if v.Decision == "refuse" {
		_, _ = fmt.Fprintln(env.Stderr, v.Message)
		exit(2)
	}
	return nil
}
