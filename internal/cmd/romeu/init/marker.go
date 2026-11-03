// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package initcmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// MarkerName is the marker of the config repo's clone, <cfg>-env/.romeu-config
// (02 2.3): one line, the dir of the clone under <cfg>-env/, written by
// init outside the clone, so git status in the clone does not show it,
// and read by sync step 2, which finds the clone by it instead of
// guessing. The sync package repeats the name, since a command package
// imports no other (10 10.7).
const MarkerName = ".romeu-config"

// errMarkerExists says the marker was there before this run; run
// refuses by the slug config-marker, as it refuses a clone that exists
// by clone-exists.
var errMarkerExists = errors.New("the marker exists")

// writeMarker writes the marker of dir under the env dir envName,
// through rt, the handle of the root: create only, mode 0600, holding
// dir and a newline. created says the file is this run's, from the
// create on, so made.undo removes it when a later step fails (the one
// place init deletes, I16). A marker that exists is errMarkerExists;
// any other failure is init's.
func writeMarker(rt *os.Root, envName, dir string) (created bool, err error) {
	f, err := rt.OpenFile(filepath.Join(envName, MarkerName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return false, errMarkerExists
	}
	if err != nil {
		return false, fmt.Errorf("init: %w", err)
	}
	_, err = f.WriteString(dir + "\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return true, fmt.Errorf("init: %w", err)
	}
	return true, nil
}
