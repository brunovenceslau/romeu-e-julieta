"""Phase 0 inventory: spec sections, lines changed by PR #36, and task citations.

Usage: python3 -I inventory.py <spec-worktree> <plan-worktree> <out-dir>
"""
import json
import pathlib
import re
import subprocess
import sys

spec_root, plan_root, out = map(pathlib.Path, sys.argv[1:4])
out.mkdir(parents=True, exist_ok=True)

# Lines of each spec file that PR #36 added or changed (new-side line numbers).
diff = subprocess.run(
    ["git", "-C", str(spec_root), "diff", "-U0", "1e63d6a^1", "1e63d6a", "--",
     "docs/spec.md", "docs/spec", "docs/adr"],
    capture_output=True, text=True, check=True).stdout
changed = {}
cur = None
for line in diff.splitlines():
    if line.startswith("+++ "):
        cur = line[6:] if line != "+++ /dev/null" else None
    m = re.match(r"@@ -\S+ \+(\d+)(?:,(\d+))? @@", line)
    if m and cur:
        start, n = int(m[1]), int(m[2] or 1)
        changed.setdefault(cur, set()).update(range(start, start + max(n, 1)))

NORM = re.compile(r"\b(must|never|always|only|shall|required|forbidden|refuses?|fails?)\b", re.I)
files = [spec_root / "docs/spec.md", *sorted((spec_root / "docs/spec").glob("*.md")),
         *sorted((spec_root / "docs/adr").glob("0*.md"))]
sections = []
for f in files:
    rel = str(f.relative_to(spec_root))
    lines = f.read_text().splitlines()
    chap = re.match(r"(\d\d)-", f.name)
    chap = chap[1] if chap and "spec/" in rel else ("index" if rel == "docs/spec.md" else "ADR " + f.name[:4])
    heads = [(i, l) for i, l in enumerate(lines, 1) if re.match(r"#{1,4} ", l)] or [(1, rel)]
    sec_id = None
    for k, (i, h) in enumerate(heads):
        end = heads[k + 1][0] - 1 if k + 1 < len(heads) else len(lines)
        title = h.lstrip("#").strip()
        m = re.match(r"(\d+\.\d+(?:\.\d+)?)\b", title)
        if m:
            sec_id = f"{chap} {m[1]}"
        cite = sec_id or chap
        body = lines[i - 1:end]
        sections.append({
            "file": rel, "start": i, "end": end, "heading": title, "cite": cite,
            "normative_lines": sum(1 for l in body if NORM.search(l)),
            "lines_changed_by_36": len(changed.get(rel, set()) & set(range(i, end + 1))),
        })

# Task citations: the Implements line of each task file, with the task's fate.
tasks = []
for t in sorted((plan_root / "docs/plan/tasks").glob("T*.md")):
    txt = t.read_text()
    fate = re.search(r"^fate:\s*(\S+)", txt, re.M)
    imp = re.search(r"^- Implements:(.*?)(?=^- \w|\Z)", txt, re.M | re.S)
    cites = sorted(set(re.findall(r"\b(\d\d \d+\.\d+(?:\.\d+)?)", imp[1] if imp else "")))
    tasks.append({"task": t.name[:4], "file": t.name, "fate": fate[1] if fate else None, "implements": cites})

live = [t for t in tasks if t["fate"] not in ("folded", "removed")]
cited = {}
for t in live:
    for c in t["implements"]:
        cited.setdefault(c, []).append(t["task"])
for s in sections:
    s["cited_by_live_tasks"] = sorted({x for c, ts in cited.items()
                                       if c == s["cite"] or c.startswith(s["cite"] + ".") for x in ts})

(out / "sections.json").write_text(json.dumps(sections, indent=1))
(out / "tasks.json").write_text(json.dumps(tasks, indent=1))
known = {s["cite"] for s in sections}
dangling = sorted({c for c in cited if c not in known and not any(c.startswith(k + ".") for k in known)})
spec_secs = [s for s in sections if not s["file"].startswith("docs/adr")]
summary = {
    "sections": len(sections),
    "spec_sections_with_normative_lines": sum(1 for s in spec_secs if s["normative_lines"]),
    "spec_sections_changed_by_36": sum(1 for s in spec_secs if s["lines_changed_by_36"]),
    "changed_and_uncited": [f'{s["cite"]} {s["heading"]}' for s in spec_secs
                            if s["lines_changed_by_36"] and not s["cited_by_live_tasks"]],
    "normative_and_uncited": [f'{s["cite"]} {s["heading"]}' for s in spec_secs
                              if s["normative_lines"] and not s["cited_by_live_tasks"]],
    "tasks": len(tasks), "live_tasks": len(live),
    "live_tasks_without_implements": [t["task"] for t in live if not t["implements"]],
    "dangling_citations": dangling,
}
(out / "summary.json").write_text(json.dumps(summary, indent=1))
print(json.dumps({k: (len(v) if isinstance(v, list) else v) for k, v in summary.items()}))
