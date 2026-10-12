import json
import re

with open("tools/materials.json", "r", encoding="utf-8") as f:
    materials = json.load(f).get("windrose", [])

with open("tools/windrose_site_items.json", "r", encoding="utf-8") as f:
    site_items = json.load(f)

print(f"Materials: {len(materials)}, Site items: {len(site_items)}")

# Build lookup by name lowercase, and by id substring
by_name = {}
for si in site_items:
    name = si.get("name", "").strip().lower()
    if name:
        by_name[name] = si

# Map our windrose items
# Let's see windrose.yaml names vs English names
# In windrose.yaml, item ids are English words like 'wood', 'hardwood', 'alchemical_base' etc.
matched = {}
unmatched = []

for m in materials:
    mid = m["id"]
    # Normalize id: e.g. alchemical_base -> "alchemical base"
    norm_name = mid.replace("_", " ").lower()
    
    # Check exact match in by_name
    if norm_name in by_name:
        matched[mid] = by_name[norm_name]
    else:
        # Check partial or special cases
        found = None
        for name, si in by_name.items():
            if name == norm_name:
                found = si
                break
        if not found:
            # Check id contains
            for si in site_items:
                sid = si.get("id", "").lower()
                if mid in sid:
                    found = si
                    break
        if found:
            matched[mid] = found
        else:
            unmatched.append(m)

print(f"Matched: {len(matched)} / {len(materials)}")
if unmatched:
    print(f"Unmatched: {len(unmatched)}")
    for u in unmatched:
        print(f"  {u['id']} ({u['name']})")
