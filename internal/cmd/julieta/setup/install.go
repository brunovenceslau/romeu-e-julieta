// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// VersionOnlyTool is the warning version-only-tool (04 4.1, Warnings):
// a tool whose backend the pinned mise exempts from the lock's URL
// check, so mise.lock holds its version and no checksum (07 7.3,
// REQ-0617): "<repo> <tool> ...".
const VersionOnlyTool = "version-only-tool"

// installFile is the file of julieta state that holds the lock-set
// digest of the last install that succeeded (02 2.5, REQ-0154).
const installFile = "install.json"

// installState is installFile: the digest of kind lock-set.
type installState struct {
	LockSet string `json:"lockSet"`
}

// readInstallState returns the digest the last successful install
// recorded, "" when there is none. A record that cannot be read is no
// record: the install runs and writes a new one.
func readInstallState(home string) string {
	data, err := os.ReadFile(filepath.Join(StateDir(home), installFile))
	if err != nil {
		return ""
	}
	var s installState
	if err := json.Unmarshal(data, &s, json.RejectUnknownMembers(true)); err != nil {
		return ""
	}
	return s.LockSet
}

// The states of a repo after Install.
const (
	// InstallDone: mise install ran in the repo.
	InstallDone = "installed"
	// InstallSkipped: the lock-set digest equals the last success.
	InstallSkipped = "skipped, the lock-set is unchanged"
	// InstallNoConfig: the repo has no mise.toml, so it contributes
	// nothing.
	InstallNoConfig = "no mise.toml"
)

// InstalledRepo is what Install did in one repo.
type InstalledRepo struct {
	Dir   string
	State string
}

// InstallOptions are the flags of julieta install.
type InstallOptions struct {
	// Repo is the dir of the one repo to install; empty means every
	// repo of the manifest.
	Repo string
	// Force installs although the lock-set digest equals the last
	// success.
	Force bool
}

// InstallResult is what Install did: each repo, in the order of the
// manifest, and the warnings version-only-tool of its locks.
type InstallResult struct {
	Repos    []InstalledRepo
	Warnings []Warning
}

// Install runs mise install, locked, in each repo of the manifest, or
// in the one opt.Repo names, and records the lock-set digest after a
// success of a run over every repo (04 4.3, REQ-0403, REQ-0618). A run
// over one repo neither skips nor records: its digest would stand for a
// set the next full run is not. Before it runs mise it reads the files of
// each repo and runs the check of lock --check on them, and refuses on
// the same findings (check-findings, RJ-328, exit 6; decided on
// 2026-10-22): a repo with a mise.toml and no mise.lock is lock-missing
// (RJ-340, REQ-0438) and a file that does not parse is lock-unparsed
// (RJ-325), and nothing is installed in either case. With the digest
// unchanged and no Force, mise is not started. The errors of a mise
// that fails are internal/mise's: install-blocked (RJ-341),
// subprocess-timeout (RJ-323) and mise-failed (RJ-351).
func Install(ctx context.Context, home string, m manifest.Manifest, opt InstallOptions) (InstallResult, error) {
	repos := m.Repos
	if opt.Repo != "" {
		repos = nil
		for _, r := range m.Repos {
			if r.Dir == opt.Repo {
				repos = []manifest.Repo{r}
			}
		}
		if repos == nil {
			return InstallResult{}, fmt.Errorf("no repo of the manifest has the dir %q", termsafe.Escape(opt.Repo))
		}
	}
	var res InstallResult
	files := make([]mise.Files, len(repos))
	locks := map[string][]byte{}
	for i, r := range repos {
		if err := CheckRepo(home, m.Project, r); err != nil {
			return InstallResult{}, err
		}
		f, err := readRepo(r)
		if err != nil {
			return InstallResult{}, err
		}
		files[i] = f
		if !f.HasConfig {
			continue
		}
		locks[r.Dir] = f.LockData
		for _, tool := range mise.VersionOnlyTools(f.Config) {
			res.Warnings = append(res.Warnings, Warning{Slug: VersionOnlyTool, Message: fmt.Sprintf(
				"%s %s: mise.lock holds its version and no checksum, so its integrity rests on its backend (07 7.3)",
				termsafe.Escape(r.Dir), termsafe.Escape(tool))})
		}
	}
	digest, err := mise.NewLockSet(locks).Digest()
	if err != nil {
		return InstallResult{}, err
	}
	full := opt.Repo == ""
	skip := full && len(locks) > 0 && !opt.Force && readInstallState(home) == digest
	var d *mise.Driver
	for i, r := range repos {
		state := InstallNoConfig
		switch {
		case !files[i].HasConfig:
		case skip:
			state = InstallSkipped
		default:
			if d == nil {
				if d, err = mise.New(os.LookupEnv, home, repoPaths(m)); err != nil {
					return InstallResult{}, err
				}
			}
			if err := d.Install(ctx, mise.Target{Dir: r.Path, Repo: r.Dir, Sandbox: m.Project, Egress: m.Egress}); err != nil {
				return InstallResult{}, err
			}
			state = InstallDone
		}
		res.Repos = append(res.Repos, InstalledRepo{Dir: r.Dir, State: state})
	}
	if full && len(locks) > 0 && !skip {
		data, err := json.Marshal(installState{LockSet: digest}, json.Deterministic(true))
		if err != nil {
			return InstallResult{}, err
		}
		if err := writeState(home, installFile, data); err != nil {
			return InstallResult{}, fmt.Errorf("julieta state %s: %w", installFile, err)
		}
	}
	return res, nil
}

