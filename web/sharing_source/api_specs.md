# ĐẶC TẢ TÀI LIỆU API (PUBLIC SHARING ENDPOINTS)

Bộ API này phục vụ cho khách hàng bên ngoài (Guest/Client) truy cập xem bộ sưu tập hoặc ảnh đơn lẻ được chia sẻ qua một `token` an toàn (Public Link), không yêu cầu đăng nhập tài khoản.

---

## 1. Lấy thông tin & nội dung liên kết chia sẻ (Get Shared Content)

Trả về toàn bộ metadata của Album hoặc Single Media, danh sách ảnh với các đường dẫn presigned URL (`thumbnailUrl`, `previewUrl`, `originalUrl`), thông tin quyền hạn (`permissions`) và số lượt yêu thích.

- **Method:** `GET`
- **Endpoint:** `/api/v1/shares/:token`
- **Yêu cầu đăng nhập:** `Không` (Public)
- **Headers:**
  ```http
  Accept: application/json
  ```

### Tham số đường dẫn (URL Parameters)
| Tham số | Kiểu dữ liệu | Bắt buộc | Mô tả |
| :--- | :--- | :--- | :--- |
| `token` | `string` | Có | Chuỗi token định danh duy nhất của link chia sẻ |

### Phản hồi thành công (200 OK) - Trường hợp chia sẻ Album (ALBUM)
```json
{
  "success": true,
  "data": {
    "type": "ALBUM",
    "share": {
      "token": "a8f9c1b2-3d4e-4f5a-b6c7-d8e9f0a1b2c3",
      "expiresAt": null,
      "permissions": ["view", "download", "style:haduanh"]
    },
    "album": {
      "id": "e4b2d3c1-8f5a-4e2b-9a1c-7f6e5d4c3b2a",
      "title": "QUỐC DUY & MINH TRANG",
      "description": "Pre-Wedding Editorial & Moments Collection",
      "clientName": "QUỐC DUY & MINH TRANG",
      "location": "Đà Lạt, Lâm Đồng",
      "eventDate": "2026-10-24T00:00:00.000Z",
      "albumType": "WEDDING",
      "coverMediaId": "m1111111-0000-0000-0000-000000000001",
      "coverStyle": "haduanh",
      "coverUrl": "https://domain.com/storage/cover.jpg",
      "itemCount": 6,
      "items": [
        {
          "id": "m1111111-0000-0000-0000-000000000001",
          "position": 1,
          "type": "IMAGE",
          "mimeType": "image/jpeg",
          "filename": "TQD_001.jpg",
          "size": 4521098,
          "width": 3840,
          "height": 2160,
          "duration": null,
          "dateTaken": "2026-10-12T08:30:00.000Z",
          "thumbnailUrl": "https://domain.com/storage/thumb_300.jpg",
          "previewUrl": "https://domain.com/storage/preview_1600.jpg",
          "originalUrl": "https://domain.com/storage/original.jpg",
          "isFavorite": true,
          "favoriteCount": 12
        }
      ]
    }
  }
}
```

### Phản hồi thành công (200 OK) - Trường hợp chia sẻ Ảnh lẻ (MEDIA)
```json
{
  "success": true,
  "data": {
    "type": "MEDIA",
    "share": {
      "token": "c7d2e3f4-5a6b-7c8d-9e0f-1a2b3c4d5e6f",
      "expiresAt": "2026-11-01T00:00:00.000Z",
      "permissions": ["view", "download"]
    },
    "item": {
      "id": "m1111111-0000-0000-0000-000000000001",
      "type": "IMAGE",
      "mimeType": "image/jpeg",
      "filename": "TQD_001_Highlight.jpg",
      "width": 3840,
      "height": 2160,
      "thumbnailUrl": "https://domain.com/storage/thumb.jpg",
      "previewUrl": "https://domain.com/storage/preview.jpg",
      "originalUrl": "https://domain.com/storage/original.jpg"
    }
  }
}
```

### Các mã lỗi thường gặp
- **`404 Not Found`**: Link không tồn tại hoặc đã bị xóa.
  ```json
  {
    "success": false,
    "error": {
      "code": "NOT_FOUND",
      "message": "Liên kết chia sẻ không tồn tại hoặc đã bị gỡ bỏ."
    }
  }
  ```
- **`403 Forbidden`**: Link bị chủ sở hữu thu hồi (Revoked).
  ```json
  {
    "success": false,
    "error": {
      "code": "REVOKED",
      "message": "Liên kết chia sẻ này đã bị thu hồi bởi tác giả."
    }
  }
  ```
- **`410 Gone`**: Link đã quá hạn sử dụng (`expiresAt < now`).
  ```json
  {
    "success": false,
    "error": {
      "code": "EXPIRED",
      "message": "Liên kết chia sẻ này đã hết hạn."
    }
  }
  ```

---

## 2. Thả tim / Bỏ thích một ảnh (Toggle Favorite)

Cho phép khách xem album bấm thả tim vào một ảnh cụ thể mà không cần đăng nhập. Kết quả sẽ tự động lưu lại vào hệ thống để chủ studio biết khách hàng thích những ảnh nào.

- **Method:** `POST`
- **Endpoint:** `/api/v1/shares/:token/favorites/:mediaId`
- **Yêu cầu đăng nhập:** `Không` (Public)

### Tham số đường dẫn (URL Parameters)
| Tham số | Kiểu dữ liệu | Bắt buộc | Mô tả |
| :--- | :--- | :--- | :--- |
| `token` | `string` | Có | Token của link chia sẻ |
| `mediaId` | `UUID string` | Có | ID của ảnh muốn thích / bỏ thích |

### Phản hồi thành công (200 OK)
```json
{
  "success": true,
  "data": {
    "favorited": true,
    "totalFavorites": 13
  }
}
```

- `favorited: true` -> Vừa bấm Thích ảnh.
- `favorited: false` -> Vừa bấm Bỏ thích ảnh.
- `totalFavorites` -> Tổng số ảnh đã được thích trong link share này.

---

## 3. Quy ước URL tài nguyên ảnh (Presigned URL Resolution)
Nếu hệ thống media lưu trữ trên S3 / MinIO tự lưu các đường dẫn dạng docker internal host (như `http://minio:9000/...`), front-end cần chuẩn hóa hostname về domain công khai qua hàm helper `resolveMediaUrl(rawUrl)` có sẵn trong mã nguồn.
