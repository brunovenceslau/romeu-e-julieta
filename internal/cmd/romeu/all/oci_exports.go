// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/oci"

// ParseDescriptor is the kit grammar's read of a descriptor file, for
// the schema check's test that holds the grammar to the vendored kit
// schema (REQ-0573): tools/** reach internal/ through this package
// (REQ-0971).
var ParseDescriptor = oci.ParseDescriptor
