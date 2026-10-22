// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package kits embeds the product kits (06 6.4), the one embed.FS of
// their directories. romeu renders from it at sync (02 2.1). The pin
// file kits/pins.yaml and the material of the mise bootstrap
// (kits/mise, kits/mise-minisign.pub) are tools of the repository and
// are not embedded. A product kit joins this file by its directory
// name on the directive below.
package kits

import "embed"

// FS holds one directory per product kit, each with its descriptor and
// the files the descriptor names.
//
//go:embed julieta
var FS embed.FS
