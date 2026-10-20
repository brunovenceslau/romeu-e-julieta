# What T080 takes from the signing check

From the security audit of T025 (#140), for the task that wires
`internal/signing` into sync, doctor and the preflight:

- Forward into the sandbox the socket path that was checked, after the
  same resolution, so the checked and the forwarded socket are one.
- Consider refusing a socket whose parent directory others may write;
  `Identities` checks the socket's owner and type only.
- `Check` is a snapshot: run it on every sync and every P command (06
  6.4 already asks for both).

Reopen: T080.
