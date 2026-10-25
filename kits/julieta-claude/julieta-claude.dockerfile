# SPDX-FileCopyrightText: 2026 Bruno Venceslau
# SPDX-License-Identifier: GPL-3.0-only
#
# The content of the julieta-claude kit: an overlay that ships the
# managed-settings template and the skills at /usr/share/julieta-claude/,
# where the install steps of julieta-claude.yaml read them. sbx stages
# only kit.yaml and kit.dockerfile at /usr/share/sandbox/kit/<name>/
# (SPEC-v3 10, sitting M0), never the other files of a kit's directory;
# the first build of this kit (B2, S5, 2026-10-25) failed on that. The
# build context is the kit's directory (SPEC-v3 4, dockerfile).
FROM scratch
COPY managed-settings.json /usr/share/julieta-claude/managed-settings.json
COPY skills/ /usr/share/julieta-claude/skills/
