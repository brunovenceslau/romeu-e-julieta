# The hint of config-project-missing in romeu init names a settings file that does not exist yet

On 2026-10-23, `romeu init` against a private config repository that
cloned (with `--git-credential-helper osxkeychain`, on macOS) stopped
with `RJ-346 config-project-missing: branch main holds no
projects/<name>.yaml` and the hint "add the config repository's own
spec under projects/, or fix config.url in settings.yaml". During
`init` the settings file does not exist yet, so the second half of the
hint points at nothing; for `init` it should say to pass another
`--config-url`.

Reopen: the next task that touches `internal/cmd/romeu/init` (T165 is
the first in order), which gives `init` its own hint for this row.
