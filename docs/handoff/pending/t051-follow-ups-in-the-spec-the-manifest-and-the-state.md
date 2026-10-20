# T051 follow-ups in the spec, the manifest and the state

What is at stake: small; each is a gap in a document or a validator
that the code of T051 works around today, caught in review if it bites.

Found by the reviews of T051 on 2026-10-20:

- Import files: a command with an output type that has a schema adds a
  type alias to its `import_<command>.go`, so `tools/ci` reaches the type
  through `all` (REQ-0971). 02 2.1, 10 (REQ-0971's "one blank-import file
  per command") and the `command` kind of `tools/new` (12, REQ-1124)
  still describe a blank import only.
- The manifest decoder checks "exactly one primary" but not that the
  primary's path lies under the home or the workspace, nor that `dir`
  and `path` are unique across repos. It needs a hostile manifest, which
  the agent already controls, so it gives nothing new.
- The julieta state file (`$HOME/.local/state/julieta/setup.json`,
  `{at, version, warnings}`) has no specified format; its reader refuses
  unknown members, so a later writer breaks older readers; setup and the
  SessionStart clear read, change and write it with no lock; and
  `default-branch-moved` warnings pile up until T035's SessionStart
  clears them. A `fetch-failed` warning carries up to 4 KiB of the
  remote's stderr, escaped for a terminal but not marked as untrusted
  for a model; cut it to one line, or label it, before SessionStart
  shows it to the agent.
- julieta checks no git floor in the sandbox (`CheckFloor` has no caller
  there); the image ships its own git.

Reopen: T035 (the state and SessionStart), the next change to
`internal/manifest`'s validators, and the next spec pull request on 02
or 12.
