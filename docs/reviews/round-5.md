# Spec round 5: the runtime ledger

Reader: reviewers checking what entered the spec after round 4 and
why, and maintainers tracing a decision. Type: reference (history).
The current state is [docs/spec.md](../spec.md); round 4 is in
[round-4.md](round-4.md).

Round 5 is not a review either. It records one write pass that
transcribed an accepted design, the runtime ledger, into the spec, and
the maintainer decisions that design rests on. It is a new file for
the reason round 4 gave: a round is history and is not rewritten.

The input was a design note with each paragraph labelled as decided by
the maintainer or as a proposal. The maintainer accepted the proposal
as a whole, to be transcribed; it becomes a decision when
[ADR 0006, record runtime events in an add-only ledger ingested on the host](../adr/0006-record-runtime-events-in-an-add-only-ledger-ingested-on-the-host.md)
is accepted. A gate had reviewed the design and left eight corrections
for this pass.

## Maintainer decisions

The first was stated on 2026-09-30, the others on 2026-09-30 and
2026-10-01, some in the maintainer's own words and some by accepting a
block of recommendations.

| Id | Decision | Where |
|---|---|---|
| op-ledger | julieta saves runtime information that feeds back into the sessions; one store per machine, shared by its projects; nothing edits it | 13; ADR 0006 |
| op-ledger-absolute | nothing in the ledger is ever edited or deleted: no exception, no tombstone, no redaction event | 13, I31; index, Boundaries |
| op-ledger-v1-detects | v1 detects a content mismatch of ingested event bytes and nothing else; a removal is undetected, so "never deleted" is a promise; a hash chain is deferred with three triggers | 13 13.1, 05 5.4; index, Deferred decisions |
| op-ledger-from-ingest | the guarantee is add-only from ingest onward; the time between a write in the sandbox and ingest is a named residual risk | 13 13.1, 05 5.4 |
| op-ledger-no-free-text | a v1 event has no free text; fields are closed and detail is limited to validated identifiers | 13 13.2, I33 |
| op-ledger-secret | a secret that lands in the ledger stays there; the remedy is rotating the credential; a pattern hit rejects the event and replaces nothing | 13 13.2, 05 5.4 |
| op-ledger-read | a sandbox may read other projects' events; the full view is on the host only | 13 13.6, I33 |
| op-ledger-versioned | the schema is versioned, and ingest accepts versions N and N-1 | 13 13.7 |
| op-ledger-retention | events are kept; `doctor` warns past a size; rotation is deferred | 13 13.8; index, Deferred decisions |
| op-ledger-placement | the three paths: the ledger under host state, the spool and the view under `<name>-env/ledger/` | 13 13.1, 02 2.3, 02 2.4 |
| op-ledger-note | `julieta event add` stays in v1 as the `note` type, without free text | 13 13.2, 04 4.3 |
| op-ledger-hooks | the hook dispatcher also emits an event, and julieta state stays its in-session cache; GitHub data owns the published delivery metrics and the ledger owns none | 13 13.2, 13 13.9, 12 12.10 |
| op-ledger-surface | `ledger` is an ask-first surface; a new field in the cross-project view needs the maintainer's quoted approval | 05 5.3 |
| op-ledger-simpler | no quarantine directory; no message catalog of the ledger's own; each deferred decision carries its trigger | 13 13.4, 04 4.4; index, Deferred decisions |

## What the pass added

| Added | Where |
|---|---|
| the runtime ledger: stores, events, the spool's reading regime, ingest, entries, the view and its policy, versions, starting values, consumers, tests | 13 |
| three invariants, I31 to I33, and a narrower scan list for `internal/ledger` under I16 | 05 5.2 |
| the `ledger` ask-first surface; seven residual risks | 05 5.3, 05 5.4 |
| two fixed mounts, the ledger directory in host state, the manifest's `ledger` paths | 02, 03 3.3, 03 3.6 |
| `romeu ledger ingest`, `romeu ledger query`, `julieta event add`, `julieta event list`; the ingest step of `sync`, `run` and `salvage`; three `doctor` rows; nine error ids, by slug | 04 |
| where ingest runs in J2, J3, J6, J7 and J10 | 09 |
| the `ledger` module, one generator row | 12 12.2, 12 12.3 |
| probe A3 extended with the view mount and a hostile spool | 11 11.1 |
| four vocabulary rows; "acceptance ledger" reworded to "acceptance file" in two places, so that "ledger" names one thing | 01 1.7, 02 2.2, 10 10.5 |
| ADR 0006, Proposed | `docs/adr/`; Q24 covers it |

