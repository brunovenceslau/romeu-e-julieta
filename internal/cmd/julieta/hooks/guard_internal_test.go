// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGuardRules holds the three patterns of 06 6.4 (REQ-0598) to
// commands as an agent writes them: alone, in a list or a pipeline,
// behind assignments and the common wrappers, in a command
// substitution and in sh -c; and holds the commands that only name
// them, or do something close, to no refusal.
func TestGuardRules(t *testing.T) {
	tests := []struct {
		command string
		want    string // the rule, "" for none
	}{
		// gh pr merge
		{"gh pr merge 12", ruleMerge},
		{"gh pr merge --squash --delete-branch", ruleMerge},
		{"cd repo && gh pr merge 1 --auto", ruleMerge},
		{"echo x; gh pr merge 1", ruleMerge},
		{"git push || gh pr merge 1", ruleMerge},
		{"GH_TOKEN=x gh pr merge 1", ruleMerge},
		{"/usr/bin/gh pr merge 1", ruleMerge},
		{"sudo -u agent gh pr merge 1", ruleMerge},
		{"env -u X FOO=1 gh pr merge 1", ruleMerge},
		{"timeout -s KILL 30 gh pr merge 1", ruleMerge},
		{"nice -n 5 nohup gh pr merge 1", ruleMerge},
		{"command gh pr merge 1", ruleMerge},
		{"echo 1 | xargs gh pr merge", ruleMerge},
		{"gh pr list -q .[].number | xargs -n 1 gh pr merge --squash", ruleMerge},
		{"(gh pr merge 1)", ruleMerge},
		{"{ gh pr merge 1; }", ruleMerge},
		{"if true; then gh pr merge 1; fi", ruleMerge},
		{"x=$(gh pr merge 1)", ruleMerge},
		{"echo \"$(gh pr merge 1)\"", ruleMerge},
		{"echo `gh pr merge 1`", ruleMerge},
		{"bash -c 'gh pr merge 1'", ruleMerge},
		{"sh -ec \"gh pr merge 1\"", ruleMerge},
		{"eval gh pr merge 1", ruleMerge},
		{"g\\h pr merge 1", ruleMerge},
		{"gh pr\nmerge 1", ""},
		{"gh \\\n  pr merge 5", ruleMerge},
		{"git push \\\n  origin \\\n  v1.0", ruleTag},
		{"gh api \\\n  -X PUT \\\n  repos/o/r/rulesets/1 \\\n  --input r.json", ruleSettings},
		{"gh api -fname=x repos/o/r", ruleSettings},
		{"gh api -X DELETE 'repos/o/r/rulesets?x=1'", ruleSettings},
		{"gh pr view 1", ""},
		{"gh pr create --title 'gh pr merge'", ""},
		{"echo 'gh pr merge 1'", ""},
		{"git commit -m \"run gh pr merge later\"", ""},
		{"grep -rn 'gh pr merge' docs | head", ""},
		{"gh pr list --search merge", ""},
		{"gh help pr merge", ""},

		// a push of a v* tag
		{"git push origin v1.2.0", ruleTag},
		{"git push origin refs/tags/v1.2.0", ruleTag},
		{"git push origin refs/tags/vnext", ruleTag},
		{"git push origin tag v1.0", ruleTag},
		{"git push --tags", ruleTag},
		{"git push origin --tags", ruleTag},
		{"git push --follow-tags origin main", ruleTag},
		{"git push --mirror backup", ruleTag},
		{"git -C repo -c push.default=simple push origin v2", ruleTag},
		{"git --git-dir=.git push origin v2", ruleTag},
		{"git push origin +refs/tags/v1:refs/tags/v1", ruleTag},
		{"git push origin HEAD:refs/tags/v3", ruleTag},
		{"git push origin :refs/tags/v1", ruleTag},
		{"git push --delete origin v1.0.0", ruleTag},
		{"git push -o ci.skip origin v1.0.0", ruleTag},
		{"git push origin main && git push origin v1.0.0", ruleTag},
		{"git push origin main", ""},
		{"git push -u origin feat/v2-thing", ""},
		{"git push origin vendor-update", ""},
		{"git push origin refs/tags/release-1", ""},
		{"git push origin v2-fix", ""},
		{"git push origin HEAD:v2-fix", ""},
		{"git push origin v2-fix:refs/tags/v2-fix", ruleTag},
		{"git push origin 'refs/tags/*'", ruleTag},
		{"git push origin 'refs/tags/*:refs/tags/*'", ruleTag},
		{"git push origin v1", ruleTag},
		{"git push origin v1.2.3-rc.1", ruleTag},
		{"git push origin v1.2.3+build.5", ruleTag},
		{"git push origin \"v${VERSION}\"", ruleTag},
		{"git push origin tag release-1", ""},
		{"git tag v1.0", ""},
		{"git push --force-with-lease origin HEAD", ""},
		{"git log --tags", ""},
		{"echo git push origin v1.0", ""},

		// gh api to rulesets and repository settings
		{"gh api -X POST repos/o/r/rulesets --input r.json", ruleSettings},
		{"gh api repos/{owner}/{repo}/rulesets/1 -X PUT --input r.json", ruleSettings},
		{"gh api --method=DELETE /repos/o/r/rulesets/1", ruleSettings},
		{"gh api --method DELETE orgs/o/rulesets/3", ruleSettings},
		{"gh api -XPATCH repos/o/r -f allow_auto_merge=true", ruleSettings},
		{"gh api repos/o/r -F delete_branch_on_merge=true", ruleSettings},
		{"gh api -X PUT repos/o/r/branches/main/protection --input p.json", ruleSettings},
		{"gh api -X DELETE repos/o/r/branches/main/protection/required_signatures", ruleSettings},
		{"gh api -X PUT repos/o/r/actions/permissions -F enabled=true", ruleSettings},
		{"gh api -X PUT repos/o/r/collaborators/someone", ruleSettings},
		{"gh api -X PUT repos/o/r/environments/prod --input e.json", ruleSettings},
		{"gh api -X PUT repos/o/r/actions/secrets/TOKEN -f encrypted_value=x", ruleSettings},
		{"gh api -X POST repos/o/r/hooks --input h.json", ruleSettings},
		{"gh api -X PUT repos/o/r/topics -f names[]=go", ruleSettings},
		{"gh api -X PUT repos/o/r/vulnerability-alerts", ruleSettings},
		{"gh api graphql -f query='mutation { createRepositoryRuleset(input: {}) { ruleset { id } } }'", ruleSettings},
		{"gh api graphql -F query=@m.graphql -f q='mutation{updateBranchProtectionRule(input:{}){clientMutationId}}'", ruleSettings},
		{"gh api graphql -f query='mutation { updateRepository(input: {}) { repository { id } } }'", ruleSettings},
		{"gh api -X PUT repos/o/r/actions/variables/X -f value=1", ruleSettings},
		{"gh api -X POST repos/o/r/actions/runners/registration-token", ruleSettings},
		{"gh api -X PUT repos/o/r/actions/oidc/customization/sub --input s.json", ruleSettings},
		{"gh api -X POST repos/o/r/actions/runs/1/rerun", ""},
		{"gh api -X POST repos/o/r/actions/workflows/ci.yml/dispatches -f ref=main", ""},

		// merging, tagging and releasing by another route (decided on 2026-10-25)
		{"gh release create v1.2.0 --notes x", ruleRelease},
		{"gh release delete v1.2.0 --yes", ruleRelease},
		{"gh -R o/r release create v1", ruleRelease},
		{"gh release view v1.2.0", ""},
		{"gh release list", ""},
		{"gh api -X PUT repos/o/r/pulls/5/merge -f merge_method=squash", ruleMerge},
		{"gh api --method PUT /repos/{owner}/{repo}/pulls/5/merge", ruleMerge},
		{"gh api repos/o/r/pulls/5/merge", ""},
		{"gh api graphql -f query='mutation { mergePullRequest(input: {pullRequestId: \"x\"}) { clientMutationId } }'", ruleMerge},
		{"gh api graphql -f query='mutation { enablePullRequestAutoMerge(input: {pullRequestId: \"x\"}) { clientMutationId } }'", ruleMerge},
		{"gh api -X POST repos/o/r/git/refs -f ref=refs/tags/v1.2.0 -f sha=abc", ruleTag},
		{"gh api repos/o/r/git/refs -f ref=refs/tags/v1.2.0 -f sha=abc", ruleTag},
		{"gh api -X DELETE repos/o/r/git/refs/tags/v1.2.0", ruleTag},
		{"gh api -X POST repos/o/r/git/refs -f ref=refs/heads/feat -f sha=abc", ""},
		{"gh api repos/o/r/git/refs/tags/v1.2.0", ""},
		{"gh api -X POST repos/o/r/releases -f tag_name=v1.2.0", ruleRelease},
		{"gh api -X DELETE repos/o/r/releases/42", ruleRelease},
		{"gh api -X PATCH repos/o/r/releases/42 -F draft=false", ruleRelease},
		{"gh api repos/o/r/releases/latest", ""},
		{"echo 1 | xargs -I{} gh api -X PUT repos/o/r/pulls/{}/merge", ruleMerge},

		{"gh api repos/o/r", ""},
		{"gh api repos/o/r/rulesets", ""},
		{"gh api -X GET repos/o/r/rulesets -f includes_parents=true", ""},
		{"gh api repos/o/r/branches/main/protection", ""},
		{"gh api repos/o/r/pulls/1/comments -f body=x", ""},
		{"gh api -X POST repos/o/r/issues -f title=t", ""},
		{"gh api -X POST repos/o/r/branches/main/rename -f new_name=trunk", ""},
		{"gh api graphql -f query='query { repository(owner: \"o\", name: \"r\") { id } }'", ""},
		{"gh api --jq .rulesets repos/o/r/rules/branches/main", ""},
		{"echo gh api -X DELETE repos/o/r/rulesets/1", ""},
	}
	for _, tt := range tests {
		got := ""
		if r := guardCheck(tt.command); r != nil {
			got = r.rule
		}
		assert.Equal(t, tt.want, got, "%q", tt.command)
	}
}

