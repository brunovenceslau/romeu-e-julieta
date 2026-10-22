// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package state is the host state of romeu (02 2.4, 03 3.8): the
// records under $XDG_STATE_HOME/romeu, their atomic write, the one
// lock, and the two state machines of 01 1.6 as transition tables
// (REQ-0961).
package state
