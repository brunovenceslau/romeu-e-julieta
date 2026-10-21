# julieta status lists the recorded hook failures (T154)

Done on 2026-10-21, in the pull request that carries this record:

- `julieta status` reads the hook failure store (`hooks.Failures`) and
  lists each recorded failure, with its hook, repo, stage, exit code
  and time, in its text and in its `--json` report (the member
  `hookFailures`); an unreadable file of the store is one `errors`
  entry. A later success of the same hook clears it, as the store
  already did.
- Decided on 2026-10-21: `julieta-status/v1` gains the member without a
  new version, by "Formats evolve by the rules of protobuf"
  (01m5egnpxhheh7197j08zdr618).

Closed by this record:

- "julieta status reads the hook failure store"
  (0000000000782g86tsyzte1t16), by T154.
