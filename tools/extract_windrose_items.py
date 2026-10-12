import json

with open("tools/windrose_svelte_body.txt", "r", encoding="utf-8") as f:
    body = f.read()

parsed = json.loads(body)

def resolve(idx):
    val = parsed[idx]
    if isinstance(val, dict):
        res = {}
        for k, v in val.items():
            if isinstance(v, int) and v < len(parsed):
                # resolve primitive or shallow
                target = parsed[v]
                if isinstance(target, (str, int, float, bool)):
                    res[k] = target
                elif isinstance(target, list) and all(isinstance(x, int) and x < len(parsed) and isinstance(parsed[x], (str, int, float, bool)) for x in target):
                    res[k] = [parsed[x] for x in target]
                else:
                    res[k] = target
            else:
                res[k] = v
        return res
    return val

# Let's inspect objects in parsed[0]
items_list = []
for idx in parsed[0]:
    if isinstance(idx, int) and idx < len(parsed):
        item_obj = resolve(idx)
        items_list.append(item_obj)

print(f"Total resolved items from parsed[0]: {len(items_list)}")
for item in items_list[:5]:
    print(item)

# Save all items
with open("tools/windrose_site_items.json", "w", encoding="utf-8") as out:
    json.dump(items_list, out, ensure_ascii=False, indent=2)
