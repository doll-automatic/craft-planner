import json

with open("tools/windrose_site_items.json", "r", encoding="utf-8") as f:
    site_items = json.load(f)

# Search items by category or keywords
terms = ["metal", "ingot", "iron", "fish", "meat", "corn", "seed", "cloth", "fabric", "plank", "plate", "armor", "food", "t03", "t02", "t01"]

for si in site_items:
    sid = si.get("id", "").lower()
    name = si.get("name", "").lower()
    cat = si.get("mainCategoryId", "")
    cats = si.get("categories", [])
    # let's look for anything with "resource" or "craft"
    if "resource" in str(cats) or "resources" in sid or "craft" in sid or "itemicon_craft" in si.get("iconPath", "") or "itemicon_resources" in si.get("iconPath", ""):
        print(f"[{sid}] name='{si.get('name')}' icon='{si.get('iconPath')}'")
