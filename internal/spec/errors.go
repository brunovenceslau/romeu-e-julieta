// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises. A newer
// version of a format is format-newer (RJ-311), declared by
// internal/manifest.
var (
	// SecretInArgv is the error secret-in-argv (RJ-314): an element of a
	// secret's argv in host settings has the shape of a token (03 3.4,
	// REQ-0206, I25). The message names the entry and the element's
	// index, never its value.
	SecretInArgv = cli.Row{
		ID:          "RJ-314",
		Slug:        "secret-in-argv",
		Exit:        2,
		Fix:         "decide: replace the value in settings.yaml with a command that fetches it, such as one that reads a Keychain item",
		RecoveredIn: "none: the edit of the hint fixes the settings, and no value was used",
	}

	// FormatUnsupported is the error format-unsupported (RJ-337): a
	// project spec or host settings file of a version older than the
	// previous one (03, opening, REQ-0167).
	FormatUnsupported = cli.Row{
		ID:          "RJ-337",
		Slug:        "format-unsupported",
		Exit:        2,
		Fix:         "decide: apply the edit in the release notes of each skipped release",
		RecoveredIn: "none: the edit of the hint brings the file to a version this build reads",
	}

	// ProjectsStrayFile is the error projects-stray-file (RJ-345): a file
	// under projects/ that is not *.yaml (03 3.1, REQ-0178).
	ProjectsStrayFile = cli.Row{
		ID:          "RJ-345",
		Slug:        "projects-stray-file",
		Exit:        2,
		Fix:         "decide: move or remove <path>",
		RecoveredIn: "none: the config repo holds the file, and the edit of the hint removes it",
	}

	// ConfigProjectMissing is the error config-project-missing (RJ-346):
	// no spec's primary repo URL equals config.url of host settings (03
	// 3.1, REQ-0179).
	ConfigProjectMissing = cli.Row{
		ID:          "RJ-346",
		Slug:        "config-project-missing",
		Exit:        2,
		Fix:         "decide: add the config repository's own spec under projects/, or fix config.url in settings.yaml",
		RecoveredIn: "none: the edit of the hint adds the spec or fixes the settings",
	}
)
