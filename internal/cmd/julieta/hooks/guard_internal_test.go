// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
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

// TestGuardSplitHoldsQuotes holds the word splitter to the quoting of
// sh: a quoted separator is part of a word, a backslash escapes, and an
// unclosed quote ends the input.
func TestGuardSplitHoldsQuotes(t *testing.T) {
	assert.Equal(t, [][]string{{"a", "b;c", "d|e"}}, splitCommands(`a "b;c" 'd|e'`))
	assert.Equal(t, [][]string{{"a", "b c"}, {"d"}}, splitCommands("a b\\ c;d"))
	assert.Equal(t, [][]string{{"echo", "it's"}}, splitCommands(`echo "it's"`))
	assert.Equal(t, [][]string{{"echo", "x", "unclosed"}}, splitCommands(`echo x 'unclosed`))
	assert.Equal(t, [][]string{{"a"}, {"b"}, {"c"}}, splitCommands("a &\nb & c"))
	assert.Equal(t, [][]string{{"a", ""}}, splitCommands(`a ""`))
}
