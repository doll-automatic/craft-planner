import json

with open("tools/materials.json", "r", encoding="utf-8") as f:
    materials = json.load(f).get("mh-wilds", [])

types = {}
for m in materials:
    t = m.get("type", "other")
    types[t] = types.get(t, 0) + 1

print("All materials count by type:")
for k, v in types.items():
    print(f"  {k}: {v}")
