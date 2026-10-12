import json

with open("tools/materials.json", "r", encoding="utf-8") as f:
    mh_items = json.load(f).get("mh-wilds", [])

print(f"Total MH-Wilds material items: {len(mh_items)}")
types = set(it.get("type") for it in mh_items)
facilities = set(it.get("facility") for it in mh_items)
print("Types:", types)
print("Facilities:", facilities)

raw_count = sum(1 for it in mh_items if it.get("is_raw"))
print(f"Raw items: {raw_count}, Intermediate: {len(mh_items) - raw_count}")

# Print first 20 items
for it in mh_items[:25]:
    print(f"  {it['id']}: {it['name']} ({it['type']}, {it['facility']})")
