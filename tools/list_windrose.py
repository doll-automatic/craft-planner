import json

with open('tools/materials.json', 'r', encoding='utf-8') as f:
    data = json.load(f)

windrose_items = data.get('windrose', [])
print(f'Windrose count: {len(windrose_items)}')
for it in windrose_items:
    print(f"{it['id']}: {it['name']}")
