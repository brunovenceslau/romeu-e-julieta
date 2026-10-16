// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// TestSendPack checks the pipe between pack-objects and unpack-objects:
// the objects of want that have does not reach arrive in the clone, and
// an unpack-objects that ends first makes pack-objects end too, so the
// pair returns at once with the error of each and nothing hangs.
func TestSendPack(t *testing.T) {
	source := gittest.New(t)
	source.Write("a.txt", "a\n")
	have := source.Commit("a")
	// A blob larger than a pipe holds, which git cannot compress, so that
	// pack-objects still writes when unpack-objects has ended.
	noise := make([]byte, 4<<20)
	_, err := rand.Read(noise)
	require.NoError(t, err)
	source.Write("b.bin", string(noise))
	want := source.Commit("b")
	sourceRepo := git.Repo{Dir: source.Dir, Env: source.Env}

	t.Run("the objects arrive", func(t *testing.T) {
		clone := gittest.New(t)
		clone.Write("a.txt", "a\n")
		require.Equal(t, have, clone.Commit("a"), "the clone holds have, as the fixtures are reproducible")
		cloneRepo := git.Repo{Dir: clone.Dir, Env: clone.Env}
		require.NoError(t, sendPack(t.Context(), sourceRepo, cloneRepo, want, have))
		_, err := cloneRepo.Run(t.Context(), nil, "cat-file", "-e", want+"^{commit}")
		assert.NoError(t, err, "the clone has the commit want")
	})
	t.Run("an unpack-objects that fails ends the pair", func(t *testing.T) {
		notARepo := git.Repo{Dir: t.TempDir(), Env: append(source.Env, "GIT_CEILING_DIRECTORIES=/")}
		// A pair that hangs ends at this limit instead of the class's.
		ctx := procgroup.WithTestTimeout(t.Context(), procgroup.ToolGit, 10*time.Second)
		done := make(chan error, 1)
		go func() { done <- sendPack(ctx, sourceRepo, notARepo, want, have) }()
		select {
		case err := <-done:
			require.Error(t, err)
			assert.Contains(t, err.Error(), "snapshot: git unpack-objects: ")
			require.NotErrorIs(t, err, context.DeadlineExceeded, "pack-objects did not wait on a pipe no one reads")
		case <-time.After(30 * time.Second):
			require.FailNow(t, "the pair did not end after unpack-objects failed")
		}
	})
}
