// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/catalog"

// The egress catalog (03 3.5): the type of the format catalog.v1, for
// the generated schema and page, and the reader and merge of its files,
// for the check "tools/ci catalog". They are here so that tools/ci,
// which reaches internal/ through this package (REQ-0971), registers the
// schema and judges the files; both binaries link internal/catalog in
// any case.
type (
	// CatalogFile is the type of the format catalog.v1.
	CatalogFile = catalog.File
	// CatalogSource is a catalog file by name.
	CatalogSource = catalog.Source
	// Catalog is the merge of the catalog files.
	Catalog = catalog.Catalog
)

// The catalog's constants and the reader of its files.
const (
	// CatalogToolKeyPattern is the pattern of a tool key.
	CatalogToolKeyPattern = catalog.ToolKeyPattern
	// CatalogBaseFile is the file that holds the meta and base sets.
	CatalogBaseFile = catalog.BaseFile
)

// CatalogBuild decodes the files strictly and merges them.
var CatalogBuild = catalog.Build
