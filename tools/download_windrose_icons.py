import urllib.request
import json
import os
import time

CDN_PREFIX = "https://cdn-hosted.gaming.tools/windrose"
OUT_DIR = "data/icon/windrose"
os.makedirs(OUT_DIR, exist_ok=True)

with open("tools/materials.json", "r", encoding="utf-8") as f:
    materials = json.load(f).get("windrose", [])

with open("tools/windrose_site_items.json", "r", encoding="utf-8") as f:
    site_items = json.load(f)

by_id = {s.get("id"): s for s in site_items if s.get("id")}
by_name_lower = {}
for s in site_items:
    name = s.get("name", "")
    if name:
        by_name_lower[name.lower().replace("’", "'").replace("`", "'").strip()] = s

manual_mapping = {
    "clay": "did_resource_clay_t01",
    "copper_nails": "did_resource_nails_t01",
    "iron_nails": "did_resource_nails_t02",
    "crocodile_hide": "did_resource_crocodileleather_t03",
    "masters_tools": "did_tradecraft_finetools_01",
    "shipwrights_tools": "did_tradecraft_shiptools_01",
    "sewing_tools": "did_tradecraft_threadsneedles_01",
    "smithing_flux": "did_tradecraft_blacksmithflux_01",
    "raw_fish": "did_resource_fishmeat_t02",
    "undead_essence": "did_resource_undeadessence_t02",
    "rum_bottle": "did_tradecraft_alcohol_sugarcane",
    "fresh_water": "did_resource_claybottle_t01",
    "wheat": "cid_food_raw_corn_t02",
    "flour": "did_resource_cornmeal_t02",
    "heavy_canvas": "did_resource_broadcloth_t04",
    "iron_armor_plating": "did_metal_ingot_iron_t02",
    "mire_metal_ore": "did_metal_ingot_ancient_t03",
    "wooden_plank": "did_resource_plankswood_t01",
}

headers = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
}

success = 0
failed = []

for m in materials:
    mid = m["id"]
    target = None
    if mid in manual_mapping and manual_mapping[mid] in by_id:
        target = by_id[manual_mapping[mid]]
    else:
        clean_id = mid.replace("_", " ").lower()
        if clean_id in by_name_lower:
            target = by_name_lower[clean_id]
        else:
            for sid, s in by_id.items():
                if mid in sid.lower():
                    target = s
                    break

    if not target or not target.get("iconPath"):
        print(f"No target/iconPath for {mid}")
        failed.append(mid)
        continue

    icon_path = target["iconPath"]
    if not icon_path.startswith("http"):
        img_url = CDN_PREFIX + icon_path
    else:
        img_url = icon_path

    out_file = os.path.join(OUT_DIR, f"{mid}.png")
    
    # Download
    req = urllib.request.Request(img_url, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = resp.read()
            with open(out_file, "wb") as out:
                out.write(data)
            success += 1
            print(f"Downloaded [{success}/56]: {mid}.png ({len(data)} bytes) from {img_url}")
    except Exception as e:
        print(f"Failed to download {mid} from {img_url}: {e}")
        failed.append(mid)
    
    time.sleep(0.05)

print(f"\nDone! Successfully downloaded: {success}/56. Failed: {len(failed)}")
if failed:
    print(f"Failed list: {failed}")
