# A wait loop on no tools/ci process anywhere never ends

On 2026-10-23 an agent waited until no `tools/ci all` process ran on
the machine, while other builders ran it for hours, so the wait never
ended.

Wait only on the agent's own process or file, never on the absence of a
process anywhere on the machine.
