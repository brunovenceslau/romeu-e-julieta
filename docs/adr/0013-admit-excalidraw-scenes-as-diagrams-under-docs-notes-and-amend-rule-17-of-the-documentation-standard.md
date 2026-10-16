# 13. Admit Excalidraw scenes as diagrams under docs/notes and amend rule 17 of the documentation standard

Date: 2026-10-16

## Status

Proposed

Supersedes in part [ADR 0001, adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md) (rule 17)

## Context

Rule 17 of ADR 0001 keeps diagrams as text, Mermaid or ASCII, and
admits only `.md`, `.json` and `.yaml` files under `docs/`, because
binary media cannot be diffed or read as text. On 2026-10-16 four
drawings of the architecture, from the context down to the generation
states, were made in Excalidraw to be kept beside the notes. An
Excalidraw scene is a JSON document: every box, arrow and label is a
text field, a diff shows which label changed, and a search finds a word
in it. It is not binary media. What it lacks is a rendering on GitHub,
which shows the JSON and not the drawing, and a layout that a person
can edit by hand the way they edit Mermaid.

## Decision

1. ADR 0001, rule 17: a file under `docs/notes/` may also carry the
   extension `.excalidraw`, an Excalidraw scene saved as JSON. Every
   other directory under `docs/` keeps the extensions `.md`, `.json` and
   `.yaml`, and the rest of the rule stands: no page of rule 1's path
   set, nor `README.md`, holds Markdown image syntax or an `<img` tag,
   and the state-machine diagrams of `ARCHITECTURE.md` stay generated.
2. A scene is linked from a note under `docs/notes/` that says, in
   prose, what each scene shows and which section of the specification
   it draws, so a reader who cannot open the scene still gets its
   content, and the scene is found by the same search as any page.
3. A scene is a reading, never a source: where it and the
   specification disagree, the specification holds and the scene is
   fixed. The note dates the reading.

## Consequences

- `tools/ci docs` (T086) admits `.excalidraw` under `docs/notes/` and
  fails it anywhere else under `docs/`, with a fixture for each case.
- Nothing checks a scene against the specification; review does, and
  the date in the note tells a reader how old the reading is. When a
  scene is found out of date by a review, the fix is to update it or to
  delete it with its line in the note; a scene that is found out of date
  a second time reopens this decision, toward Mermaid or a generator.
- ADR 0001 gains its `Superseded in part by` line for rule 17.
