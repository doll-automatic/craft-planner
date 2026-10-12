import json

with open("tools/materials.json", "r", encoding="utf-8") as f:
    materials = json.load(f).get("mh-wilds", [])

with open(r"C:\Users\Doll\.cursor\projects\c-Users-Doll-Documents-MyPrograms-AI\agent-tools\21da7882-befc-41ac-935f-962924f9560d.txt", "r", encoding="utf-8") as f:
    mhdb_items = json.load(f)

mhdb_by_name = {it.get("name"): it for it in mhdb_items if it.get("name")}

raw_items = [m for m in materials if m.get("type") in ["原材料", "中間素材", "消耗品", "弾薬"]]
print(f"Raw + intermediate materials count: {len(raw_items)}")
matched = [m for m in raw_items if m["name"] in mhdb_by_name]
print(f"Matched in MHDB: {len(matched)} / {len(raw_items)}")

# Print icon kind and color distributions
kinds = {}
for m in matched:
    it = mhdb_by_name[m["name"]]
    k = it.get("icon", {}).get("kind", "unknown")
    kinds[k] = kinds.get(k, 0) + 1

print("\nKinds in MHDB:")
for k, v in sorted(kinds.items(), key=lambda x: -x[1]):
    print(f"  {k}: {v}")
