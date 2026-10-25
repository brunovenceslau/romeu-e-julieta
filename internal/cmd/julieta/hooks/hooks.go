// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package hooks holds the commands of julieta hooks (04 4.3): run, the
// dispatcher that git's hooks run through in a sandbox (REQ-0408), on
// the chain of internal/hooks, and guard (REQ-0410), the PreToolUse
// hook of the julieta-claude kit, in guard.go.
package hooks
