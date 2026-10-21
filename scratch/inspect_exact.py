import re

vn_char = re.compile(r'[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ]', re.I)
path = r'packages/core/src/components/public/PublicAlbumGallery.vue'
with open(path, 'r', encoding='utf-8') as f:
    lines = f.readlines()

for i, l in enumerate(lines):
    if vn_char.search(l) or any(ord(ch) > 10000 for ch in l):
        print(f'{i+1}: {ascii(l)}')
