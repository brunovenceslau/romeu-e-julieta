# The v3 kit schema of sbx

A copy of the directory `schema/` of
[docker/sandbox-kit-spec](https://github.com/docker/sandbox-kit-spec) at
the tag `v3.0.0-m.8` (commit
`129be2ff45e8f9463450eb3cf04ddcb52c2b76e5`; the annotated tag object is
`0cfa5d261ab6d159a92deac3ec66b6edf524a3eb`), fetched from
`https://raw.githubusercontent.com/docker/sandbox-kit-spec/v3.0.0-m.8/schema/`
on 2026-10-22. It is Apache-2.0, `Copyright 2026 Docker, Inc.`;
`REUSE.toml` here records it. One change was made: in
`schema/capabilities/com.docker.sandbox/network-policy@2.schema.json`
the one U+2014 of a description is written as the JSON escape
`\u2014`, which decodes to the same value, because `tools/ci hygiene`
refuses the character in a tracked file.

`tools/ci schema` reads this copy from the commit it judges, validates
every v3 kit descriptor under `examples/` against
`schema/kit.schema.json`, which composes the files below it by
reference, and requires those descriptors to carry its modeline at the
same tag (REQ-0836). The product kits under `kits/` are not judged yet:
`kits/git-ssh-sign` does not validate against this tag's schema (its
ssh-agent capability has no `phase`). The frontend pin of
`kits/pins.yaml` and this tag move together: a new frontend milestone
copies the new tag's `schema/` here and changes `kitSpecTag` in
`../../kits.go`.

The upstream tests of these files, `spec/schema_test.go`, pin the
schemas' constants to the upstream Go types of package `spec`, which
this repository does not have, so they are not copied.
