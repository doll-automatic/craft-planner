import json

with open("tools/materials.json", "r", encoding="utf-8") as f:
    materials = json.load(f).get("windrose", [])

with open("tools/windrose_site_items.json", "r", encoding="utf-8") as f:
    site_items = json.load(f)

# Build comprehensive lookup
by_id = {s.get("id"): s for s in site_items if s.get("id")}
by_name_lower = {}
for s in site_items:
    name = s.get("name", "")
    if name:
        by_name_lower[name.lower().replace("’", "'").replace("`", "'").strip()] = s

manual_mapping = {
    "copper_nails": "did_resource_nails_t01", # Nails (crafted from copper)
    "iron_nails": "did_resource_nails_t02",   # Steel Nails (crafted from iron)
    "crocodile_hide": "did_resource_crocodileleather_t03", # Crocodile Hide Piece
    "masters_tools": "did_tradecraft_finetools_01", # Master's Tools
    "shipwrights_tools": "did_tradecraft_shiptools_01", # Shipwright's Tools
    "sewing_tools": "did_tradecraft_threadsneedles_01", # Sewing Tools
    "smithing_flux": "did_tradecraft_blacksmithflux_01", # Smithing Flux
    "rough_hide": "did_resource_leather_t01", # let's check
    "meat": "did_resource_meat_t01", # let's check
    "raw_fish": "did_resource_fishmeat_t02", # Fish Fillet
    "undead_essence": "did_resource_undeadessence_t02", # Undead Heart / Undead Essence
    "rum_bottle": "did_tradecraft_alcohol_sugarcane", # or brandy/madeira/whiskey
    "fresh_water": "did_resource_claybottle_t01", # fallback bottle or clay pot
    "wheat": "cid_food_raw_corn_t02", # or cornmeal
    "flour": "did_resource_cornmeal_t02", # Cornmeal (milled flour equivalent in Windrose)
    "heavy_canvas": "did_resource_fabric_t01", # or broadcloth t04
    "iron_armor_plating": "did_metal_ingot_iron_t02",
    "mire_metal_ore": "did_metal_ingot_ancient_t03", # or raw_ore
    "wooden_plank": "did_resource_plankswood_t01",
}

mapped_all = {}
for m in materials:
    mid = m["id"]
    if mid in manual_mapping and manual_mapping[mid] in by_id:
        mapped_all[mid] = by_id[manual_mapping[mid]]
        continue
    
    # check exact name
    clean_id = mid.replace("_", " ").lower()
    if clean_id in by_name_lower:
        mapped_all[mid] = by_name_lower[clean_id]
        continue
    
    # check if mid in site_item id
    found = None
    for sid, s in by_id.items():
        if mid in sid.lower() or sid.lower() in mid:
            found = s
            break
    if found:
        mapped_all[mid] = found
    else:
        # check partial name match
        for name, s in by_name_lower.items():
            if clean_id in name or name in clean_id:
                mapped_all[mid] = s
                break

print(f"Total mapped: {len(mapped_all)} / {len(materials)}")
missing = [m for m in materials if m["id"] not in mapped_all]
print(f"Missing: {[m['id'] for m in missing]}")
