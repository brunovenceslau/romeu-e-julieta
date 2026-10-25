# SPDX-FileCopyrightText: 2026 Bruno Venceslau
# SPDX-License-Identifier: GPL-3.0-only
#
# The content of the julieta kit: an overlay that ships SHA256SUMS at
# /usr/share/julieta-kit/, where the install steps of julieta.yaml check
# the downloads against it. sbx stages only kit.yaml and kit.dockerfile
# at /usr/share/sandbox/kit/<name>/ (SPEC-v3 10, sitting M0), never the
# other files of a kit's directory; the first build of the agent kit
# (B2, 2026-10-25) failed on that. The build context is the kit's
# directory (SPEC-v3 4, dockerfile).
FROM scratch
COPY SHA256SUMS /usr/share/julieta-kit/SHA256SUMS
