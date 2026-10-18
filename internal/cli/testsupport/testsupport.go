// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package testsupport holds the one helper a test asserts the id of an
// error through (10 10.1, Error ids): tests assert ids, never message
// text, and the meta-test of 10 10.1 collects the id each call names.
package testsupport

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// RequireID fails the test unless err carries the error of the table
// whose id is want: the outermost *cli.Error in its chain, or the error
// internal (RJ-348) for a chain with none.
func RequireID(tb testing.TB, want string, err error) {
	tb.Helper()
	require.Error(tb, err)
	require.Equal(tb, want, cli.Of(err).ID, "the id of %v", err)
}