Concepts the ledger adds, counted as the index asks: seven. The spool,
the ledger, the view, the event schema with its closed tables, ingest,
the view policy and the drain. What it reuses instead of adding: the
memory store's file reader and caps, fixed mounts and the writability
check of `.romeu/bin`, the one error table, plain sha256 for raw
bytes, struct tags with `go generate`, the version rows of
`versions.go` and the N and N-1 rule of the manifest protocol,
temporary file and rename, `romeu.lock`, the clock and entropy seams,
the I16 scan, the salvage record's `reasons`, `doctor`, probe A3, Q24
and the Deferred decisions table.

## The gate's corrections to the design

Each was applied as a correction, not weighed as an option.

| Correction | Applied as |
|---|---|
| the key uses plain sha256 of the raw spool bytes, with no new `canon.Digest` kind | 13 13.4 step 1; 03 3.12 lists spool files among raw bytes |
| a name that exists is compared by event region (rejected entries: key and reason id), not by whole file; rows for equal and different bytes | 13 13.5; 13 13.10, entries table |
| an `exit_code` range from zero rejects a process killed by a signal | the range admits -1; a `signal` field with its own table would be larger (13 13.2) |
| files newer than the version window are outside the count cap and still read on each ingest | a bound on the names one ingest examines, reported as `listing-truncated` (13 13.3, 13 13.8) |
| the membership tables come from the spec and the host catalog, never from a project file; a tool only in a project's manifest | stated in 13 13.2 and 12 12.3; the closed member `tool: other`, with the reason |
| "never opens an entry for writing" was checked only where a name exists | I31 reworded: one create function, and a scan with four admitted call sites; 13 13.5 says what neither proves |
| test specifiability | number forms and the `path` byte set (13 13.2); the racing ingests, no leftover temporary name, view derivation serialized by `romeu.lock` (13 13.4); the same bytes after a drain; a missing or symlinked spool root, the exact count cap, a device node, the host clock moving backwards; the `afterOpen` seam; an outcome on each hostile row (13 13.10) |
| the gate-run event stays open | no v1 event type; the existing row of the Deferred decisions table now says why and has a new trigger |

## The writer's own decisions

The design leaves these open. The maintainer has not ruled on them and
the sweep should look at each:

- I33 as a third invariant; the design listed two.
- Which fields each event type carries, and that `hook-failed` names
  neither the hook nor the repo in v1.
- `command-failed` covers each julieta command that exits non-zero
  except `hooks run`, `pane run` and `event`; for `julieta install`,
  `tool` is the catalog key of each tool that failed.
- Emitting is best effort and never changes a command's exit status.
- The drain runs in `julieta setup`, not on each julieta start, to
  keep its cost bounded.
- The check order: syntax, then version, then keys, so an event of a
  newer version with a new field is skipped and kept. The design had
  the unknown-key check first.
- The entry file: the name `<project>.<key>.entry` in one flat
  directory, and a header line followed by the event region.
- The view files `keys`, `own.jsonl` and `others.jsonl`; no key in
  `others.jsonl`; a view file is rewritten only when its bytes differ.
- The four commands and their flags.
- A failed ingest inside `sync` and `run` is reported and changes no
  exit status; `ledger-incomplete` in a salvage record changes no
  `result`.
- The nine error slugs, in exit classes 1 and 2, with numbers left to
  the table.
- The number of names one ingest examines, a starting value like the
  others (13 13.8).
- The formats live in 13 with a pointer from 03 3.13, as 11 and 10 do
  for theirs.

One thing could not be transcribed without inventing: the secret
patterns. The design names a pattern check and no pattern. The spec
fixes where the list lives, who reviews it and how it is tested, and
leaves its entries to the plan task that builds the ingester.

## Deferred

In the index's [Deferred decisions](../spec.md#deferred-decisions)
table, each with its trigger: rotation and a hard size limit; a hash
chain; cross-machine sync; finer visibility per project; analysis
beyond `query`; a free-text note; keeping rejected bytes; a cleaner
for skipped spool files; more event types and fields; tuning the
starting values and the pattern list. The row for a record of gate
runs existed and was updated.

Question changed: Q24 now covers the six Proposed records.
