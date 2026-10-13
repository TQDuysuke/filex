# PUBLIC SHARING FRONTEND SOURCE PACKAGE

Gói mã nguồn này đã được tách độc lập toàn bộ phần **UI Front-End Link Share Public cho khách xem ảnh**, sẵn sàng để bạn tích hợp ngay vào bất kỳ dự án nào khác (React, Next.js, Vite, Vue, PHP/Laravel, WordPress hoặc HTML thuần).

---

## 📁 Cấu Trúc Thư Mục

```text
sharing_source/
├── README.md                      # Tài liệu tổng quan & hướng dẫn tích hợp chi tiết
├── api_specs.md                   # Đặc tả chi tiết các API (Request/Response, Errors)
│
├── mock_data/                     # Dữ liệu JSON mẫu (dùng để preview offline / mock test)
│   ├── album_share_response.json  # Mẫu response đầy đủ của GET /api/v1/shares/:token (Album)
│   ├── media_share_response.json  # Mẫu response của GET /api/v1/shares/:token (Single Media)
│   └── favorite_toggle_response.json # Mẫu response khi bấm thích POST /favorites/:mediaId
│
├── react/                         # Gói tích hợp cho dự án React / Next.js / Vite
│   ├── components/
│   │   └── PublicSharePage.tsx    # Component giao diện chính (tự chứa, đầy đủ tính năng)
│   ├── services/
│   │   └── shareApi.ts            # Hàm gọi API, xử lý URL MinIO/S3
│   ├── styles/
│   │   ├── share-gallery.css      # CSS hiệu ứng drop-shadow trái tim, scrollbar, lightbox
│   │   └── tailwind.config.snippet.js # Cấu hình mở rộng cho TailwindCSS
│   └── types/
│       └── share.types.ts         # TypeScript Interfaces chuẩn
│
└── vanilla/                       # Gói tĩnh Standalone (Không cần framework/build tool)
    ├── index.html                 # Trang HTML hoàn chỉnh, mở trực tiếp trên trình duyệt
    ├── css/
    │   └── public-share.css       # CSS thiết kế chuẩn Luxury Editorial Gallery
    └── js/
        └── public-share.js        # Logic thuần: Masonry, Thả tim, Filter, Lightbox
```

---

## 🌟 Các Tính Năng Nổi Bật của Giao Diện

1. **4 Phong Cách Ảnh Bìa (Hero Cover Templates):**
   - **`haduanh` (Luxury Editorial):** Phong cách trang bìa điện ảnh cao cấp, 2 line mạ thanh mảnh ngang màn hình, typography in hoa bold, nút *"View Gallery"* góc phải dưới.
   - **`editorial`:** Căn giữa nghệ thuật, typography font serif cổ điển, thông tin ngày chụp và địa điểm.
   - **`minimal`:** Thiết kế khung bo tròn tối giản ở giữa màn hình.
   - **`split`:** Bố cục chia 2 cột (Cột thông tin bên trái, cột ảnh dọc nổi bật bên phải).
2. **Masonry Gallery Tự Nhiên 100%:**
   - Bảo toàn tỷ lệ khung hình gốc của từng bức ảnh (ảnh dọc, ảnh ngang, ảnh vuông) mà **không bao giờ bị crop vuông cưỡng ép**.
3. **Thả Tim / Tương Tác Yêu Thích (Favorite System):**
   - Nút trái tim màu trắng thanh lịch với hiệu ứng đổ bóng `photo-heart-shadow` nổi bật trên mọi nền ảnh sáng/tối.
   - Cơ chế **Optimistic UI Update**: Chuyển trạng thái tức thì ngay khi bấm, tự động đồng bộ ngầm về backend.
4. **Bộ Lọc Thông Minh (Filter Mode):**
   - Chuyển đổi linh hoạt giữa tab **"Tất cả"** và **"Đã thích"** để cô dâu/chú rể hoặc khách hàng dễ dàng rà soát những tấm ảnh mình ưng ý nhất.
5. **Lightbox Modal Toàn Màn Hình:**
   - Xem ảnh chất lượng cao trên nền đen mờ sang trọng.
   - Hỗ trợ đầy đủ phím tắt bàn phím: `Esc` để đóng, `Mũi tên trái / phải` để chuyển ảnh tiếp theo.
   - Tích hợp nút **Tải ảnh gốc** trực tiếp từ Lightbox.

---

## 🚀 Hướng Dẫn Tích Hợp Vào Dự Án Khác

### Cách 1: Tích Hợp Vào Dự Án React / Next.js / Vite

#### 1. Cài đặt các thư viện phụ thuộc (nếu dự án chưa có)
```bash
npm install lucide-react axios
```

#### 2. Copy mã nguồn vào dự án của bạn
- Copy thư mục `sharing_source/react/` vào thư mục `src/` của dự án bạn (ví dụ đặt tại `src/modules/share/` hoặc `src/components/share/`).

