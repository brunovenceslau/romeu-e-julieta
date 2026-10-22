# T077 waits for the A12 measurement

What is at stake: medium. Until A12 is measured, the julieta kit ships
without os-base and with a timezone that works only for UTC.

Scope: this repository.

- The branch `feat/t077-julieta-kit` holds the julieta kit without
  os-base. A separate first commit on it allows the embed directive in
  the kits embed file only (the route of ADR 0007).
- A12, the packages the workload lacks, was never measured. The workload
  has no tzdata (no /usr/share/zoneinfo), so arg `tz` is inert for any
  real zone and only UTC is right. os-base waits for the full A12 at the
  sitting.
- Unmeasured until a real build (B2 or sitting M2): whether sbx accepts
  the `install` phase of the network-policy@1 capability (the workload
  descriptor shows only `runtime`, as `{phase: {allow: [host:port]}}`),
  whether it stages SHA256SUMS beside a mixin's descriptor, and whether
  a failed install hook stops the build.
- The task file of T077 on that branch has the details.

Reopen: the A12 measurement, which then opens the pull request and plans
os-base if the set is not empty.
