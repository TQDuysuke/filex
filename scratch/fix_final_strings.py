import os

path = r'packages/core/src/components/public/PublicAlbumGallery.vue'
with open(path, 'r', encoding='utf-8') as f:
    lines = f.readlines()

new_lines = []
for idx, line in enumerate(lines):
    line_num = idx + 1
    if "aiSearchError.value = 'Không tìm thấy" in line:
        new_lines.append("      aiSearchError.value = 'No matching faces found in this album.';\n")
    elif 'class="gallery-nav-ai-count">' in line and 'photos' in line:
        new_lines.append('                · <span class="gallery-nav-ai-count">{{ displayItems.length }} {{ displayItems.length === 1 ? \'photo\' : \'photos\' }} found</span>\n')
    elif 'Xuất CSV' in line and 'totalFavorites' in line:
        new_lines.append('                  :title="totalFavorites > 0 ? `Export CSV of ${totalFavorites} favorited photos` : \'Export favorited photos list to CSV\'"\n')
    elif line.strip() == 'Chuẩn':
        new_lines.append(line.replace('Chuẩn', 'Standard'))
    elif line.strip() == 'Lớn':
        new_lines.append(line.replace('Lớn', 'Large'))
    elif line.strip() == 'Nhỏ':
        new_lines.append(line.replace('Nhỏ', 'Compact'))
    elif line.strip() == 'Tỉ lệ gốc':
        new_lines.append(line.replace('Tỉ lệ gốc', 'Original'))
    elif line.strip() == 'Vuông 1:1':
        new_lines.append(line.replace('Vuông 1:1', 'Square 1:1'))
    elif '<strong>Bảo mật riêng tư:</strong>' in line:
        new_lines.append('                <strong>Privacy Protection:</strong> Your selfie is only used for temporary on-device face matching in memory and is never permanently stored on the server.\n')
    elif 'aifs-selfie-searching-hint' in line:
        new_lines.append('                <p class="aifs-selfie-searching-hint">Please wait a few seconds...</p>\n')
    else:
        new_lines.append(line)

with open(path, 'w', encoding='utf-8') as f:
    f.writelines(new_lines)

print('Final strings replaced successfully.')
