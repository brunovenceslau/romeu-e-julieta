// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package git

import (
	"os"
	"slices"
	"strings"
)

// LocalEnv are the variables that tell git which repository, index,
// object store, work tree or configuration to use: the names "git
// rev-parse --local-env-vars" lists (git 2.53.0; TestCleanEnv compares
// the list with the git that runs the tests). Git exports such
// variables to a hook, and the githooks page clears this same list
// before a hook runs git on another tree
// (https://git-scm.com/docs/githooks).
var LocalEnv = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
	"GIT_OBJECT_DIRECTORY", "GIT_DIR", "GIT_WORK_TREE", "GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE",
	"GIT_INDEX_FILE", "GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE", "GIT_PREFIX", "GIT_SHALLOW_FILE",
	"GIT_COMMON_DIR",
}

// CleanEnv returns base (this process's environment when nil) without
// the variables of LocalEnv and without every variable whose name
// starts with GIT_CONFIG (the "git -c" and GIT_CONFIG_COUNT settings
// among them), so that git finds the repository from its working
// directory, with that repository's own index, objects and
// configuration. It adds GIT_OPTIONAL_LOCKS=0, in place of the
// caller's, so that "git status" does not write the index
// (https://git-scm.com/docs/git#Documentation/git.txt-GITOPTIONALLOCKS).
// A git command that a hook, or a tool started from a hook, runs on a
// repository it names by directory takes this environment.
func CleanEnv(base []string) []string {
	if base == nil {
		base = os.Environ()
	}
	env := slices.DeleteFunc(slices.Clone(base), func(kv string) bool {
		key, _, _ := strings.Cut(kv, "=")
		return slices.Contains(LocalEnv, key) || strings.HasPrefix(key, "GIT_CONFIG_") || key == "GIT_OPTIONAL_LOCKS"
	})
	return append(env, "GIT_OPTIONAL_LOCKS=0")
}
