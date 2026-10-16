// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package license

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// reuseImage is the container image of the license step: reuse 6.2.0,
// with its Python and every dependency, pinned by the digest of the
// image (12 12.1). mise cannot pin reuse for every platform: reuse
// publishes one wheel, for linux x86_64, and the mise pypi backend
// does not build the others from source.
const reuseImage = "fsfe/reuse:6.2.0@sha256:85462a75c0f8efda09ddd190b92816b70e7662577c8427429e11e1b9f25a992e"

// GitCommonDir returns the absolute path of the git directory that
// holds the repository's objects and configuration: root/.git in a
// clone, and the main clone's .git in a linked worktree.
func GitCommonDir(ctx context.Context, root string) (string, error) {
	out, err := git.Repo{Dir: root}.Run(ctx, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// Step runs "reuse lint" (REUSE 3.3, S11) from reuseImage on
// linux, the platform its image is built for, with the working tree
// and the git directory mounted read-only, each at its own path, and no
// network for the container. The container runs as the user nobody
// (65534), with no capability, no way to gain privileges, and a
// read-only root file system. Its git is told that both paths are safe:
// the tree belongs to another user, and without it git refuses the
// repository and reuse reads ignored files too (measured with reuse
// 6.2.0: an ignored file without a header then fails). The git
// directory is mounted because a linked worktree's ".git" file points
// to it, outside the tree: without it git fails the same way, and a
// worktree is where agents work (measured with reuse 6.2.0: an ignored
// file without a header failed there). In a clone it lies inside the
// tree, and the second mount repeats a part of the first. On another
// platform the step is not run, and all says so in its output with a
// "--" line instead of "ok": the check reads file content only, which
// is the same on every platform, and the Linux runners of the same
// commit run it.
//
// Threat model of the step. The image is named by its digest, so the
// code that runs is the code that was reviewed, whatever the tag points
// at later. The container reads the tree and the git directory through
// read-only mounts, so it cannot change what the other checks read, and
// it has no network, so it can neither fetch code nor send what it
// reads anywhere. It runs before unit and race (allSteps), so no test
// of the change can rewrite a file before reuse reads it, and runAll
// fails a run whose steps left the tree changed. The code inside the
// image is out of scope, as the code of a pinned action is: the review
// of the pin covers it.
func Step(root, gitCommon, goos string, env []string) check.Step {
	s := check.Step{Name: "license"}
	if goos != "linux" {
		s.Skip = "not run on " + goos + ": reuse runs from a Linux container image; the hosted Linux runners run it"
		return s
	}
	s.Argv = []string{"docker", "run", "--rm", "--network", "none",
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--read-only", "--user", "65534",
		"--env", "GIT_CONFIG_COUNT=2",
		"--env", "GIT_CONFIG_KEY_0=safe.directory", "--env", "GIT_CONFIG_VALUE_0=" + root,
		"--env", "GIT_CONFIG_KEY_1=safe.directory", "--env", "GIT_CONFIG_VALUE_1=" + gitCommon,
		"--mount", "type=bind,source=" + root + ",target=" + root + ",readonly",
		"--mount", "type=bind,source=" + gitCommon + ",target=" + gitCommon + ",readonly",
		"--workdir", root, reuseImage, "lint"}
	s.Environ = env
	for _, path := range []string{root, gitCommon} {
		if strings.ContainsAny(path, ",=") {
			s.Unavailable = fmt.Errorf("a path holds \",\" or \"=\", which the bind mount of the license step cannot name: %s", path)
		}
	}
	return s
}

// Steps returns the license step for the tree at p.Root, in the
// environment of the steps with the Docker daemon's variables
// (check.DockerPassThrough). A git directory that git cannot name makes
// the step unavailable, where it runs at all.
func Steps(ctx context.Context, p check.Plan) []check.Step {
	gitCommon, gitErr := GitCommonDir(ctx, p.Root)
	s := Step(p.Root, gitCommon, runtime.GOOS, check.WithProcessEnv(check.StepEnv(p.Tools.GoDir), check.DockerPassThrough))
	if gitErr != nil && s.Skip == "" {
		s.Unavailable = gitErr
	}
	return []check.Step{s}
}
