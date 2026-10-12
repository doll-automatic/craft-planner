import json
import re

with open("tools/mhwilds_fandom_files.json", "r", encoding="utf-8") as f:
    files = json.load(f)

# Let's see all distinct kind names in fandom
fandom_kinds = {}
for f in files:
    # MHWilds-<Kind>_Icon_<Color>.png
    m = re.match(r'MHWilds-([A-Za-z_-]+)_Icon(?:_([A-Za-z_]+))?\.png', f["name"])
    if m:
        kind = m.group(1)
        color = m.group(2) or "Base"
        if kind not in fandom_kinds:
            fandom_kinds[kind] = {}
        fandom_kinds[kind][color] = f["url"]

print(f"Fandom kinds count: {len(fandom_kinds)}")
for k in sorted(fandom_kinds.keys()):
    print(f"  {k} ({len(fandom_kinds[k])} colors): {list(fandom_kinds[k].keys())[:5]}")
