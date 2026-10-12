import json

with open("tools/windrose_site_items.json", "r", encoding="utf-8") as f:
    site_items = json.load(f)

for s in site_items:
    sid = s.get("id", "").lower()
    name = s.get("name", "").lower()
    icon = s.get("iconPath", "")
    if sid == "did_resource_clay_t01" or (name == "clay" and "items" in icon):
        print("Clay item:", s)
