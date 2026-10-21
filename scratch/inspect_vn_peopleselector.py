import re

vn_char = re.compile(r'[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ]', re.I)
emoji_pattern = re.compile(r'[\U00010000-\U0010ffff\u2600-\u27bf]', flags=re.UNICODE)

with open('packages/core/src/components/public/PeopleSelector.vue', 'r', encoding='utf-8') as f:
    for i, l in enumerate(f.readlines()):
        if vn_char.search(l):
            print(f'PeopleSelector.vue:{i+1}: {ascii(l)}')

with open('packages/core/src/components/public/PublicShareBody.vue', 'r', encoding='utf-8') as f:
    for i, l in enumerate(f.readlines()):
        em = emoji_pattern.findall(l)
        if em:
            print(f'PublicShareBody.vue:{i+1}: {ascii(l)}')
