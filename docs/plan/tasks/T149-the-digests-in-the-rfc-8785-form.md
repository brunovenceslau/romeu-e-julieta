---
id: T149
title: "The digests in the RFC 8785 form"
milestone: M1
module: canon
fate: new
owns:
  - internal/canon/digest.go
  - internal/canon/digest_test.go
  - internal/canon/kindmix_test.go
  - internal/manifest/manifest.go
  - internal/manifest/manifest_test.go
  - internal/manifest/strict_test.go
  - docs/handoff/pending/hand-written-code-a-maintained-library-could-replace.md
needs: []
after: []
ask_first:
  - digests
sitting: none
---

# T149 - The digests in the RFC 8785 form

Goal: item 7 of the inventory of
[ADR 0038, search for a maintained library before writing one](../../adr/0038-search-for-a-maintained-library-before-writing-one.md):
the canonical JSON of REQ-0254 becomes RFC 8785, the JSON
Canonicalization Scheme, computed by `jsontext.Value.Canonicalize` of
the standard library, decided on 2026-10-20. It replaces the
hand-written token copy, key sort and string writer of `internal/canon`
and the v1 encoder of `internal/manifest`, with no new module. Decided
with it: an integer beyond 2^53 keeps its digits
(`CanonicalizeRawInts(false)`), nil lists and maps stay null in a
digest, and the domain `romeu/<kind>/v1` and the format `manifest/v1`
stay, since no released binary has written a digest; REQ-0254 is
amended in place. The edit to `docs/spec/03-formats.md` (3.12) is
T021's file, which stays open (kept), so it is left out of owns.

What changes in the form: keys sort by their UTF-16 code units, not
their UTF-8 bytes; U+2028 and U+2029 are written as they are, not
escaped; a number of a MarshalJSON method takes its ECMAScript form,
so 1, 1.0 and 1e0 share one digest and -0 is 0. Both manifest fixtures
keep their bytes and their run digest.

- Implements: REQ-0254 (03 3.12).
- Operator: no.
- Acceptance:
  - The API of `canon` and of `manifest` does not change, and no caller
    changes.
  - The vectors of `TestCanonicalJSON` hold the RFC 8785 form: the key
    order and the string escapes of RFC 8785 3.2.3 and 3.2.2.2, a
    supplementary character before U+FB33, numbers of a MarshalJSON
    method in one form, and an integer beyond 2^53 with its digits.
  - Every refusal of `TestDigestRefuses` stays a refusal, and text that
    is not UTF-8, a lone surrogate and a U+FFFD are `ErrInvalidText`.
  - `Encode` writes the bytes canon digests (`TestEncodeWritesTheCanonicalJSONOfCanon`,
    `FuzzRunIsCanonical`), and the fixtures round trip unchanged.
  - No file of `internal/canon` or `internal/manifest` imports
    `encoding/json`.
  - The inventory marks item 7 done by T149.
- Verify: `go test ./internal/canon/... ./internal/manifest/... &&
  go test -run='^$' -fuzz=FuzzRunIsCanonical -fuzztime=1m ./internal/manifest/`.
