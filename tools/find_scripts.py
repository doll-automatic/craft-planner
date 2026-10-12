import re

with open("tools/windrose_items_raw.html", "r", encoding="utf-8") as f:
    html = f.read()

scripts = re.findall(r'<script[^>]*src="([^"]+)"', html)
print("Scripts:")
for s in scripts:
    print(s)

# Also check for inline scripts with JSON or buildId / app data
inline = re.findall(r'<script[^>]*>(.*?)</script>', html, re.DOTALL)
print(f"Inline scripts count: {len(inline)}")
for i, sc in enumerate(inline):
    if len(sc) > 100:
        print(f"Inline script {i}: {sc[:150]}... (len {len(sc)})")
