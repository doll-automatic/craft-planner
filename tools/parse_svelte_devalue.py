import json
import re

with open("tools/windrose_svelte_body.txt", "r", encoding="utf-8") as f:
    body = f.read()

# SvelteKit devalue is JSON array
parsed = json.loads(body)
print(f"Parsed array length: {len(parsed)}")

# In SvelteKit devalue, elements refer to indices in the parsed array.
# Let's inspect some elements to see the object structure of items!
for i, el in enumerate(parsed[:30]):
    print(f"[{i}]: {repr(el)[:100]}")
