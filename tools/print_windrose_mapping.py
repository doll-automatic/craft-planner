import json

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
    if target:
        print(f"{mid:22} -> id: {target.get('id'):32} name: {target.get('name'):24} icon: {target.get('iconPath')}")
    else:
        print(f"FAILED: {mid}")
