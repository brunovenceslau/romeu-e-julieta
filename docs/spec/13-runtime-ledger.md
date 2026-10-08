# 13. Runtime ledger

Back to [index](../spec.md). Reader: implementers of `internal/ledger`
and of the commands that call it; security reviewers. Type: reference.

Problems show up when things execute. julieta records what failed
inside a sandbox, romeu keeps those records on the host, and later
sessions and the maintainer read them to improve the product and the
environment. The decision, and what it rejects, is in
[ADR 0006, record runtime events in an add-only ledger ingested on the host](../adr/0006-record-runtime-events-in-an-add-only-ledger-ingested-on-the-host.md).

The rule the maintainer set for it is absolute: nothing in the ledger
is edited or deleted. v1 has no tombstone and no redaction event. This
page says, for each part of that rule, whether a mechanism enforces it
or whether it is a promise.

This store is not the memory store of
[08 8.1](08-memory-handoff-salvage.md#81-memory-store): memory is per
repo, an agent edits it, and it persists; the ledger is per machine,
nothing edits it, and a sandbox has no path to it.

## 13.1 Stores

```text
sandbox (untrusted)                 host (trusted)
-------------------                 --------------
julieta -> spool  ---ingest--->     ledger  ---derive--->  view
           read-write mount         never mounted          read-only mount,
           of one project                                  one per project
```

| Store | Path | Who writes | Who reads | Guarantee |
|---|---|---|---|---|
| spool, one per project | `<name>-env/ledger/spool/` | anything in that project's sandbox | romeu, at ingest and in `doctor` | none: input an agent controls |
| ledger, one per machine | `$XDG_STATE_HOME/romeu/ledger/` | romeu | romeu (`ledger query`, `doctor`, view derivation) | add-only from ingest onward (I31) |
| view, one per project | `<name>-env/ledger/view/` | romeu | that project's sandbox | derived from the ledger and the view policy (I33) |

*Why for us (three stores):* each follows from a rule the spec already
has. Agent output reaches the host only as data the host validates
([01 1.2](01-system-model.md#12-sources-of-truth-vs-derived)); host-only
state is never mounted (I20); no directory is mounted into two
sandboxes. One shared directory would break the second and the third,
and a host store written from a sandbox would break the first.

What the guarantees mean:

- **The spool is a claim, not a record.** It is a read-write mount, so
  an agent can write, alter, forge or delete a spool file before
  ingest, with or without julieta. Nothing inside a sandbox can make
  julieta the one writer of a directory the agent can also write. A
  deleted spool file leaves no trace.
- **Add-only starts at ingest** and is enforced on the host. A ledger
  entry is data an agent may have written and that passed validation.
- **v1 detects one thing about a stored entry**: that the event bytes
  of an ingested entry no longer match the digest in its name. A
  removed entry is not detected. So "never deleted" is a promise in
  v1, not a verified property, and nothing here protects the ledger
  from the host, which the threat model trusts
  ([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)).
  A hash chain is a
  [deferred decision](../spec.md#deferred-decisions).

The two mounts are fixed: no spec field names them, they are the same
for each project, and they are rendered into `sbxenv.yaml`
([03 3.3](03-formats.md#33-rendered-sbxenvyaml-sbx-env-file-schemaversion-1))
like `.romeu/bin`. They are separate mounts whose roots are `spool/`
and `view/`, so `<name>-env/ledger/` itself is outside every mount and
romeu's temporary files for the view live inside `view/`. Because they
are in the rendered file they are inside `renderDigest`
([01 1.4](01-system-model.md#gate-2-project-widening-per-project)): a
project rendered without them meets gate 2 once and needs one
recreate, since sbx applies mounts at create. v1.0.0 renders them from
the first `sync`, so that cost falls only on a project created by a
build older than this section.

## 13.2 Events

One event is one JSON file in the spool, schema `runtime-event/v1`. An
event has closed fields and no free text, for the writing project's
own events as much as for other projects'. That is what lets the
absolute rule hold: a field that cannot carry a sentence cannot carry
a pasted token, so there is nothing to redact later. The widest value
an agent chooses is `id`, 26 characters of a fixed alphabet, and it
stays out of the cross-project view.

```json
{"schema":"runtime-event/v1","id":"01j9zc2q8x7w4t6y3r5e1v9k0p","observed":"2026-10-02T09:20:00Z","type":"command-failed","tool":"core:go","exit_code":1,"duration_ms":4210}
```

Fields julieta writes. Each is a claim of the sandbox:

| Field | Domain | In the cross-project view |
|---|---|---|
| `schema` | `runtime-event/v<n>` | yes |
| `id` | a ULID in the lowercase Crockford alphabet (26 characters), from julieta's injected entropy ([10 10.4](10-testing-style.md#104-determinism-seams-and-golden-governance)); equals the spool file's name without `.json` | no: it is 26 characters an agent chose, and no consumer in another project needs it |
| `observed` | RFC 3339, UTC, the `Z` suffix, whole seconds, from julieta's injected clock | yes |
| `type` | a member of the event-type table: `command-failed`, `hook-failed`, `note` | yes |
| `tool` | a member of the tool table: a julieta command name, a key of the embedded catalog's `tools`, or `other` | yes |
| `ref` | a member of the id table: an invariant id, a probe id or an error id | yes |
| `exit_code` | an integer in the range of [13.8](#138-starting-values) | yes |
| `duration_ms` | an integer in the range of 13.8 | yes |

Fields romeu stamps at ingest. They are never read from the spool:

| Field | Source |
|---|---|
| `project` | the project whose spool romeu opened the file from |
| `ingested` | romeu's injected clock; RFC 3339, UTC, whole seconds |

Which fields a type carries:

| `type` | `tool` | `ref` | `exit_code`, `duration_ms` | Producer |
|---|---|---|---|---|
| `command-failed` | required | optional: the error id the command failed with | required | julieta, when one of its commands exits non-zero |
| `hook-failed` | required: `hooks run` | absent | required | julieta's hook dispatcher, when a hook exits non-zero |
| `note` | optional | optional | absent | `julieta event add`; at least one of the two is present |

An optional field that does not apply is absent. `null` is outside
every domain.

Rules:

- **Closed sets are membership, never a pattern.** `type`, `tool` and
  `ref` are valid when the value is a row of the generated table for
  that field. The tables are generated from the code and data the
  release is built from: the event types, the command definitions, the
  embedded catalog, the invariant tags, the probe definitions and the
  `internal/cli` error table
  ([12 12.3](12-engineering.md#123-generators)). No table reads a
  project's spec, manifest or repository. A string an agent chose
  therefore has no way into `type`, `tool` or `ref`. A julieta command
  name is the full command path as `--help` prints it, words separated
  by one space (`setup`, `hooks run`, `memory add`). In v1 a table only
  gains members (13.7).
- **`tool: other`** is the one member that stands for "a tool the
  catalog does not know". julieta writes it when a tool that fails is
  only in a project's own `egress.tools` or `mise.toml`. We added the
  member instead of rejecting such an event whole: an install failure
  of an uncatalogued tool is one of the signals the ledger exists for
  (it points at a catalog gap), rejecting it would lose its exit code
  and duration and mint one rejected entry per failure, and `other`
  carries no string from the project.
- **`exit_code` admits -1.** Go reports -1 for a process that was
  killed by a signal or did not exit, so a range from zero up would
  reject each out-of-memory kill and each timeout, and those would
  never reach the ledger. Widening the range by one value is smaller
  than a `signal` field with its own table; the cost is that the event
  does not say which signal.
- **Numbers have one form.** `exit_code` and `duration_ms` are JSON
  numbers written as `-?(0|[1-9][0-9]*)`: no fraction, no exponent, no
  `-0`, no string. `1.0`, `1e2` and `"1"` are outside the domain.
- **One fact has one field, and an event carries no message.** An
  error id goes in `ref` and an exit status in `exit_code`. The
  message and the fix hint of an error id are read from the error
  table ([04 4.1](04-cli.md#41-conventions-both-binaries)).
- **A value outside its domain rejects the whole event** (13.4). No
  field is dropped from a view silently.
- **romeu orders by its own stamps.** `id` and `observed` come from
  the sandbox; views and queries are ordered by `ingested`, then
  `project`, then entry key, and a test pins that order (13.10). v1
  has no host decision that reads `id` or `observed`. A consumer that
  computes a duration from `observed` computes with a claim.
- **An event names no file.** v1 has no field for a path: a field
  wide enough for one is wide enough for a token, and no consumer of
  13.9 reads one. It is a
  [deferred decision](../spec.md#deferred-decisions) with its trigger.
- **A secret that still lands in the ledger stays there.** The remedy
  is to rotate the credential (05 5.4).

Producers in v1. These are the code paths that emit; an agent can
also write a spool file by hand, which is why the spool is a claim
(13.1):

- julieta emits `command-failed` from the code path that maps a
  command's error to its exit status, so capturing a failure does not
  depend on an agent remembering a command. `tool` is the command's
  name; for `julieta install` it is the key of each tool mise failed
  to install, or `other`. Three commands emit none: `hooks run` (the
  dispatcher emits `hook-failed`, so one failure is one event),
  `pane run` (its status is a pane step's, not julieta's) and `event`
  (it would report on itself).
- The hook dispatcher emits `hook-failed` and still records the
  failure in julieta state, which stays the in-session cache that
  SessionStart reads
  ([08 8.2](08-memory-handoff-salvage.md#82-session-hooks-user-side-middleware)).
  The v1 event does not name the hook or the repo; julieta state does.
- `julieta event add` emits `note`.
- romeu writes rejected entries (13.5) and no event.

julieta's emit and drain live in `internal/ledger/emit`, a package
only julieta links
([10 10.7](10-testing-style.md#107-module-boundaries-enforced-by-toolsci-imports)),
so the code that deletes spool files is outside what romeu links and
outside the scan of 13.5. The validator is in the parent package,
shared by emit and ingest. julieta emits only when the manifest names
a spool
([03 3.6](03-formats.md#36-julieta-manifest-schema-manifestv1)), and
emitting is best effort: it validates the event with the same
generated validator romeu uses, writes it to a temporary name in the
spool and renames it to `<id>.json`. A failure to emit changes
neither the output nor the exit status of the command that failed.

## 13.3 Spool

The spool is the second agent-writable directory the host reads, so it
gets the reading regime the memory directories have (I12, I24), as its
own invariant, I32. The two regimes differ on purpose: a memory
violation refuses `sync` and `run`, and a spool violation is skipped
and reported; memory persists, and the spool is drained. The file
reader and the caps helper are the ones `memstore` has, so the regime
has one implementation.

- **Layout**: `<id>.json` files directly under `spool/`, where `<id>`
  is a lowercase ULID. A name in another case is outside the layout.
  The reason is the host filesystem: APFS is case-insensitive by
  default, so two names that differ in case alone would collide there
  and one event would be dropped as a duplicate, which a Linux CI run
  cannot show. For the same reason an entry key is lowercase hex and a
  project name comes from a lowercase grammar
  ([03 3.1](03-formats.md#31-name-and-path-rules-shared)).
- **The root**: romeu opens `spool/` with `os.Root` after checking,
  with Lstat, that it exists and is a directory. A root that is
  missing, a symlink or another file type fails that project's ingest
  (13.4) and nothing in it is read.
- **Listing**: romeu reads at most one name more than the bound in
  13.8, sorts the names it read, and examines the first of them, up to
  the bound; a name outside the layout counts toward it. When that one
  extra name is read, the spool holds more names than the bound and is
  reported as `listing-truncated`. The directory hands its names over
  in the filesystem's order, so the names examined are the first in
  name order of those read, not of the whole spool. This bounds the work of one ingest whatever
  the names are: a file that is skipped stays in the spool and would
  otherwise be read again by each ingest without limit.
- **Reading one file**: opened once through the root, without
  following links and without blocking, then checked on the open
  descriptor: a regular file, within the size cap. romeu reads it once
  into memory, at most the size cap plus one byte, so a file that grew
  after the check is seen as over the cap without being read whole.
  romeu validates, hashes and stores those same bytes. A file swapped
  or rewritten after the open therefore cannot make romeu store bytes
  it did not validate, and a FIFO cannot hang `salvage` or `rm`.
- **Path-level violations** are skipped, never ingested, and have no
  key because nothing was read:

  | Reason | Meaning |
  |---|---|
  | `name` | a name outside the layout: another extension, a dotfile, uppercase, a julieta temporary name |
  | `type` | not a regular file: a symlink, a FIFO, a directory, a device |
  | `size` | over the size cap |
  | `version-newer` | a syntactically valid event of a version newer than the window (13.7) |

  Each ingest and `doctor` report a count per reason and the first
  paths of each reason, in name order, up to the reporting limit of
  13.8. v1 has no cleaner for them (a deferred decision).
- **Draining.** romeu deletes nothing it does not own (I16), and
  julieta owns the spool. The view lists the keys that have an entry
  (13.6). `julieta setup` hashes each spool file and deletes it when
  its sha256 is a listed key, and deletes its own leftover temporary
  names. A file whose digest is not listed stays for the next ingest.
  The same bytes written again after a drain match an existing entry,
  so ingest writes and reports nothing and the next `setup` drains
  them again. A file swapped between julieta's hash and its delete is
  lost, which is again the power the agent already has.

## 13.4 Ingest

Ingest reads one project's spool and creates ledger entries. It runs
inside four commands, at the step each one names:
`romeu sync` (for each target project), `romeu run`, `romeu salvage`
(before the sandbox half, so also in `rm`, `recreate` and `retire`),
and `romeu ledger ingest`
([04 4.2](04-cli.md#42-romeu-host)). `retire` ingests a second time,
after the sandbox half and before it moves `<name>-env/` away: the
spool moves with that directory, so the events julieta wrote during
its last salvage would otherwise have no next ingest. The spool is a
host directory, so ingest needs no sandbox.

**A failed ingest never blocks.** An ingest that could not finish (a
bad spool root, an entry that could not be written, an entry whose
name exists with other content) is reported with the error id
`ledger-incomplete`, the one id an ingest ends with; its details carry
the reason of each file or entry, `ledger-entry-mismatch` for the
last case (04 4.4). The command it runs inside continues with its
exit status unchanged. `salvage` records `ledger-incomplete` in the
salvage record's `reasons` without changing `result`
([08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction)).
Only `romeu ledger ingest`, whose one job it is, exits 1.

For each file the spool reader hands over:

1. **Key.** The key is the plain sha256 of the raw spool bytes, in
   lowercase hex. It is not a `canon.Digest` kind: that function is
   for structured values, and raw file bytes use plain sha256 so that
   `shasum` reproduces them
   ([03 3.12](03-formats.md#312-digests-internalcanon)).
2. **Idempotency.** If an entry named by this `project` and key
   exists, ingest writes nothing and reports nothing. The same file
   ingested on `run` and again on `salvage` gives one entry. Two
   identical failures in one session differ in `id` and `observed`, so
   they are two entries. The same bytes in two projects are two
   entries, so one project cannot probe another's events through a
   key.
3. **Checks, in this order; the first failure is the reason.** The
   reasons are slugs of error ids in the one error table (04 4.1); the
   ledger has no catalog of its own.

   | Step | Check | Reason |
   |---|---|---|
   | 1 | syntax: UTF-8 without a byte-order mark, one JSON object, no duplicate key | `event-syntax` |
   | 2 | `schema` is present, has the form `runtime-event/v<n>`, and `n` is inside the window (13.7). A newer `n` is not a rejection: it is the path-level violation `version-newer` | `event-version` |
   | 3 | the keys are those of that version and of the event's `type`: none unknown, none missing | `event-keys` |
   | 4 | `id` equals the file name | `event-id` |
   | 5 | each field is inside its domain (13.2) | `event-domain` |

   The version is read before the keys, so an event of a newer version
   that carries a field this romeu does not know is skipped and kept,
   not rejected.
4. **Entry.** A file that passes becomes an ingested entry; a file
   that fails becomes a rejected entry (13.5). Either way the key has
   an entry, the view lists it, and julieta drains the file. v1 has no
   quarantine directory: the rejected entry reports a bad file once.

After the files, romeu derives the view (13.6) for the project it
ingested.

Mutating romeu commands hold `romeu.lock` (04 4.1), so two ingests do
not overlap in normal use and view derivation is serialized by that
lock. I31 does not depend on the lock: it holds for writers that race.
One thing does depend on it: a temporary name a crash left in the
ledger directory is unlinked by the command, under the lock, before
its first ingest. The ingest function itself removes only the
temporary name it created, so one racing ingest cannot unlink a file
another is about to link.

## 13.5 Entries

The ledger is one directory, mode 0700, with one file per entry, mode
0600:

```text
$XDG_STATE_HOME/romeu/ledger/<project>.<key>.entry
```

`<project>` is a project name (03 3.1, no `.`) and `<key>` is 64
lowercase hex characters. Entries of a retired project stay. A project
created later under a retired project's name reads those entries as
its own, because in v1 the name is the identity.

An entry is one header line and, for an ingested entry, the event
bytes:

```text
{"schema":"ledger-entry/v1","kind":"ingested","project":"shop","key":"<64 hex>","ingested":"2026-10-02T09:21:07Z","size":212}
<exactly 212 bytes: the spool file, verbatim>
```

```text
{"schema":"ledger-entry/v1","kind":"rejected","project":"shop","key":"<64 hex>","ingested":"2026-10-02T09:21:07Z","reason":"RJ-2nn"}
```

The header is canonical JSON (03 3.12) ending in one newline. The
**event region** of an ingested entry is the `size` bytes after that
newline, and nothing follows it. `doctor` re-hashes that region and
compares it with the key in the name. A rejected entry holds the key
and the reason's error id and not the rejected bytes: they are
unvalidated and may hold anything.

**Creating an entry (I31).** One function creates entries, and it is
the guard:

1. Create a temporary file inside the ledger directory, with a name
   the entry grammar cannot produce (`.tmp-<ULID>`); a hard link
   across filesystems would fail, so it cannot live elsewhere.
2. Write it, sync it to disk and close it.
3. Hard-link it to the entry's name. A link fails when the name
   exists, so an entry is never replaced.
4. Unlink the temporary name. A leftover from a crash is unlinked by
   the next command that ingests, under `romeu.lock` and before its
   first ingest (13.4), never by the ingest function.

When the link fails because the name exists (in Go,
`errors.Is(err, fs.ErrExist)`), romeu compares the new entry with the
existing one: for an ingested entry, `kind` and the event region; for
a rejected entry, `kind`, the key and the reason id. Equal means the
entry is present. Different is the reason `ledger-entry-mismatch`:
the ingest ends with `ledger-incomplete`, and `doctor` reports the
mismatch under its own id (04 4.4). The headers are
not compared whole: the `ingested` stamp of two racing writers
differs, so a whole-file compare would call each race a mismatch.

What enforces "created once, never written again", stated plainly:

- The unit test proves that creating an entry whose name exists fails
  and leaves the existing bytes unchanged.
- The I16 scan
  ([05 5.2](05-security.md#52-invariants-and-their-tests)) proves that
  `internal/ledger` has no other call that could write or remove an
  entry: it admits, in that package, a write-open only with
  `O_CREATE|O_EXCL` on a temporary name, `os.Link` only in the create
  function, `os.Remove` only on a temporary name, and a rename only
  inside a view directory. Each is a named function in the table of
  admitted call sites (05 5.2). `internal/ledger/emit` is not in this
  scan: romeu does not link it.
- Neither proves that another package, another program or a person on
  the host leaves the directory alone. That part is a promise.

## 13.6 View

The view is how a sandbox reads the ledger. The read policy is the
maintainer's: a sandbox reads its own project's events whole, other
projects' events as closed-schema fields only, and the full ledger is
read only on the host, with `romeu ledger query`.

romeu derives a project's view after each ingest of that project. It
builds the three files in memory and replaces a file, by temporary
file and rename inside `view/`, only when its bytes differ, so an
ingest that added nothing writes nothing.

| File | Content |
|---|---|
| `keys` | the keys of this project's entries, ingested and rejected, one per line, sorted |
| `own.jsonl` | this project's entries, one canonical JSON object per line: `{"key","project","ingested","event":{...}}` with each event field, or `{"key","project","ingested","rejected":"<error id>"}` |
| `others.jsonl` | other projects' ingested entries: `{"policy":<n>,"project","ingested","event":{...}}` with the event fields the view policy allows, and no key |

Lines are ordered by `ingested`, then `project`, then key. The key of
another project's entry is left out because it is the digest of bytes
that include `id`, which does not cross projects, and a sandbox could
test guesses against it.

**View policy.** The cross-project allowlist is one current policy
with its own version, `ledger-view/v<n>`. It is the struct tags on the
event type (the "In the cross-project view" column of 13.2), applied
at derivation time to each stored entry whatever its event version.
It is not tied to an event's own version: with more than one version
ingested, a sandbox could otherwise keep writing the version with the
widest allowlist. Changing it is a diff on the `ledger` surface with a
regenerated golden file.

What I33 guards: the ledger directory is under host state and never
mounted (I20); the view mount is read-only, and `julieta setup`
exits 2 with `ledger-view-writable` when it can write there, so `run`
does not attach; `others.jsonl` is produced by one function from the
generated allowlist.

## 13.7 Versions

- **Event format.** Each event carries `runtime-event/v<n>`. julieta
  writes the current version. Stored entries are never migrated: that
  would be an edit. A new version is additive; a breaking change is a
  new major with its own ADR.
- **Membership tables only grow.** The tables of `type`, `tool` and
  `ref` are bound to the release, and the ingest window to the event
  version alone. So a release that removed a member (a catalog `tools`
  key, an error id, a probe id, a command) would reject, on the first
  `run` after the upgrade, each spool file an older julieta wrote with
  that member. In v1 a member is only ever added; removing one is a
  breaking change of the event format. The cost is that a dead key
  stays in its table until a major. Validating against the previous
  release's tables is a
  [deferred decision](../spec.md#deferred-decisions).
- **Ingest window.** romeu at version N ingests N and N-1, the rule
  the manifest protocol has (03 3.6). A version older than the window
  is rejected (`event-version`). A newer one is skipped as
  `version-newer` and stays in the spool, so it is ingested after
  romeu is upgraded. In v1 the window holds one version.
- `runtime-event`, `ledger-entry` and `ledger-view` are rows of
  `internal/spec/versions.go` (the `contracts` surface), and
  `runtime-event` has a generated JSON Schema in `schemas/`.

## 13.8 Starting values

These are proposed defaults. The maintainer accepted the design as a
whole and has not read these values one by one; the plan may tune
them, and a [deferred decision](../spec.md#deferred-decisions) says
what reopens them. This table is the one place they are written.

| Value | Default |
|---|---|
| size cap of one spool file | 64 KiB |
| names examined by one ingest | 20 000 |
| paths reported per violation reason | 10 |
| `exit_code` | -1 to 255 |
| `duration_ms` | 0 to 86 400 000 (one day) |
| ledger size at which `doctor` warns | 64 MiB |

The ledger only grows. Past the size in the last row `doctor` warns,
and v1 does not refuse an ingest. View derivation reads the whole
ledger, so its cost grows with it; the same warning is what reopens
rotation.

## 13.9 Consumers

| Consumer | Reads | Through |
|---|---|---|
| the lessons loop ([12 12.7](12-engineering.md#127-text-standard-and-lessons)) | `command-failed`, `hook-failed` and `note`: what failed, how often, in which tool, with which error id | `julieta event list` in a session; `romeu ledger query` on the host |
| improving romeu and julieta | `command-failed` with a `ref` (which error ids users hit) and with `tool: other` (catalog gaps); rejected entries (validator and producer bugs) | `romeu ledger query` |
| the delivery metrics ([12 12.10](12-engineering.md#1210-delivery-metrics)) | nothing | - |

The ledger owns no metric. Each input of `tools/ci dora` stays on
GitHub data and git history, and the tool does not read the ledger. A
lesson may cite ledger events beside the metrics as failure and rework
signals from inside a sandbox, which GitHub cannot see. v1 has no
analysis beyond `query` and `event list`.

## 13.10 Tests

U = unit, E = e2e with the fake sbx, H = host
([10 10.1](10-testing-style.md#101-test-levels)). The clock and the
entropy source are fixed (10 10.4). Each row states its outcome:
**entry**, **rejected** with its reason, **skip** with its path-level
reason, or what the row says.

The hostile spool table (U; I32):

| Spool content | Outcome |
|---|---|
| a symlink, a FIFO, a directory, each named `<id>.json` | skip `type`; the FIFO does not block. A device node has the same outcome and needs root to create, so probe A3 plants it and no unit row does |
| a dotfile; a name containing `..`; another extension; an uppercase `<id>` | skip `name` |
| a valid-shaped `<id>.json` name with one added newline, control character (U+0001-U+001F, U+007F-U+009F) or bidi codepoint (the I29 set), one case per class | skip `name`; the report prints the path in `termsafe`'s escaped form (I29) |
| two names that differ in case alone | the lowercase one has its own outcome; the uppercase one is skip `name` |
| a valid event padded with whitespace to the size cap | entry |
| the same, one byte over | skip `size` |
| a file that grows past the cap after the check, through the reader's `afterOpen` test hook | skip `size`; at most the cap plus one byte was read |
| a file replaced by rename after the open, through the same hook | the outcome of the bytes of the file that was opened |
| a file rewritten in place after the open, through the same hook | the outcome of the bytes read; the stored region hashes to the key |
| more violations of one reason than the reporting limit | the exact count; the first paths in name order, up to the limit |
| N names, where N is the listing bound of 13.8 read from its constant, some of them outside the layout | each name is examined, those outside the layout counted toward N; no `listing-truncated` |
| N+1 names | `listing-truncated`; the first N in name order are examined, and no more |
| a zero-byte file; a truncated document; invalid UTF-8; a byte-order mark; a duplicate key; a top-level array | rejected `event-syntax` |
| no `schema`; a malformed `schema`; version N-2 | rejected `event-version`, one entry however often it is ingested |
| version N+1, with and without a key N does not know | skip `version-newer`; no entry; ingested after the window moves |
| versions N and N-1 | entry |
| an unknown key; a missing required key; a key the `type` does not carry | rejected `event-keys` |
| an `id` that differs from the file name | rejected `event-id` |
| the spool root missing, a symlink, or a regular file | that project's ingest fails with `ledger-incomplete`; no entry; other projects are ingested |
| an empty spool | no entry; the view is derived |

The version rows use a version table injected by the test, since v1
has one version.

Per field (U; I33):

| Input | Outcome |
|---|---|
| a `type`, `tool` or `ref` that is not a table member, including one that matches the shape of a member | rejected `event-domain`; in neither view file |
| `tool: other`; a catalog key; a julieta command name | entry |
| `exit_code` of -1, 0 and the upper bound | entry |
| `exit_code` of -2 and the upper bound plus one; `duration_ms` of -1 and the upper bound plus one | rejected `event-domain` |
| `1.0`, `1e2`, `"1"`, `-0`, `null` for a number | rejected `event-domain` |
| `01` for a number | rejected `event-syntax` |
| an in-domain `id` | in `own.jsonl`; absent from another project's `others.jsonl` |
| each field of the event type | `others.jsonl` holds it only when its tag puts it on the allowlist |

Entries (U; I31):

| Case | Outcome |
|---|---|
| create an entry whose name exists | fails; the existing bytes are unchanged |
| name exists, same event region | present; no error |
| name exists, different event region or another `kind` | `ledger-entry-mismatch` |
| rejected entry exists, same key and reason; a different reason | present; `ledger-entry-mismatch` |
| N processes call the ingest function on one spool at once, without the lock | each returns without error; one entry per key; its event region equals the spool bytes; no temporary name is left, since each racer unlinks its own; then one derivation gives the golden view |
| a temporary name left in the ledger directory by a crash | the next command that ingests unlinks it under the lock, before its first ingest; the ingest function called alone leaves it |
| the same bytes in two projects | two entries |
| ingest the same spool twice | one set of entries; the second run writes nothing |
| a crash after the entries and before the view | the next ingest derives the view; no entry is duplicated |
| a full disk while writing the temporary file | no entry; the spool file stays; `ledger-incomplete` |
| mode bits | the directory is 0700, entries are 0600 |
| `observed` far from `ingested`, earlier and later | entry; the order follows `ingested` |
| the host clock moves backwards between two ingests | both entries; the view order follows the stamps, so the second ingest's entry sorts first |

On Linux CI the race row proves the link semantics of the CI
filesystem and nothing about APFS; the APFS run belongs to the host
suite.

Other rows:

- **Scan (U; I31)**: the I16 scan fails a fixture of `internal/ledger`
  with each call the list of 13.5 does not admit.
- **Drain (U)**: julieta deletes a spool file whose sha256 is a listed
  key, keeps a file with other bytes under the same name, deletes its
  own leftover temporary names, and after the same bytes are written
  again the next ingest writes and reports nothing.
- **Golden**: the two views of a fixed ledger fixture with entries of
  versions N and N-1 from two projects, under the current policy. It
  pins the order, the absence of the other project's `id` and key,
  and escaping through the CLI writer in `ledger query` (I29). The
  view is byte-identical across romeu versions unless the policy
  version changed.
- **Salvage (E)**: with an ingest that fails, `salvage` and `rm`
  complete with the exit status they would have had, and the salvage
  record holds `ledger-incomplete`.
- **View mount (X)**: a writable view mount makes `julieta setup`
  exit 2 and `run` stop before `layout up`.
- **Doctor (U)**: on fixtures, it re-hashes each ingested entry
  against its name and derives each view and compares it byte for
  byte. The real ledger is machine-local and CI does not see it.
- **Host probe (H)**: A3
  ([11 11.1](11-host-probes.md#111-block-a---sbx-and-runtime-facts-first-in-parallel-with-the-first-build-layer)),
  with the view mount and a hostile spool.
