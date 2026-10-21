import os

path = r'packages/core/src/components/public/PublicAlbumGallery.vue'
with open(path, 'r', encoding='utf-8') as f:
    c = f.read()

# Exact remaining replacements
c = c.replace(
    "aiSearchError.value = 'Không tìm thấy khuôn mặt tương ứng trong album này.';",
    "aiSearchError.value = 'No matching faces found in this album.';"
)

c = c.replace(
    '<span class="gallery-nav-ai-count">✨',
    '<span class="gallery-nav-ai-count">'
)

c = c.replace(
    ">Chuẩn<",
    ">Standard<"
)

c = c.replace(
    ">Lớn<",
    ">Large<"
)

c = c.replace(
    ">Nhỏ<",
    ">Compact<"
)

c = c.replace(
    ">Tỉ lệ gốc<",
    ">Original<"
)

c = c.replace(
    ">Vuông 1:1<",
    ">Square 1:1<"
)

c = c.replace(
    'alt="Khuôn mặt"',
    'alt="Face"'
)

c = c.replace(
    "Danh sách ảnh đã thích ·",
    "Favorited Photos ·"
)

c = c.replace(
    "<strong>Bảo mật riêng tư:</strong> Ảnh selfie chỉ được dùng để nhận diện và so khớp tạm thời trong phiên làm việc này, không lưu trữ vĩnh viễn trên máy chủ.",
    "<strong>Privacy Protection:</strong> Your selfie is only used for temporary on-device face matching and is never permanently stored on the server."
)

c = c.replace(
    "{{ aiSearchStep || 'AI đang quét & so khớp khuôn mặt...' }}",
    "{{ aiSearchStep || 'AI is scanning and matching faces...' }}"
)

c = c.replace(
    "Chờ một khoảng 1-2 giây...",
    "Please wait a few seconds..."
)

c = c.replace(
    ":title=\"totalFavorites > 0 ? `Xuất CSV ${totalFavorites} ảnh đã thích` : 'Xuất danh sách ảnh đã thích ra CSV'\"",
    ":title=\"totalFavorites > 0 ? `Export CSV of ${totalFavorites} favorited photos` : 'Export favorited photos list to CSV'\""
)

# Comments cleanup
c = c.replace("* - Sticky navigation bar with title, metadata, and filter pills (Tất cả / Đã thích)", "* - Sticky navigation bar with title, metadata, and filter pills (All / Favorites)")
c = c.replace("<!-- 1. HERO COVER SECTION (Tuỳ theo coverStyle: haduanh, editorial, minimal, split) -->", "<!-- 1. HERO COVER SECTION (coverStyle: haduanh, editorial, minimal, split) -->")
c = c.replace("<!-- TEMPLATE 1: HADUANH LUXURY HERO (Chuẩn phong cách tạp chí cao cấp) -->", "<!-- TEMPLATE 1: HADUANH LUXURY HERO (Luxury editorial hero style) -->")
c = c.replace("<!-- TEMPLATE 2: EDITORIAL SERIF (Căn giữa nghệ thuật) -->", "<!-- TEMPLATE 2: EDITORIAL SERIF (Centered artistic serif) -->")
c = c.replace('<!-- 2. STICKY NAVBAR (Tên album, Lọc Tất cả/Yêu thích, Nút Chọn, Menu "...")    -->', '<!-- 2. STICKY NAVBAR (Album title, All/Favorites filter, Select button, More menu) -->')
c = c.replace("<!-- Filter Pills: Tất cả / Yêu thích / Tìm thấy -->", "<!-- Filter Pills: All / Favorites / Found -->")
c = c.replace('<!-- Right: Action buttons (Chọn & "...") -->', '<!-- Right: Action buttons (Select & "...") -->')
c = c.replace("<!-- Normal mode: Chọn button -->", "<!-- Normal mode: Select button -->")
c = c.replace('<!-- The "..." More Menu (Tất cả tính năng nâng cao & công cụ cho photo được giấu gọn gàng ở đây!) -->', '<!-- The "..." More Menu (Advanced features & photographer tools) -->')
c = c.replace("<!-- Chia sẻ liên kết album -->", "<!-- Share album link -->")
c = c.replace("<!-- Tải ảnh đã thích (nếu có) -->", "<!-- Download favorited photos -->")
c = c.replace("<!-- TÍNH NĂNG NÂNG CAO CHO PHOTOGRAPHER: Export favorited photos (CSV) -->", "<!-- ADVANCED PHOTOGRAPHER TOOL: Export favorited photos (CSV) -->")
c = c.replace("<!-- Chuyển sang xem file FileX -->", "<!-- Switch to file list view -->")
c = c.replace("<!-- Chế độ giao diện Sáng / Tối -->", "<!-- Day / Night Theme Toggle -->")
c = c.replace("<!-- AI Face Indexing Live Banner & Report (nếu đang indexing nền) -->", "<!-- AI Face Indexing Live Banner & Report -->")
c = c.replace("<!-- 3.1 KHU VỰC TÌM ẢNH BẰNG KHUÔN MẶT (AI FACE HERO BANNER - AIFS LUXURY)   -->", "<!-- 3.1 AI FACE HERO BANNER - AIFS LUXURY -->")
c = c.replace("<!-- 3.2 ACTIVE FACE FILTER BANNER (Khi đang lọc ảnh theo khuôn mặt)          -->", "<!-- 3.2 ACTIVE FACE FILTER BANNER (Face filter active) -->")
c = c.replace("<!-- Option 1: Chụp ảnh selfie ngay -->", "<!-- Option 1: Take selfie now -->")
c = c.replace("<!-- Option 2: Chọn ảnh từ thư viện máy -->", "<!-- Option 2: Choose photo from library -->")
c = c.replace("Matching web/sharing_source: Phong cách Luxury Editorial Gallery", "Matching web/sharing_source: Luxury Editorial Gallery")
c = c.replace('/* Nav Action Buttons: Chọn & "..." */', '/* Nav Action Buttons: Select & "..." */')

with open(path, 'w', encoding='utf-8') as f:
    f.write(c)

print('Cleaned remaining lines successfully.')