// readRepo reads and checks the mise files of one repo for Install: none
// at all for a repo that is not cloned; lock-unparsed for a file that
// does not parse or cannot be read as one; lock-missing for a mise.toml
// with no lock; check-findings for what lock --check would find.
func readRepo(r manifest.Repo) (mise.Files, error) {
	dir := termsafe.Escape(r.Dir)
	unparsed := func(err error) error {
		e := cli.LockUnparsed.Wrap(err, "the mise files of "+dir+" cannot be read")
		e.Fill = map[string]string{"<dir>": r.Dir}
		return e
	}
	raw, err := mise.ReadRaw(r.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return mise.Files{}, nil // the repo is not cloned: it has no mise.toml
	}
	if err != nil {
		return mise.Files{}, unparsed(err)
	}
	f, err := raw.Parse()
	if err != nil {
		return mise.Files{}, unparsed(err)
	}
	if !f.HasConfig {
		return f, nil
	}
	if !f.HasLock {
		e := cli.LockMissing.New(fmt.Sprintf("%s has a mise.toml and no mise.lock", dir))
		e.Fill = map[string]string{"<dir>": r.Dir}
		return mise.Files{}, e
	}
	if findings := f.Findings(); len(findings) > 0 {
		var details []cli.Detail
		var slugs []string
		for _, fd := range findings {
			details = append(details, cli.Detail{Key: dir + " " + termsafe.Escape(strings.TrimPrefix(fd.String(), fd.Slug+" ")), Value: fd.Slug})
			slugs = append(slugs, fd.Slug)
		}
		return mise.Files{}, cli.CheckFindings.New(fmt.Sprintf("the mise.lock of %s is not fresh, so nothing is installed: %d findings: %s", dir, len(findings), strings.Join(slugs, ", ")), details...)
	}
	return f, nil
}

// repoPaths returns the repo paths of the manifest, the only
// configuration paths mise trusts (REQ-0426).
func repoPaths(m manifest.Manifest) []string {
	paths := make([]string, len(m.Repos))
	for i, r := range m.Repos {
		paths[i] = r.Path
	}
	return paths
}

// installTools is the step install of setup (04 4.3, REQ-0400): Install
// of every repo. A failure is reported and leaves the sandbox usable,
// as the exit rule of 04 4.3 says: only the PATH link, the dispatcher, a
// secondary clone, the system hooks path and memory check do not.
func installTools(ctx context.Context, r *run) {
	res, err := Install(ctx, r.home, r.m, InstallOptions{})
	for _, w := range res.Warnings {
		r.warn(w.Slug, w.Message)
	}
	if err != nil {
		r.report.Errors = append(r.report.Errors, "install: "+Describe(err))
	}
}

// Describe returns err as one line for a report: its text, escaped,
// and the details of an error of the table, each as "key: value".
func Describe(err error) string {
	parts := []string{termsafe.Escape(err.Error())}
	if e, ok := errors.AsType[*cli.Error](err); ok {
		parts[0] = fmt.Sprintf("%s %s: %s", e.ID, e.Slug, termsafe.Escape(e.Error()))
		for _, d := range e.Details {
			parts = append(parts, termsafe.Escape(d.Key)+": "+termsafe.Escape(d.Value))
		}
		parts = append(parts, "fix: "+termsafe.Escape(e.Hint()))
	}
	return strings.Join(parts, "; ")
}
