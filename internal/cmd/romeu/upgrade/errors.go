// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package upgrade

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// DownloadFailed is the error upgrade-download-failed (RJ-362,
	// REQ-1332): the releases API or an asset read failed, timed out or
	// passed its cap, or the release has no darwin archive for
	// runtime.GOARCH or no checksums.txt; nothing is written, and the
	// details hold the URL and the status.
	DownloadFailed = cli.Row{
		ID:          "RJ-362",
		Slug:        "upgrade-download-failed",
		Exit:        1,
		Fix:         "decide: check the network and the release on GitHub, then run romeu upgrade again",
		RecoveredIn: "none: nothing was written, and the rerun of the hint reads the release again",
	}

	// VerifyFailed is the error upgrade-verify-failed (RJ-363,
	// REQ-1333): the sha256 of the archive differs from checksums.txt,
	// or a command of the attestation entry of the verification
	// contract exited non-zero; the running binary is unchanged, and the
	// details hold gh's output, escaped and cut to its first 4 KiB. The
	// new binary's preview that does not run is this error too (T196,
	// provisional): the archive did not prove it runs here.
	VerifyFailed = cli.Row{
		ID:          "RJ-363",
		Slug:        "upgrade-verify-failed",
		Exit:        1,
		Fix:         "decide: do not install this archive; report the release as an issue",
		RecoveredIn: "none: the running binary and toolchain.json are unchanged",
	}

	// GhUnusable is the error upgrade-gh-unusable (RJ-364, REQ-1334):
	// tools.gh is unset, not an absolute path to an executable, or below
	// the floor of 12 12.1, found before any download.
	GhUnusable = cli.Row{
		ID:          "RJ-364",
		Slug:        "upgrade-gh-unusable",
		Exit:        2,
		Fix:         "decide: set tools.gh in the host settings to a gh at or above the floor, or install by hand as J1 step 2 says",
		RecoveredIn: "none: nothing was read or written, and the rerun of romeu upgrade checks tools.gh again",
	}

	// BinaryUnwritable is the error upgrade-binary-unwritable (RJ-365,
	// REQ-1335): the directory of the running binary, its links
	// resolved, is not writable by the user, found before any download.
	BinaryUnwritable = cli.Row{
		ID:          "RJ-365",
		Slug:        "upgrade-binary-unwritable",
		Exit:        2,
		Fix:         "decide: move romeu to a directory you own, or install by hand as J1 step 2 says",
		RecoveredIn: "none: nothing was read or written, and the rerun of romeu upgrade checks the directory again",
	}

	// Declined is the error upgrade-declined (RJ-366, REQ-1349): the
	// answer to the confirmation was anything but y; the temporary file
	// is deleted, and the running binary and toolchain.json are
	// unchanged.
	Declined = cli.Row{
		ID:          "RJ-366",
		Slug:        "upgrade-declined",
		Exit:        1,
		Fix:         "romeu upgrade",
		RecoveredIn: "none: the running binary and toolchain.json are unchanged, and the rerun of the hint asks again",
	}
)
