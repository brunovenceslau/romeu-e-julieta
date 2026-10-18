// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package all is the assembled command set of romeu: one blank import
// per command, each in its own import_<command>.go, links the command's
// package in so its init registers its definition (04 4.1; 12 12.3).
// The binary's main.go imports this package once.
package all
