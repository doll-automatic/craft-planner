import json
import re

with open("tools/materials.json", "r", encoding="utf-8") as f:
    materials = json.load(f).get("mh-wilds", [])

with open(r"C:\Users\Doll\.cursor\projects\c-Users-Doll-Documents-MyPrograms-AI\agent-tools\21da7882-befc-41ac-935f-962924f9560d.txt", "r", encoding="utf-8") as f:
    mhdb_items = json.load(f)

with open("tools/mhwilds_fandom_files.json", "r", encoding="utf-8") as f:
    fandom_files = json.load(f)

fandom_map = {}
for f in fandom_files:
    fandom_map[f["name"].lower()] = f["url"]

mhdb_by_name = {it.get("name"): it for it in mhdb_items if it.get("name")}

# Mapping from MHDB kind to Fandom kind
kind_mapping = {
    "claw": "Claw",
    "scale": "Scale",
    "shell": "Shell",
    "hide": "Hide",
    "extract": "Chemical", # or Sac
    "bone": "Bone",
    "gem": "Ball", # or Gem / Monster_Part
    "ore": "Ore",
    "certificate": "Ticket",
    "plant": "Herb",
    "powder": "Pod", # or Chemical
    "tail": "Tail",
    "skull": "Head",
    "mushroom": "Mushroom",
    "wing": "Wing",
    "medulla": "Sac",
    "seed": "Seed",
    "nut": "Seed",
    "drug": "Drug",
    "bug": "Bug",
    "poop": "Dung",
    "potion": "Medicine",
    "crystal": "Crystal",
    "plate": "Plate",
    "barrel": "Barrel",
    "web": "Spiderweb",
    "curative": "Medicine",
    "pill": "Pill",
    "honey": "Herb",
    "meat": "Meat",
    "knife": "Knife",
    "trap-tool": "Trap",
}

def capitalize_color(c):
    # e.g. 'dark-blue' -> 'Dark_Blue', 'light-green' -> 'Light_Green', 'green' -> 'Green'
    parts = c.replace("-", " ").replace("_", " ").split()
    return "_".join(p.capitalize() for p in parts)

success_count = 0
unresolved = []

for m in materials:
    jp_name = m["name"]
    mid = m["id"]
    if jp_name in mhdb_by_name:
        it = mhdb_by_name[jp_name]
        icon_info = it.get("icon", {})
        k = icon_info.get("kind", "")
        c = icon_info.get("color", "")
        
        fk = kind_mapping.get(k, k.capitalize())
        fc = capitalize_color(c)
        
        # Test exact match
        test_filename = f"MHWilds-{fk}_Icon_{fc}.png".lower()
        if test_filename in fandom_map:
            success_count += 1
            continue
        
        # Test without color or with alternative color / kind
        found = False
        for fname, url in fandom_map.items():
            if f"mhwilds-{fk.lower()}_icon" in fname and fc.lower() in fname:
                found = True
                break
        if not found:
            # check any color of that kind
            for fname, url in fandom_map.items():
                if f"mhwilds-{fk.lower()}_icon" in fname:
                    found = True
                    break
        if found:
            success_count += 1
        else:
            unresolved.append((mid, jp_name, k, c, fk, fc))
    else:
        unresolved.append((mid, jp_name, "NOT_IN_MHDB", "", "", ""))

print(f"Resolvable icons: {success_count} / {len(materials)}")
print(f"Unresolved count: {len(unresolved)}")
print("\nSample unresolved (first 20):")
for u in unresolved[:20]:
    print(" ", u)
