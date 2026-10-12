import json

with open("tools/windrose_site_items.json", "r", encoding="utf-8") as f:
    site_items = json.load(f)

unmatched_keys = [
    "nail", "crocodile", "flour", "water", "canvas", "plate", "plating",
    "master", "mire", "ore", "fish", "shipwright", "wheat"
]

for key in unmatched_keys:
    matches = [s for s in site_items if key in s.get("name", "").lower() or key in s.get("id", "").lower()]
    print(f"\n--- Matches for '{key}' ({len(matches)}) ---")
    for m in matches[:6]:
        print(f"  id: {m.get('id')}, name: {m.get('name')}, icon: {m.get('iconPath')}")
