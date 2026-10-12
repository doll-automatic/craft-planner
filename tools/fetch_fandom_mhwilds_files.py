import urllib.request
import json
import time

all_files = []
ai_continue = ""

headers = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"
}

while True:
    url = "https://monsterhunter.fandom.com/api.php?action=query&list=allimages&aiprefix=MHWilds-&ailimit=500&format=json"
    if ai_continue:
        url += f"&aicontinue={urllib.parse.quote(ai_continue)}"
    
    req = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(req) as resp:
        data = json.load(resp)
        images = data.get("query", {}).get("allimages", [])
        for img in images:
            all_files.append({
                "name": img["name"],
                "url": img["url"]
            })
        print(f"Fetched {len(images)} images, total so far: {len(all_files)}")
        if "continue" in data and "aicontinue" in data["continue"]:
            ai_continue = data["continue"]["aicontinue"]
            time.sleep(0.1)
        else:
            break

print(f"Total MHWilds files on Fandom: {len(all_files)}")
with open("tools/mhwilds_fandom_files.json", "w", encoding="utf-8") as out:
    json.dump(all_files, out, indent=2)
