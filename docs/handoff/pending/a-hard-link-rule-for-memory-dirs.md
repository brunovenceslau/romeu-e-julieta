# A hard-link rule for memory dirs

What is at stake: small. A hard link gives one file two names in a
memory dir; nothing escapes the dir, since a hard link cannot cross a
mount, but a reader that counts names or salvage that copies them can
see one entry twice. A rule is one more finding of `memstore.Check`.

Scope: this repository.

Deferred on 2026-10-20, while building T035:

- The layout allowlist of 08 8.1 (REQ-0645) admits regular files,
  judged with Lstat, and says nothing of their link count; a hard link
  is a regular file with `Nlink` above 1.
- The memory dir is a mount of its own in the sandbox (REQ-0644), so a
  hard link can only join two names inside it; it cannot point at a
  file of the host outside the dir.
- No reader of v1 relies on one name per inode: an entry's id is its
  name, and the decoder checks that the id inside matches it.

Reopen: a reader or salvage relies on one name per entry.
