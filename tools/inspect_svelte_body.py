import json
import re

with open("tools/windrose_items_raw.html", "r", encoding="utf-8") as f:
    html = f.read()

inline = re.findall(r'<script[^>]*>(.*?)</script>', html, re.DOTALL)
sc8 = inline[8]
try:
    data = json.loads(sc8)
    body = data.get("body", "")
    print(f"body type: {type(body)}, length: {len(body)}")
    with open("tools/windrose_svelte_body.txt", "w", encoding="utf-8") as out:
        out.write(body)
    
    # Let's inspect strings inside body
    # SvelteKit devalue format usually has a string table
    # Let's search for t_itemicon or .webp or item names
    icons = re.findall(r't_itemicon[a-zA-Z0-9_]+', body)
    print(f"Total icon names in body: {len(icons)}, unique: {len(set(icons))}")
    print("Sample icons:", list(set(icons))[:20])

    # Let's check for "wood", "stone", "copper", "hardwood", etc.
    for term in ["wood", "hardwood", "bark", "stone", "copper", "fabric", "plank"]:
        matches = [ic for ic in set(icons) if term in ic.lower()]
        print(f"Term '{term}': {matches}")
except Exception as e:
    print(f"Error parsing sc8: {e}")
