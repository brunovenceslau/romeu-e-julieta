// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"errors"
	"fmt"
	"strings"
)

// ExceptionsFile is the file of a repo that excuses a tool from the
// checksum rule of 07 7.3 (decided on 2026-10-22): one line for each tool,
// "<key of mise.toml> <reason>", blank lines and lines that start with
// "#" left out. It is a tracked file of the repo, so an exception is
// reviewed in the diff that adds it; it is per tool, so one tool whose
// vendor publishes no checksum excuses no other; and the check reads it
// with the repo's own files, in the pre-commit action and in CI, where the
// project spec, which lives in the config repo, is not at hand. It
// excuses the missing checksum only: a malformed one, a missing platform
// and a missing entry stay findings.
const ExceptionsFile = ".julieta/lock-exceptions"

// ErrUnparsed is the error of a file of the repo that does not parse:
// mise.toml, mise.lock or the exceptions file. install returns it as
// lock-unparsed (RJ-325), and the check as a finding of the same slug.
var ErrUnparsed = errors.New("a mise file does not parse")

// Exceptions are the tools the exceptions file excuses, each with its
// reason.
type Exceptions map[string]string

// ParseExceptions reads the exceptions file: at most MaxFileSize bytes,
// no control character, one reason for each tool and a tool once.
func ParseExceptions(data []byte) (Exceptions, error) {
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("%s: %d bytes pass the cap of %d", ExceptionsFile, len(data), MaxFileSize)
	}
	ex := Exceptions{}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.ContainsFunc(line, func(r rune) bool { return r < 0x20 && r != '\t' || r == 0x7f }) {
			return nil, fmt.Errorf("%s:%d: a control character", ExceptionsFile, i+1)
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, reason, _ := strings.Cut(line, " ")
		reason = strings.TrimSpace(reason)
		if reason == "" {
			return nil, fmt.Errorf("%s:%d: the tool %q has no reason", ExceptionsFile, i+1, key)
		}
		if _, dup := ex[key]; dup {
			return nil, fmt.Errorf("%s:%d: the tool %q is excused twice", ExceptionsFile, i+1, key)
		}
		ex[key] = reason
	}
	return ex, nil
}
