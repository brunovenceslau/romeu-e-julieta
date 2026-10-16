// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

// The one go:generate directive of the repository (12 12.3): "go
// generate ./..." runs the generate subcommand of this command, which
// the generated check registers (check_generated.go) and writes the
// generated files. The generated check refuses a go:generate line
// anywhere else (generated.Site).
//go:generate go run . generate