// TestGuardReadsEveryCommand holds the walk of the parsed script
// (mvdan.cc/sh/v3/syntax): each refused command is refused wherever a
// shell runs it, in a command substitution, in backquotes, in a
// subshell, in a function body, after a heredoc whose body holds an
// apostrophe, after a comment that holds one, and behind a leading
// redirection; a heredoc body or a comment that only names it is no
// command.
func TestGuardReadsEveryCommand(t *testing.T) {
	refused := map[string]string{
		"gh pr merge 1":                         ruleMerge,
		"git push origin v1.2.0":                ruleTag,
		"gh api -X DELETE repos/o/r/rulesets/1": ruleSettings,
		"gh release create v1.2.0":              ruleRelease,
	}
	wraps := []string{
		`echo "$(%s)"`,
		"echo `%s`",
		"(%s)",
		"f() { %s; }; f",
		`echo "$(date)"; %s`,
		`X="$(git rev-parse HEAD)" && %s`,
		`echo "a" "$(true)" ; %s`,
		"cat <<EOF\nit's here\nEOF\n%s",
		"cat <<'EOF'\nit's here\nEOF\n%s",
		"# don't\n%s",
		"true # don't do it\n%s",
		">/dev/null %s",
		"2>&1 %s | cat",
		"cat <<EOF\n$(%s)\nEOF",
	}
	for cmd, rule := range refused {
		for _, w := range wraps {
			text := strings.ReplaceAll(w, "%s", cmd)
			r := guardCheck(text)
			if assert.NotNil(t, r, "%q", text) {
				assert.Equal(t, rule, r.rule, "%q", text)
			}
		}
	}
	for _, text := range []string{
		"cat <<EOF\ngh pr merge 1\nEOF",
		"cat <<'EOF' >notes.md\ngit push origin v1.2.0\nEOF",
		"# gh pr merge 1",
		"true # gh pr merge 1",
		"echo hi\n# gh release create v1",
	} {
		assert.Nil(t, guardCheck(text), "%q", text)
	}
}

// TestGuardFailsClosedOnAParseError holds a command the parser refuses
// to a refusal that says so.
func TestGuardFailsClosedOnAParseError(t *testing.T) {
	for _, text := range []string{"echo 'unclosed", "if then fi (", "bash -c 'echo \"x'"} {
		r := guardCheck(text)
		if assert.NotNil(t, r, "%q", text) {
			assert.Equal(t, ruleUnparsable, r.rule, "%q", text)
			assert.Contains(t, r.text, "cannot be parsed", "%q", text)
			assert.Contains(t, r.text, "fails closed", "%q", text)
		}
	}
}
