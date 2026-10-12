import json

with open(r"C:\Users\Doll\.cursor\projects\c-Users-Doll-Documents-MyPrograms-AI\agent-tools\21da7882-befc-41ac-935f-962924f9560d.txt", "r", encoding="utf-8") as f:
    data = json.load(f)

print(f"Total items from wilds.mhdb.io: {len(data)}")
for it in data[:10]:
    print(f"ID: {it.get('id')}, Name: {it.get('name')}, Icon: {it.get('icon')}")
