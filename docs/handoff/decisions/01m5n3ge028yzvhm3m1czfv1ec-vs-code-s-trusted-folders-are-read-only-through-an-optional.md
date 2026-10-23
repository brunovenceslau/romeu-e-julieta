# VS Code's trusted folders are read only through an optional seam

What is at stake: low. Without the seam the check cannot say whether a
folder is trusted in VS Code, and it says so; it never says that a
folder is not trusted when it did not look.

Decided on 2026-10-23. Scope: this repository, the check `vscode-trust`
of the preflight and `doctor` (01 1.5, 04 4.2).

VS Code's trusted folders live in its state database (SQLite). The check
reads them only through an optional seam; when no caller supplies one,
the check reports "not checked" for the trusted folders and still checks
the setting `security.workspace.trust.enabled`.

Reopen: the first caller that can read that store, or T049 or T134,
whichever comes first.
