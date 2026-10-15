# Ship T007 termsafe and T017 canon

Branches feat/t007-termsafe and feat/t017-canon each hold one signed
commit; whether a ship gate ran on them is unknown, so run it
(code-reviewer sonnet, security-auditor opus, test-engineer sonnet) and
open each PR to main with an unticked line: `termsafe` for T007,
`digests` for T017.

T007 builder: all gates exit 0, 100% coverage, 31 of 31 mutants killed.
Notes for its PR body: the package doc of internal/cli lives in
writer.go until T008 settles it; I29's bidi code points use Unicode's
Bidi_Control set (a spec PR candidate); termsafe.Escape escapes LF but
the Writer keeps it, so callers pass values through Escape first; --json
output is indented with spaces, and encoding/json/v2 (REQ-0960) still
needs a GOEXPERIMENT on Go 1.27.2; the fuzz seeds are f.Add calls, not
testdata files, because of SPDX and REUSE.

T017 builder: all gates exit 0, 96% coverage, 24 of 24 mutants killed.
Notes: invalid UTF-8 is normalised to U+FFFD by encoding/json
(documented); one ULIDSource per process (T008's Env); the spec text
follows the typed API in the spec fixes PR.

Reopen: the next session.
