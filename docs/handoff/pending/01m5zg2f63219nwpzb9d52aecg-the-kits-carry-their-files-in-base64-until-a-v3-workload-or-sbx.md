# The kits carry their files in base64 until a v3 workload or sbx applies a mixin's recipe

What is at stake: medium. The julieta-claude kit's first install step
holds its skills and settings template as a gzip tar in base64, and the
julieta kit's first step writes its sums with printf. It works and is
tested, but it is not how a v3 kit ships files, and the payload has to
be written again (`go test ./internal/agent/claude -run
TestKitPayloadIsTheKitFiles -update`) after each change to those files.

Scope: this repository, kits/julieta and kits/julieta-claude (T079).

Measured on 2026-10-27, in a sandbox of the v2 agent `claude-base` with
v3 mixins (the daily definition of romeu-e-julieta): sbx put no file
of a mixin's directory and no layer of its recipe into the sandbox, and
`/usr/share/sandbox/kit` did not exist (a probe kit with a `FROM scratch`
recipe). SPEC-v3 10 says a runtime lays a mixin's layers into the
composed image, which sitting M0 saw staged in a sandbox of the v3
workload. Decided on 2026-10-27: the base64 payload is interim; the
recipe (`<kit>.dockerfile` with `COPY`) is the target.

Measured on 2026-10-27 with sbx v0.50.0, in a sandbox of the v3
workload (`agent: sbx-kit-claude`, the workload of kits/pins.yaml as the
first kit, a local probe kit and julieta-claude at af294e73, the recipe
version, from git): sbx assembled one image from the kits' layers, the
probe read its recipe's file (`overlay-ok`), `/usr/share/sandbox/kit/`
held each kit's `kit.yaml` and `kit.dockerfile` only, and the settings
step of julieta-claude passed from the recipe's copy. The skills step
failed: the workload's `agent-skills@1` mounts `~/.claude/skills`
read-only, so `cp` into it cannot work there.

The target, for a v3 workload:

- julieta-claude: `julieta-claude.dockerfile` (`FROM scratch`, `COPY`
  of `managed-settings.json` and `skills/` to `/usr/share/julieta-claude/`),
  the settings step as it is, and one `com.docker.sandbox/agent-skill@1`
  per skill with `path: /usr/share/julieta-claude/skills/<skill>`, in
  place of the steps that make `~/.claude/skills` and copy into it. The
  runtime exposes a bundled skill and does not overwrite one of the same
  name, so no other kit may install a `handoff` skill.
- julieta: `julieta.dockerfile` with a `COPY` of `SHA256SUMS`, which the
  steps read from the overlay (the af294e73 version of the kit).

Reopen: the daily definitions move to a v3 workload (as 03 3.6 renders
them), or a release of sbx lays a v3 mixin's recipe over a v2 agent.
Then the kits move to the target above, and the payload and its test
are removed.
