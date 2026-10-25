# Metrics of the development process from deterministic hooks, after v1

What is at stake: low now. It is a post-v1 idea; the risk is building
it early, or building a second record beside the ledger of
[ADR 0006, record runtime events in an add-only ledger ingested on the host](../../adr/0006-record-runtime-events-in-an-add-only-ledger-ingested-on-the-host.md).

Scope: this repository, for the product after v1; proposed on
2026-10-25.

The idea: counters and latency percentiles taken at deterministic
points (the session hooks, the hook dispatcher, the guard, `julieta`
and `romeu` commands), plus the use of Claude Code itself, exposed by
an HTTP server the sandbox starts at boot and restarts when it falls
(a container only if it stays light), scraped by Prometheus and shown
in Grafana dashboards of the development process, so the tools improve
from how they are used.

Before any design, read and decide against what exists:

- the ledger of spec 13 and ADR 0006, which already records runtime
  events on the host: metrics derived from it may need no second
  writer;
- [ADR 0004, measure delivery with the five DORA metrics computed by a tool](../../adr/0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md);
- Claude Code's own telemetry export (OpenTelemetry metrics), to read
  in its documentation before writing a collector, per
  [ADR 0038, search for a maintained library before writing one](../../adr/0038-search-for-a-maintained-library-before-writing-one.md);
- the egress and process rules a long-running server inside the
  sandbox must keep (05, 06 6.4).

Reopen: v1.0.0 is released, or the maintainer asks.
