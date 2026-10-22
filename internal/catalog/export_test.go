// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package catalog

// UploadFlag returns the upload flag of a domain and whether it is
// declared; the tests read the data through it, and no caller does,
// since a domain without an entry is gated (Gated).
func (c Catalog) UploadFlag(domain string) (upload, declared bool) {
	upload, declared = c.upload[domain]
	return upload, declared
}
