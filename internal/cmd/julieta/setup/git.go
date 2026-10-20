// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// gitPaths are where julieta looks for git: the directories of the
// PATH every julieta git call runs with (gitsafe, sandbox mode), so no
// PATH the agent sets chooses the program.
var gitPaths = []string{"/usr/bin/git", "/bin/git"}

// NewGit returns the gitsafe runner of julieta, in sandbox mode with
// HOME home (05 5.1, REQ-0470).
func NewGit(home string) (*gitsafe.Runner, error) {
	for _, p := range gitPaths {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return gitsafe.New(gitsafe.Config{Mode: gitsafe.Sandbox, Git: p, Home: home})
		}
	}
	return nil, fmt.Errorf("no git at %s", strings.Join(gitPaths, " or "))
}

// The forms of 03 3.1 a repo of the manifest is checked against before
// its values become a path or a URL (REQ-0271): a project name
// (REQ-0175) and a repo dir (REQ-0176), with the names romeu reserves.
var (
	projectForm = regexp.MustCompile(`^[a-z][a-z0-9-]{0,38}[a-z0-9]$`)
	dirForm     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
	reserved    = []string{"memory", "review", "ledger", ".romeu", "sbxenv.yaml", ".git"}
)

// CheckRepo returns why the repo r of the manifest of project cannot be
// used in a sandbox whose HOME is home, or nil: a dir that is not a
// repo dir of 03 3.1, a path that is not absolute and clean, a
// secondary whose path is not home/src/<project>/<dir> (REQ-0221), and
// a URL gitsafe refuses.
func CheckRepo(home, project string, r manifest.Repo) error {
	if !dirForm.MatchString(r.Dir) || r.Dir == "." || r.Dir == ".." || slices.Contains(reserved, strings.ToLower(r.Dir)) {
		return fmt.Errorf("the dir %q is not a repo dir of 03 3.1", r.Dir)
	}
	if !filepath.IsAbs(r.Path) || filepath.Clean(r.Path) != r.Path {
		return fmt.Errorf("the path %q is not absolute and clean", r.Path)
	}
	if !r.Primary {
		if !projectForm.MatchString(project) {
			return fmt.Errorf("the project %q is not a project name of 03 3.1", project)
		}
		if want := filepath.Join(home, "src", project, r.Dir); r.Path != want {
			return fmt.Errorf("the path %q of a secondary repo is not %s", r.Path, want)
		}
	}
	return gitsafe.CheckURL(r.URL)
}
