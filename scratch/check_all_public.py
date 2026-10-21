import os, re

public_dir = r'packages/core/src/components/public'

vn_char = re.compile(r'[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ]', re.I)
emoji_pattern = re.compile(r'[\U00010000-\U0010ffff\u2600-\u27bf]', flags=re.UNICODE)
red_patterns = [
    r'#ef4444', r'#dc2626', r'#b91c1c', r'#f87171', r'#f43f5e',
    r'#e11d48', r'#be123c', r'rgba?\s*\(\s*239', r'rgba?\s*\(\s*220', r'#fca5a5', r'#fecaca', r'#fee2e2'
]

targets = [
    'PublicAlbumGallery.vue',
    'PublicShareBody.vue',
    'PeopleSelector.vue',
    'IOSAlbumSaveModal.vue',
    'QRCodeModal.vue',
    'MultiDownloadWidget.vue'
]

for fname in targets:
    fpath = os.path.join(public_dir, fname)
    if not os.path.exists(fpath): continue
    with open(fpath, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    
    vn_count = sum(1 for l in lines if vn_char.search(l))
    # Exclude harmless ascii symbols like copyright, bullets
    emoji_count = sum(len(emoji_pattern.findall(l)) for l in lines)
    red_count = sum(1 for l in lines if any(re.search(pat, l, re.I) for pat in red_patterns))
    print(f'{fname}: VN={vn_count}, Emojis={emoji_count}, RED={red_count}')
