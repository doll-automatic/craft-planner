import urllib.request
import json
import os
import time

OUT_DIR = "data/icon/mh-wilds"

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

kind_mapping = {
    "claw": "Claw",
    "scale": "Scale",
    "shell": "Shell",
    "hide": "Hide",
    "extract": "Chemical",
    "bone": "Bone",
    "gem": "Ball",
    "ore": "Ore",
    "certificate": "Ticket",
    "plant": "Herb",
    "powder": "Pod",
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
    parts = c.replace("-", " ").replace("_", " ").split()
    return "_".join(p.capitalize() for p in parts)

opener = urllib.request.build_opener()
opener.addheaders = [
    ("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
    ("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8"),
    ("Accept-Language", "en-US,en;q=0.9"),
    ("Referer", "https://monsterhunter.fandom.com/"),
]

success = 0
failed = 0

for m in materials:
    mid = m["id"]
    jp_name = m["name"]
    out_file = os.path.join(OUT_DIR, f"{mid}.png")
    if os.path.exists(out_file) and os.path.getsize(out_file) > 500:
        success += 1
        continue

    if jp_name not in mhdb_by_name:
        continue

    it = mhdb_by_name[jp_name]
    icon_info = it.get("icon", {})
    k = icon_info.get("kind", "")
    c = icon_info.get("color", "")
    
    fk = kind_mapping.get(k, k.capitalize())
    fc = capitalize_color(c)
    
    target_url = None
    test_filename = f"mhwilds-{fk.lower()}_icon_{fc.lower()}.png"
    if test_filename in fandom_map:
        target_url = fandom_map[test_filename]
    else:
        for fname, url in fandom_map.items():
            if f"mhwilds-{fk.lower()}_icon" in fname and fc.lower() in fname:
                target_url = url
                break
        if not target_url:
            for fname, url in fandom_map.items():
                if f"mhwilds-{fk.lower()}_icon" in fname:
                    target_url = url
                    break

    if not target_url:
        continue

    # Strip /revision/... from fandom URL to get clean static asset
    clean_url = target_url.split("/revision/")[0]

    try:
        req = urllib.request.Request(clean_url, headers=dict(opener.addheaders))
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = resp.read()
            with open(out_file, "wb") as out:
                out.write(data)
            success += 1
            if success % 20 == 0:
                print(f"Downloaded [{success}] items... last: {mid}")
    except Exception as e:
        failed += 1
        print(f"Failed {mid}: {e}")
    
    time.sleep(0.1)

print(f"\nFinal: Success: {success}, Failed: {failed}")
