// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package salvage

import (
	"sync"
	"time"
)

// LockFunc takes the store lock at path and returns its release, as
// memstore/write.Lock does: it calls onWait once just before it waits,
// when another holds the lock. cmd/julieta passes write.Lock, which
// this package, linked by romeu too, does not import (REQ-0699).
type LockFunc func(path string, onWait func()) (unlock func(), err error)

// LockReport is how long salvage waits on one store lock before it says
// which (REQ-0698). A variable, so a test can lower it.
var LockReport = 10 * time.Second

// WaitLocks waits on each store lock of locks in turn and releases it
// at once (08 8.5 step 1, REQ-0697): a SessionEnd hook or a stamp that
// holds one finishes first, and one written after the wait is allowed
// and not counted (REQ-0699). The wait has no bound, since a stamp must
// finish and a signal is the way out (04 4.1); when it lasts more than
// LockReport on a lock, waiting is called with its path, once.
func WaitLocks(locks []string, lock LockFunc, waiting func(path string)) error {
	for _, p := range locks {
		var mu sync.Mutex
		var timer *time.Timer
		unlock, err := lock(p, func() {
			mu.Lock()
			defer mu.Unlock()
			timer = time.AfterFunc(LockReport, func() { waiting(p) })
		})
		mu.Lock()
		if timer != nil {
			timer.Stop()
		}
		mu.Unlock()
		if err != nil {
			return err
		}
		unlock()
	}
	return nil
}
