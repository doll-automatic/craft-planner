import json
import re

with open("tools/materials.json", "r", encoding="utf-8") as f:
    materials = json.load(f).get("mh-wilds", [])

with open(r"C:\Users\Doll\.cursor\projects\c-Users-Doll-Documents-MyPrograms-AI\agent-tools\21da7882-befc-41ac-935f-962924f9560d.txt", "r", encoding="utf-8") as f:
    mhdb_items = json.load(f)

with open("tools/mhwilds_fandom_files.json", "r", encoding="utf-8") as f:
    fandom_files = json.load(f)

# Build mhdb lookup by Japanese name
mhdb_by_name = {it.get("name"): it for it in mhdb_items if it.get("name")}

# Build fandom icon file lookup by filename lower
fandom_by_filename = {f["name"].lower(): f for f in fandom_files}

print(f"Materials count: {len(materials)}")
print(f"MHDB items count: {len(mhdb_items)}")

matched_in_mhdb = 0
matched_fandom_icon = 0

for m in materials:
    jp_name = m["name"]
    if jp_name in mhdb_by_name:
        matched_in_mhdb += 1
        it = mhdb_by_name[jp_name]
        icon_info = it.get("icon", {})
        kind = icon_info.get("kind", "")
        color = icon_info.get("color", "")
        # let's test possible fandom file names
        # e.g. MHWilds-Herb_Icon_Green.png or MHWilds-Scale_Icon_Red.png
        # MHWilds-<Kind>_Icon_<Color>.png
        # Check if color or kind matches
        # Note: color in MHDB is 'green', 'blue', etc.
        # Fandom uses 'Green', 'Blue', 'Light_Blue', etc.

print(f"Matched in MHDB: {matched_in_mhdb} / {len(materials)}")
