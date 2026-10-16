# gitsafe neutralizes every config key that runs a program

Decided on 2026-10-16, on the security audit of T023:

- Host mode sets core.askPass and core.alternateRefsCommand empty, as it
  already does for every other git config key that runs a program. A
  host clone's trusted local config then cannot run code.
- The http.* keys and a symbolic ref planted in a fetch destination stay
  under REQ-0463's trust in that config, and the spec says so.
- SSL_CERT_FILE stays on the sandbox network pass-through list of
  REQ-0470. git's libcurl-gnutls ignores it, but it adds no capability,
  serves a git built on OpenSSL, and keeps the list equal to
  tools/ci's networkPassThrough.
