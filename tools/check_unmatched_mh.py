import json

with open("tools/materials.json", "r", encoding="utf-8") as f:
    materials = json.load(f).get("mh-wilds", [])

with open(r"C:\Users\Doll\.cursor\projects\c-Users-Doll-Documents-MyPrograms-AI\agent-tools\21da7882-befc-41ac-935f-962924f9560d.txt", "r", encoding="utf-8") as f:
    mhdb_items = json.load(f)

mhdb_by_name = {it.get("name"): it for it in mhdb_items if it.get("name")}

unmatched = [m for m in materials if m["name"] not in mhdb_by_name]
print(f"Unmatched in MHDB: {len(unmatched)}")
types = {}
for u in unmatched:
    t = u.get("type", "other")
    types[t] = types.get(t, 0) + 1
print("Unmatched by type:", types)

print("\nSample unmatched:")
for u in unmatched[:20]:
    print(f"  {u['id']}: {u['name']} ({u['type']})")
