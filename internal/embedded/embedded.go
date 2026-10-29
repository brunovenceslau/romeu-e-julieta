// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package embedded holds the julieta linux binary that romeu delivers
// (02 2.1, REQ-0129). The file julieta beside it is an empty placeholder
// in the tree: a source build embeds zero bytes (REQ-0130), and
// tools/release build maps the julieta of romeu's GOARCH onto it with a
// go build overlay, writing nothing into the tree. Only romeu links this
// package (10 10.7, REQ-0968).
package embedded

import (
	_ "embed"
	"slices"
)

//go:embed julieta
var julieta []byte

// Julieta returns a copy of the embedded julieta binary: empty in a
// source build, whose placeholder holds no binary (REQ-0130).
func Julieta() []byte {
	return slices.Clone(julieta)
}
