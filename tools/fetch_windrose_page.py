import urllib.request
import json
import re
import os
import time

URL = "https://windrose.gaming.tools/items"

headers = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
}

req = urllib.request.Request(URL, headers=headers)
try:
    with urllib.request.urlopen(req) as resp:
        html = resp.read().decode('utf-8', errors='ignore')
        print(f"Fetched HTML, length: {len(html)}")
        with open("tools/windrose_items_raw.html", "w", encoding="utf-8") as out:
            out.write(html)
except Exception as e:
    print(f"Error: {e}")
