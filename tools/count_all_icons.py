import os

icon_dir = "data/icon"
games = [d for d in os.listdir(icon_dir) if os.path.isdir(os.path.join(icon_dir, d))]

print("Game Icon Directory Status:")
total = 0
for g in sorted(games):
    path = os.path.join(icon_dir, g)
    files = [f for f in os.listdir(path) if f.endswith(".png")]
    valid_files = [f for f in files if os.path.getsize(os.path.join(path, f)) > 100]
    total += len(valid_files)
    print(f"  {g:15}: {len(valid_files)} icons (total files: {len(files)})")

print(f"\nTOTAL ICONS ACROSS ALL GAMES: {total}")
