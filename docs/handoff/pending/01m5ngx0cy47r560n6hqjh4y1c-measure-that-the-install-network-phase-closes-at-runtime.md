# Measure that the install network phase closes at runtime

What is at stake: high. The `julieta` kit opens `github.com:443` and
`objects.githubusercontent.com:443`, and `os-base` opens
`deb.debian.org:443`, under the network-policy phase `install`
(REQ-0592). No descriptor read so far uses that phase, and the
container e2e runs the hooks outside sbx, so it cannot see egress. If
sbx merges an unknown phase into `runtime`, those hosts stay open to the
agent for the whole life of the sandbox, and `github.com` takes gists,
issues and pushes. Undoing it is a kit change and a rebuild.

Scope: this repository.

At the first real `sbx` build of the two kits (probe B2, or sitting M2):

- The build itself passes: the install hooks reach the three hosts. A
  403 there means sbx does not read the phase `install` as the
  descriptors assume.
- From inside the created sandbox, as the agent user,
  `curl -sS -o /dev/null -w '%{http_code}' https://github.com/` and
  the same for `https://objects.githubusercontent.com/` and
  `https://deb.debian.org/` each answer 403 from the proxy.

Record the result in T077's task file and in the descriptor comments of
`kits/julieta/julieta.yaml` and `kits/os-base/os-base.yaml`. If a host
answers at runtime, the kits move the downloads to a phase sbx closes
before the agent starts, and a runtime e2e against real sbx holds it.

Reopen: the first real `sbx` build of the `julieta` or `os-base` kit.
