// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package initcmd

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// ConfigCloneFailed is the error config-clone-failed (RJ-356): git
	// could not clone the config repo, for a URL that names no
	// repository, an access the origin refuses, or a private repository
	// and no --git-credential-helper. The details hold the start of
	// git's output, escaped (04 4.1, Error ids, item 7).
	ConfigCloneFailed = cli.Row{
		ID:          "RJ-356",
		Slug:        "config-clone-failed",
		Exit:        1,
		Fix:         "decide: check the URL and your access; a private config repo needs --git-credential-helper",
		RecoveredIn: "none: the details hold what git reported, and nothing was written",
	}

	// RootUnsafe is the error root-unsafe (RJ-352): the root fails a
	// check doctor runs on it (04 4.2, the root checks); the message
	// names the rule.
	RootUnsafe = cli.Row{
		ID:          "RJ-352",
		Slug:        "root-unsafe",
		Exit:        2,
		Fix:         "decide: pass another root with --root",
		RecoveredIn: "none: the message names the rule, and nothing was written",
	}

	// SettingsExist is the error settings-exist (RJ-353): host settings
	// exist at the settings path, or appeared there while init ran; init
	// never overwrites them.
	SettingsExist = cli.Row{
		ID:          "RJ-353",
		Slug:        "settings-exist",
		Exit:        2,
		Fix:         "decide: keep the settings that exist, or move them aside and run init again",
		RecoveredIn: "none: the settings that exist are untouched",
	}

	// SettingsUnwritable is the error settings-unwritable (RJ-354): the
	// settings directory or file cannot be made or written; the details
	// hold the system's message.
	SettingsUnwritable = cli.Row{
		ID:          "RJ-354",
		Slug:        "settings-unwritable",
		Exit:        2,
		Fix:         "decide: make the settings directory writable, or point ROMEU_SETTINGS at a path that is, then run init again",
		RecoveredIn: "none: the details hold the system's message, and no settings were written",
	}
)
