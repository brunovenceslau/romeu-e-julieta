// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package lock

import (
	"fmt"
	"maps"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// unknownTools returns the backend:tool of each entry of lock that cat
// does not have, sorted and once each (07 7.3, REQ-0620; the lookup of
// 07 7.5, step 2). The key is the backend the lock records, the same one
// romeu sync looks up, and a version-only tool is looked up too, since
// the catalog must carry its domains (REQ-0616).
func unknownTools(cat catalog.Catalog, lock mise.Lock) []string {
	seen := map[string]bool{}
	for _, name := range slices.Sorted(maps.Keys(lock.Tools)) {
		for _, e := range lock.Tools[name] {
			if _, ok := cat.Tool(e.Backend); !ok {
				seen[e.Backend] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// warnUnknownTools prints the warning lock-unknown-tool for each entry of
// the mise.lock in raw whose backend:tool the embedded catalog lacks,
// naming the two fixes of 07 7.5, step 2: a pull request that adds the
// tool to the catalog, or an egress.tools entry in the project spec,
// which is gated and is a valid fix outside the repo, so the check warns
// and does not fail. A repo without a lock, and a lock that does not
// parse, have nothing to look up: the second is the finding lock-unparsed.
func warnUnknownTools(w *cli.Writer, dir string, raw mise.Raw) error {
	if !raw.HasLock {
		return nil
	}
	lock, err := mise.ParseLock(raw.Lock)
	if err != nil {
		return nil
	}
	cat, err := catalog.Embedded()
	if err != nil {
		return fmt.Errorf("the embedded egress catalog: %w", err)
	}
	for _, tool := range unknownTools(cat, lock) {
		cli.Warn(w, "lock-unknown-tool", fmt.Sprintf("%s in the mise.lock of %s is not in the embedded egress catalog: "+
			"open a pull request that adds it to the egress catalog (internal/catalog/egress/), or declare it in egress.tools of the project spec, "+
			"which is gated; until then romeu sync fails on it with catalog-unknown-tool",
			termsafe.Escape(tool), termsafe.Escape(dir)))
	}
	return nil
}
