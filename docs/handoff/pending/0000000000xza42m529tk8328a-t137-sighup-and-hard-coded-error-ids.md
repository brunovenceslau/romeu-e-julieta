# T137: SIGHUP and hard-coded error ids

- SIGHUP leaves process groups running, because Setpgid shields them
  from the hangup. This is a spec decision and a candidate Deferred
  decisions row.
- RJ-323 and exit 1 are hard-coded in internal/procgroup/errors.go
  until cli.Error exists (internal/cli/errors.go, surface `contracts`);
  the task that creates cli.Error adopts them.

Reopen: the task that creates cli.Error; the next spec pass on signals.
