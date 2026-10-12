// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package goenv holds the lists the environment of a go command is
// built from, in tools/ci (its steps, mise, the pre-push hook) and in
// tools/mutants: the environment is built from nothing, so a variable
// of the caller that changes what is built or tested has no effect.
package goenv

// PassThrough are the variables of this process that the steps of fast
// and "mise which" keep, and the pre-push hook with them (hookAllow in
// tools/ci), and that tools/mutants keeps for its go commands. Each one
// says where a program or a file is, never what to check or how:
//   - PATH: finds mise and go; HOME: the installs of mise, the caches and
//     the git configuration; TMPDIR: temporary files;
//   - XDG_CACHE_HOME, XDG_DATA_HOME, XDG_STATE_HOME: where mise and go
//     keep their files when the person moved them. XDG_CONFIG_HOME is
//     not here: it would let a caller's git/config (core.fsmonitor, say)
//     run inside the git calls of the hook, and mise reads no
//     configuration there (miseNoGlobalConfig in tools/ci);
//   - MISE_DATA_DIR, MISE_CACHE_DIR, MISE_STATE_DIR: the same for mise;
//   - GOPATH, GOCACHE, GOMODCACHE: where the module cache and the build
//     cache are, so the first go run finds the modules.
var PassThrough = []string{
	"PATH", "HOME", "TMPDIR",
	"XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME",
	"MISE_CACHE_DIR", "MISE_DATA_DIR", "MISE_STATE_DIR",
	"GOPATH", "GOCACHE", "GOMODCACHE",
}

// Pin holds the two variables that keep the go command on the pinned
// go and on the process's own environment: GOENV=off (no "go env -w"
// file is read) and GOTOOLCHAIN=local (no other go is downloaded or
// run). It is the one list that stepEnv and miseEnv of tools/ci (every
// mise run, "mise install" and its builds of govulncheck and gopls
// among them) and the go commands of tools/mutants all hold.
var Pin = []string{
	"GOENV=off",
	"GOTOOLCHAIN=local",
}

// Build holds the two variables that keep the go command on the
// modules of the tree: GOWORK=off (no go.work file, in the tree or
// above it, changes them) and GOFLAGS=-mod=readonly (it builds from
// go.mod and the module cache, never from a vendor directory, and the
// caller's GOFLAGS is replaced; an empty one would not do, since go
// then reads the "go env -w" file unless GOENV=off). It is the one list
// that stepEnv of tools/ci, the pre-push hook (hookScript in the tests
// of tools/ci, held to the tracked hook by TestHook) and tools/mutants
// all hold. GOPROXY=off is not in it: the hook's first go run fills the
// module cache. The steps of fast and the go commands of tools/mutants
// add GOPROXY=off themselves.
var Build = []string{
	"GOWORK=off",
	"GOFLAGS=-mod=readonly",
}