#### 3. Sử dụng Component trong Route
```tsx
import React from 'react';
import { PublicSharePage } from './modules/share/components/PublicSharePage';

// Ví dụ 1: Nhúng vào React Router
// <Route path="/share/:token" element={<PublicSharePage apiBaseUrl="https://api.yourdomain.com" />} />

export const ClientGalleryView = () => {
  return (
    <PublicSharePage
      // token="abc-xyz-token" // (Nếu không truyền, component tự lấy từ URL hiện tại)
      apiBaseUrl="https://api.yourdomain.com" // Base URL của backend
    />
  );
};
```

#### 4. Nếu dự án của bạn dùng Tailwind CSS
Mở `tailwind.config.js` trong dự án của bạn và bổ sung cấu hình font từ file `sharing_source/react/styles/tailwind.config.snippet.js`:
```js
module.exports = {
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'Outfit', 'system-ui', 'sans-serif'],
        serif: ['Cormorant Garamond', 'Playfair Display', 'serif'],
      },
    },
  },
};
```

---

### Cách 2: Tích Hợp Vào Dự Án PHP / Laravel / WordPress / HTML Thuần

Bạn chỉ cần dùng thư mục `sharing_source/vanilla/`:

1. Copy toàn bộ file trong `sharing_source/vanilla/` vào thư mục public của website bạn.
2. Cấu hình địa chỉ backend API trong file `index.html` hoặc trước khi load script:
   ```html
   <script>
     window.SHARE_API_BASE_URL = 'https://api.yourdomain.com';
   </script>
   <script src="./js/public-share.js"></script>
   ```
3. Truy cập xem qua đường dẫn: `https://your-domain.com/path-to-folder/?token=TOKEN_CHIA_SE`

---

## 🧪 Chạy Thử Offline Bằng Mock JSON (Không Cần Backend)

Trong thư mục `sharing_source/mock_data/` có sẵn 3 file JSON mẫu:
1. `album_share_response.json`: Bộ ảnh cưới mẫu đầy đủ thông tin, ảnh bìa, các kích thước ảnh preview/original từ Unsplash.
2. `media_share_response.json`: Dành cho chia sẻ 1 ảnh duy nhất.
3. `favorite_toggle_response.json`: Dữ liệu phản hồi khi bấm nút yêu thích.

Bạn có thể double-click mở trực tiếp file `sharing_source/vanilla/index.html` trên trình duyệt để trải nghiệm toàn bộ giao diện ngay lập tức mà không cần bất kỳ server backend nào!

---

## 📸 Tính Năng Chia Sẻ Album Qua File JSON Manifest (Không Cần API Backend)

Hệ thống cho phép chia sẻ Album ảnh song song với dạng tải file thông thường mà **không cần dựng API backend**:

1. **Khi chia sẻ thư mục:**
   - Trong bảng điều khiển chia sẻ (*Chia sẻ / Quyền truy cập*), người dùng chọn **Chế độ chia sẻ**:
     - `Tải file thông thường`: Khách nhận liên kết tải hoặc duyệt danh sách file.
     - `Album ảnh nghệ thuật`: Hệ thống tự động tạo file manifest `album_manifest.json` (và `album.json`) lưu trực tiếp trong thư mục chia sẻ.
   - Cấu hình Album bao gồm:
     - **Tiêu đề Album** (Title)
     - **Tên khách hàng / Cặp đôi** (Client Name)
     - **Địa điểm & Ngày sự kiện** (Location & Event Date)
     - **Mô tả** (Description)
     - **Phong cách ảnh bìa** (`haduanh` Luxury, `editorial`, `minimal`, `split`)

2. **Cấu trúc file `album_manifest.json`:**
   ```json
   {
     "success": true,
     "data": {
       "type": "ALBUM",
       "share": {
         "token": "...",
         "permissions": ["view", "download", "style:haduanh"]
       },
       "album": {
         "id": "alb_...",
         "title": "QUỐC DUY & MINH TRANG",
         "clientName": "QUỐC DUY & MINH TRANG",
         "location": "Đà Lạt, Lâm Đồng",
         "eventDate": "2026-10-24",
         "coverStyle": "haduanh",
         "itemCount": 12,
         "items": [
           {
             "id": "m_1_abc",
             "filename": "PHOTO_01.jpg",
             "thumbnailUrl": "/s/{token}/f/PHOTO_01.jpg",
             "previewUrl": "/s/{token}/f/PHOTO_01.jpg",
             "originalUrl": "/s/{token}/f/PHOTO_01.jpg?download=1"
           }
         ]
       }
     }
   }
   ```

3. **Khi khách truy cập:**
   - Người xem mở link `/s/{token}`: Giao diện tự động phát hiện `album_manifest.json` và mở thẳng giao diện **Album ảnh nghệ thuật (Luxury Showcase)** với đầy đủ Hero Cover, Masonry Grid, Yêu thích (lưu trữ `localStorage`), Bộ lọc và Lightbox toàn màn hình.
   - Khách có thể bấm nút **"Xem dạng file"** để chuyển sang danh sách file tải về thông thường hoặc **"Tải ZIP"** bất kỳ lúc nào.

