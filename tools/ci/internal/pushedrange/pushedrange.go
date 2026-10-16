// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package pushedrange is the pushed range check of 10 10.2, which runs
// in the pre-push run of fast alone: the forbidden-name check over what
// a push would publish, and the rule of the never-tracked names over the
// paths its commits add.
package pushedrange

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/hygiene"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/pushed"
)

// Range applies the name matcher to what a push would publish:
// the remote ref names, the text of each pushed annotated tag and the
// four readings of each pushed commit. It applies the prologue rule to
// the paths those commits add, which needs no denylist entry.
//
// The push is judged by the denylist at head, the commit judgeCommit
// judged, together with the one at the remote's default branch
// (defaultBranchDenylist): an entry that the pushed commit removes
// stays in force until its removal reaches the default branch through
// review. A finding says which of the two lists holds the name.
// It returns the verdicts of the "pushed range" check: its findings,
// and a note when neither list has an entry, as there is nothing to
// match names with then (the prologue rule ran all the same). A
// denylist at head that cannot be read is a verdict with its error,
// as the runner reports a check that cannot judge what it read; a
// missing or empty one is the finding of hygiene. Any other failure, such
// as a remote that is not configured, is an error.
func Range(ctx context.Context, repo git.Repo, remote string, push pushed.Push, head string) ([]check.Verdict, error) {
	const name = "pushed range"
	atHead, _, err := hygiene.LoadDenylist(ctx, repo, head)
	if err != nil {
		return []check.Verdict{{Name: name, Err: err}}, nil
	}
	atDefault, err := defaultBranchDenylist(ctx, repo, remote)
	if err != nil {
		return nil, err
	}
	var findings []check.Finding
	list := &names.List{}
	list.Merge(atHead)
	list.Merge(atDefault)
	err = pushed.Walk(ctx, repo, remote, push, func(r pushed.Reading) {
		if r.Field == pushed.FieldPath && hygiene.IsPrologue(r.Path) {
			// A path that holds a listed name is not printed.
			where := git.Printable(r.Path)
			if list.Match([]byte(r.Path)) {
				where = "(withheld)"
			}
			findings = append(findings, check.Finding{Where: readingAt(r) + "added path " + where, Rule: "prologue", Msg: hygiene.PrologueMsg})
		}
		if !list.Match(r.Text) {
			return
		}
		listed := "a name listed at HEAD"
		if !atHead.Match(r.Text) {
			listed = "a name listed at the remote's default branch"
		}
		of := readingAt(r)
		switch r.Field {
		case pushed.FieldRef:
			findings = append(findings, check.Finding{Where: "pushed ref " + strconv.Itoa(r.Line), Rule: "name", Msg: "the remote ref name holds " + listed})
		case pushed.FieldMessage:
			findings = append(findings, check.Finding{Where: of + "message line " + strconv.Itoa(r.Line), Rule: "name", Msg: "the line holds " + listed})
		case pushed.FieldAuthor, pushed.FieldCommitter, pushed.FieldTagger:
			findings = append(findings, check.Finding{Where: of + r.Field, Rule: "name", Msg: "the name and email hold " + listed})
		case pushed.FieldTagName:
			findings = append(findings, check.Finding{Where: of + r.Field, Rule: "name", Msg: "the tag's name holds " + listed})
		case pushed.FieldPath:
			findings = append(findings, check.Finding{Where: of + "added path (withheld)", Rule: "name", Msg: "the path holds " + listed})
		case pushed.FieldLine:
			path := git.Printable(r.Path)
			if list.Match([]byte(r.Path)) {
				path = "(path withheld)"
			}
			findings = append(findings, check.Finding{Where: of + path + ":" + strconv.Itoa(r.Line), Rule: "name", Msg: "the added line holds " + listed})
		}
	})
	if err != nil {
		return nil, err
	}
	checked := list.Len() > 0
	var verdicts []check.Verdict
	if checked || len(findings) > 0 {
		verdicts = append(verdicts, check.Verdict{Name: name, Findings: findings})
	}
	if !checked {
		verdicts = append(verdicts, check.Verdict{Name: name, Note: "not checked against names, no denylist entry at HEAD or at the remote's default branch"})
	}
	return verdicts, nil
}

// readingAt says where a reading is: its commit or its tag, and the
// mergetag header it comes from, with the colon and space that follow.
func readingAt(r pushed.Reading) string {
	of := "commit " + r.Commit + ": "
	if r.Tag != "" {
		of = "tag " + r.Tag + ": "
	}
	if r.Mergetag > 0 {
		of += "mergetag " + strconv.Itoa(r.Mergetag) + " "
	}
	return of
}

// defaultBranchDenylist reads the denylist at the default branch of
// remote as this clone knows it: the commit of
// refs/remotes/<remote>/HEAD, which "git fetch" creates when the remote
// has a HEAD and the clone has no such ref yet
// (https://git-scm.com/docs/git-config#Documentation/git-config.txt-remotenamefollowRemoteHEAD)
// and "git remote set-head <remote> --auto" sets
// (https://git-scm.com/docs/git-remote#Documentation/git-remote.txt-set-head).
//
// It fails closed: a remote that is not configured, such as a push to a
// URL, which is never printed, and a ref that is missing, stop the
// push, and so does a denylist there that cannot be read. A default
// branch that holds no denylist adds nothing: the denylist enters the
// repository through a pull request, and until that one is merged the
// default branch has none. The local refs and the git binary are
// trusted, as the toolchain is (ADR 0007, Threat model).
func defaultBranchDenylist(ctx context.Context, repo git.Repo, remote string) (*names.List, error) {
	out, err := repo.Run(ctx, nil, "remote")
	if err != nil {
		return nil, err
	}
	if !slices.Contains(strings.Fields(string(out)), remote) {
		return nil, errors.New("the push names no configured remote, so the denylist of its default branch is not known here; add the remote, run \"git fetch\" for it, and push to it by name")
	}
	commit, err := check.CommitOf(ctx, repo, "refs/remotes/"+remote+"/HEAD")
	if err != nil {
		return nil, fmt.Errorf("the default branch of %s is not known here, and its denylist judges the push; run \"git fetch %s\" (or \"git remote set-head %s --auto\") and push again; if the remote has no commit yet, no fetch can succeed, and the operator makes the first push of a new repository with --no-verify: %w", remote, remote, remote, err)
	}
	list, _, err := hygiene.DenylistAt(ctx, repo, commit)
	if err != nil {
		return nil, fmt.Errorf("%s at the default branch of %s: %w", names.Path, remote, err)
	}
	return list, nil
}

// Judge is the pushed range check of a pre-push run (Range), for the
// push of c.Hook; outside a pre-push run it judges nothing.
func Judge(ctx context.Context, c check.Commit) ([]check.Verdict, error) {
	if c.Hook == nil {
		return nil, nil
	}
	return Range(ctx, c.Repo, c.Hook.Remote, c.Hook.Push, c.ID)
}
