// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package shquote is the shell quoter of 02 2.1 (REQ-0758): it writes a
// word, or a list of words, as POSIX sh reads it back, unchanged. romeu
// builds the secret command of sbxenv.yaml through it (03 3.3,
// REQ-0201), and the julietaBin argument of a kit (06 6.4).
//
// The form is the one of the POSIX Shell Command Language, 2.2.3: a
// word of the safe characters stays bare, and any other word is put in
// single quotes, inside which sh reads every byte as itself; a single
// quote in it closes the quotes, is written as a backslash and a quote,
// and opens them again. The safe characters are letters, digits and
// _-./:,+@% of ASCII: none of them is an operator, a quote, an
// expansion, a glob, a comment or a tilde to sh, and = is left out, so
// that no word reads as an assignment at the head of a command. A NUL
// byte cannot be carried by any word of sh, and is refused.
//
// Written by hand (ADR 0038): the rule is the few lines above, and the
// maintained quoters found (github.com/kballard/go-shellquote,
// al.essio.dev/pkg/shellescape, mvdan.cc/sh/v3/syntax.Quote) are each a
// new dependency for those lines; go.mod holds none of them.
package shquote

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNUL is the error of a word that holds a NUL byte, which no word of
// sh can carry.
var ErrNUL = errors.New("shquote: a word holds a NUL byte, which sh cannot carry")

// Quote returns s as one word of sh: bare when every byte is safe and
// s is not empty, else in single quotes.
func Quote(s string) (string, error) {
	if strings.IndexByte(s, 0) >= 0 {
		return "", ErrNUL
	}
	if s != "" && strings.Trim(s, safe) == "" {
		return s, nil
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'", nil
}

// Join returns words as one line of sh: each word quoted by Quote, and
// one space between two words.
func Join(words []string) (string, error) {
	out := make([]string, len(words))
	for i, w := range words {
		q, err := Quote(w)
		if err != nil {
			return "", fmt.Errorf("word %d: %w", i+1, err)
		}
		out[i] = q
	}
	return strings.Join(out, " "), nil
}

// safe are the bytes a bare word may hold.
const safe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-./:,+@%"
