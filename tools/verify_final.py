import os

icon_dir = "data/icon"
for g in sorted(os.listdir(icon_dir)):
    p = os.path.join(icon_dir, g)
    if os.path.isdir(p):
        print(f"{g}: {len(os.listdir(p))} files")
