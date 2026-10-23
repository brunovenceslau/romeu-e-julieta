// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// helperCorpus are values of tools.gitCredentialHelper on both sides of
// REQ-1267, and the seeds of FuzzCredentialHelperParity.
var helperCorpus = []string{
	"osxkeychain", "store", "cache", "libsecret", "manager", "manager-core", "a", "0", "x-1",
	strings.Repeat("a", 63), strings.Repeat("a", 64), "OSXKeychain", "-x", "a_b", "a.b", ".hidden", "a b",
	"/usr/local/bin/git-credential-manager", "/opt/me/bin/git-credential-gh", "/opt/a+b/c-d_e.f",
	"/", "/a/", "/a//b", "/a/./b", "/a/../b", "/a b", "/a$(id)", "/a;id", "/a`id`", "/a'b", `/a"b`, "/a\\b",
	"/a\nb", "/a\x00b", "/a\tb", "/é", "a/b", "./a", "~/bin/h", "!gh auth git-credential",
	"!/opt/homebrew/bin/gh auth git-credential", "!f() { cat; }; f", "store --file=/x", "",
}

// FuzzCredentialHelperParity holds gitsafe's shapes of a credential
// helper to the rule of internal/spec (REQ-1267) on one corpus: a value
// one admits, the other admits, below the cap only gitsafe has.
func FuzzCredentialHelperParity(f *testing.F) {
	for _, v := range helperCorpus {
		f.Add(v)
	}
	base := spec.HostSettings{
		Schema: "host-settings/v1",
		Root:   "/h/dev",
		Config: spec.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
	}
	base.Tools.Sbx, base.Tools.Git = "/usr/local/bin/sbx", "/usr/bin/git"
	require.NoError(f, base.Validate(), "the settings without a helper are valid")
	f.Fuzz(func(t *testing.T, v string) {
		if v == "" || len(v) > gitsafe.MaxCredentialHelper {
			return // empty is no helper; the cap is gitsafe's own
		}
		h := base
		h.Tools.GitCredentialHelper = v
		s := h.Validate() == nil
		g := gitsafe.CheckCredentialHelper(v) == nil
		require.Equal(t, s, g, "%q: spec admits %v, gitsafe %v", v, s, g)
	})
}
