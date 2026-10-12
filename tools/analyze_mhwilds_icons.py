import json
import re

with open("tools/mhwilds_fandom_files.json", "r", encoding="utf-8") as f:
    files = json.load(f)

# Inspect what types of filenames exist
icon_files = [f for f in files if "icon" in f["name"].lower()]
print(f"Total icon files: {len(icon_files)}")

# Group by category pattern
patterns = {}
for f in icon_files:
    name = f["name"]
    # MHWilds-<Kind>_Icon_<Color>.png or similar
    m = re.match(r'MHWilds-([A-Za-z_-]+)_Icon', name)
    if m:
        kind = m.group(1)
        patterns[kind] = patterns.get(kind, 0) + 1
    else:
        # maybe monster icon or other
        patterns["other"] = patterns.get("other", 0) + 1

print("Icon kinds found:")
for k, count in sorted(patterns.items(), key=lambda x: -x[1])[:30]:
    print(f"  {k}: {count}")

print("\nSample icon filenames:")
for f in icon_files[:20]:
    print(" ", f["name"])
