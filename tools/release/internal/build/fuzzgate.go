// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// invariantTag is what a file that holds a guard or a test tag of an
// invariant contains (05 5.2, REQ-0472).
const invariantTag = "//romeu:invariant"

// repoName is the form of a GitHub "owner/name".
var repoName = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// commitID is the form of a full commit id.
var commitID = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// DefaultAPIHost is the one host the token goes to unless the caller
// names another (FuzzGate.AllowHost).
const DefaultAPIHost = "api.github.com"

// FuzzGate is step 1 of the publish job (REQ-0902): a build that is not
// a dry run refuses unless the latest scheduled fuzz.yml run on main
// succeeded, its head commit is an ancestor of the release commit and no
// file that holds a guard or test tag changed between the two. It reads
// the run through the GitHub API with Token, the step's GH_TOKEN.
type FuzzGate struct {
	API   string // the API root, https://api.github.com
	Repo  string // owner/name
	Token string // GH_TOKEN
	// AllowHost is the one other host:port the API may be, named by an
	// explicit flag. The API is https and api.github.com otherwise, and
	// the token goes nowhere else, a redirect included.
	AllowHost string
	Client    *http.Client // nil means a client with a 30 second timeout
}

// run is the part of a workflow run the gate reads.
type run struct {
	Event      string `json:"event"`
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// Check judges the release commit that root's HEAD names. Any answer
// but a qualifying run, an API error included, is a refusal.
func (g FuzzGate) Check(ctx context.Context, root string) error {
	latest, err := g.latest(ctx)
	if err != nil {
		return fmt.Errorf("the fuzz run: %w", err)
	}
	if latest.Conclusion != "success" {
		return fmt.Errorf("the latest scheduled fuzz.yml run on main is %s, not success (%s)", orUnknown(latest.Conclusion), orUnknown(latest.Status))
	}
	if err := ancestor(ctx, root, latest.HeadSHA); err != nil {
		return err
	}
	return guardsUnchanged(ctx, root, latest.HeadSHA)
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// latest returns the newest run of fuzz.yml on main whose event is
// schedule. A workflow_dispatch run does not count, whatever the query
// asked for.
func (g FuzzGate) latest(ctx context.Context) (run, error) {
	if !repoName.MatchString(g.Repo) {
		return run{}, fmt.Errorf("repository %q is not owner/name", g.Repo)
	}
	if g.Token == "" {
		return run{}, errors.New("GH_TOKEN is empty")
	}
	base, err := g.apiBase()
	if err != nil {
		return run{}, err
	}
	q := url.Values{"branch": {"main"}, "event": {"schedule"}, "per_page": {"20"}}
	endpoint := base + "/repos/" + g.Repo + "/actions/workflows/fuzz.yml/runs?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return run{}, err
	}
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := &http.Client{Timeout: 30 * time.Second}
	if g.Client != nil {
		client = g.Client
	}
	// A redirect to another host would carry the token with it.
	copied := *client
	copied.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		if next.URL.Scheme != "https" || next.URL.Host != req.URL.Host {
			return fmt.Errorf("the API redirected to %s", next.URL.Redacted())
		}
		return nil
	}
	client = &copied
	resp, err := client.Do(req)
	if err != nil {
		return run{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return run{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return run{}, fmt.Errorf("the API answered %s", resp.Status)
	}
	var answer struct {
		Runs []run `json:"workflow_runs"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return run{}, fmt.Errorf("the API answer: %w", err)
	}
	for _, r := range answer.Runs {
		if r.Event == "schedule" && r.HeadBranch == "main" {
			return r, nil
		}
	}
	return run{}, errors.New("no scheduled fuzz.yml run on main found")
}

// apiBase returns the API root when it is https and on a host the token
// may go to.
func (g FuzzGate) apiBase() (string, error) {
	u, err := url.Parse(g.API)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("the API root %q is not an https URL", g.API)
	}
	if u.Host != DefaultAPIHost && (g.AllowHost == "" || u.Host != g.AllowHost) {
		return "", fmt.Errorf("the API host %s is not %s, and no flag names it", u.Host, DefaultAPIHost)
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}

// ancestor refuses unless fuzzHead is an ancestor of HEAD. A head the
// repository does not hold is a refusal too.
func ancestor(ctx context.Context, root, fuzzHead string) error {
	if !commitID.MatchString(fuzzHead) {
		return fmt.Errorf("the fuzz run's head %q is not a commit id", fuzzHead)
	}
	_, err := git(ctx, root, "merge-base", "--is-ancestor", fuzzHead, "HEAD")
	if err == nil {
		return nil
	}
	if exit, ok := errors.AsType[*procgroup.ExitError](err); ok && exit.Code == 1 {
		return fmt.Errorf("the fuzz run's head %s is not an ancestor of the release commit", fuzzHead)
	}
	return fmt.Errorf("the fuzz run's head %s: %w", fuzzHead, err)
}

// guardsUnchanged refuses when a file that holds a guard or test tag, at
// the fuzz head or at the release commit, differs between the two.
func guardsUnchanged(ctx context.Context, root, fuzzHead string) error {
	changed, err := git(ctx, root, "diff", "--name-only", "--no-renames", "-z", fuzzHead, "HEAD", "--")
	if err != nil {
		return err
	}
	if len(changed) == 0 {
		return nil
	}
	tagged := map[string]bool{}
	for _, rev := range []string{fuzzHead, "HEAD"} {
		out, err := git(ctx, root, "grep", "-l", "-z", "--fixed-strings", "-e", invariantTag, rev, "--")
		if exit, ok := errors.AsType[*procgroup.ExitError](err); ok && exit.Code == 1 {
			continue
		}
		if err != nil {
			return err
		}
		for name := range bytes.SplitSeq(out, []byte{0}) {
			if _, file, ok := strings.Cut(string(name), ":"); ok {
				tagged[file] = true
			}
		}
	}
	var hit []string
	for name := range bytes.SplitSeq(changed, []byte{0}) {
		if tagged[string(name)] {
			hit = append(hit, string(name))
		}
	}
	slices.Sort(hit)
	if len(hit) > 0 {
		return fmt.Errorf("files with a guard or test tag changed since the fuzz run: %s", strings.Join(hit, ", "))
	}
	return nil
}
