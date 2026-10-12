import json
import re

with open("tools/windrose_items_raw.html", "r", encoding="utf-8") as f:
    html = f.read()

# Check for img tags or json data or links
# Let's search for "t_itemicon"
matches = re.findall(r'https?://[^\s"\'<>]+t_itemicon[^\s"\'<>]+', html)
print(f"Total t_itemicon matches: {len(matches)}")
for m in matches[:10]:
    print(m)

# Also let's check for links like /items/did_...
item_links = re.findall(r'/items/([a-zA-Z0-9_-]+)', html)
print(f"Total item links: {len(set(item_links))}")
for l in list(set(item_links))[:15]:
    print(l)
