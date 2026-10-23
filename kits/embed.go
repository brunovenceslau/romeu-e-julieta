// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package kits embeds the product kits (06 6.4), the one embed.FS of
// their directories. romeu renders from it at sync (02 2.1). The pin
// file kits/pins.yaml and the material of the mise bootstrap
// (kits/mise, kits/mise-minisign.pub) are tools of the repository and
// are not embedded, nor is the testdata/ of a kit. A product kit joins
// this file on the directive below: by its directory name, or by its
// descriptor when the directory holds a testdata/.
package kits

import "embed"

// FS holds one directory per product kit, each with its descriptor and
// the files the descriptor names.
//
//go:embed julieta os-base git-ssh-sign/git-ssh-sign.yaml
var FS embed.FS
