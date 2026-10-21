import re

vn_char = re.compile(r'[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ]', re.I)
emoji_pattern = re.compile(r'[\U00010000-\U0010ffff\u2600-\u27bf]', flags=re.UNICODE)
red_patterns = [
    r'#ef4444', r'#dc2626', r'#b91c1c', r'#f87171', r'#f43f5e',
    r'#e11d48', r'#be123c', r'rgba?\s*\(\s*239', r'rgba?\s*\(\s*220', r'#fca5a5', r'#fecaca', r'#fee2e2'
]

path = r'packages/core/src/components/public/PublicAlbumGallery.vue'
with open(path, 'r', encoding='utf-8') as f:
    lines = f.readlines()

print('=== CHECKING PublicAlbumGallery.vue ===')
vn_count = 0
for i, l in enumerate(lines):
    if vn_char.search(l):
        vn_count += 1
        safe_l = l.strip().encode('ascii', errors='replace').decode('ascii')
        print(f'VN Line {i+1}: {safe_l[:100]}')

emoji_count = 0
for i, l in enumerate(lines):
    emojis = emoji_pattern.findall(l)
    if emojis:
        emoji_count += len(emojis)
        print(f'Emoji Line {i+1}: count={len(emojis)}')

red_count = 0
for i, l in enumerate(lines):
    for pat in red_patterns:
        if re.search(pat, l, re.I):
            red_count += 1
            print(f'RED Line {i+1}: {l.strip()[:80]}')

print(f'Total VN lines: {vn_count}')
print(f'Total Emojis: {emoji_count}')
print(f'Total RED lines: {red_count}')
