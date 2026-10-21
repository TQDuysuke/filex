import os, re

path = r'packages/core/src/components/public/PublicAlbumGallery.vue'
with open(path, 'r', encoding='utf-8') as f:
    content = f.read()

# 1. Loading Preload Duration & Stabilization
old_preload = """  const minTimer = new Promise<void>((resolve) => setTimeout(resolve, 400));
  void Promise.race([
    Promise.all([preloadPromise, minTimer]),
    new Promise<void>((resolve) => setTimeout(resolve, 1800)),
  ]).then(() => {
    isLoadingGallery.value = false;
  });"""

new_preload = """  // Guaranteed minimum loading duration to ensure smooth, stable initial render
  const minTimer = new Promise<void>((resolve) => setTimeout(resolve, 2000));
  void Promise.race([
    Promise.all([preloadPromise, minTimer]),
    new Promise<void>((resolve) => setTimeout(resolve, 4000)),
  ]).then(() => {
    isLoadingGallery.value = false;
  });"""

if old_preload in content:
    content = content.replace(old_preload, new_preload)
    print("Replaced preload timer with 2000ms minimum.")
else:
    print("WARNING: old_preload not found!")

# Preload more images (16 instead of 8) and use img.decode
old_slice = "props.album.items.slice(0, 8).forEach((item) => {"
new_slice = "props.album.items.slice(0, 16).forEach((item) => {"
content = content.replace(old_slice, new_slice)

old_img_loader = """        new Promise<void>((resolve) => {
          const img = new Image();
          img.onload = () => resolve();
          img.onerror = () => resolve();
          img.src = url;
        }),"""

new_img_loader = """        new Promise<void>((resolve) => {
          const img = new Image();
          img.onload = () => {
            if ('decode' in img) {
              img.decode().then(() => resolve()).catch(() => resolve());
            } else {
              resolve();
            }
          };
          img.onerror = () => resolve();
          img.src = url;
        }),"""
content = content.replace(old_img_loader, new_img_loader)

# 2. Add isUiHidden state and toggleUi function
if "const isUiHidden = ref(false);" not in content:
    content = content.replace(
        "const isLoadingGallery = ref(true);",
        "const isLoadingGallery = ref(true);\nconst isUiHidden = ref(false);\n\nfunction toggleUi() {\n  isUiHidden.value = !isUiHidden.value;\n}"
    )

# In onKeydown:
old_keydown = """  if (e.key === 'Escape') closeLightbox();
  else if (e.key === 'ArrowLeft') prevLightbox();
  else if (e.key === 'ArrowRight') nextLightbox();"""

new_keydown = """  if (e.key === 'Escape') {
    if (showInfo.value) {
      showInfo.value = false;
    } else if (isUiHidden.value) {
      isUiHidden.value = false;
    } else {
      closeLightbox();
    }
  } else if (e.key === 'ArrowLeft') {
    prevLightbox();
  } else if (e.key === 'ArrowRight') {
    nextLightbox();
  } else if (e.key === 'f' || e.key === 'F' || e.key === ' ') {
    if (activeLightboxIndex.value !== null) {
      toggleUi();
    }
  }"""
content = content.replace(old_keydown, new_keydown)

# In openLightbox:
old_open_lb = """function openLightbox(index: number) {
  activeLightboxIndex.value = index;
}"""
new_open_lb = """function openLightbox(index: number) {
  activeLightboxIndex.value = index;
  isUiHidden.value = false;
}"""
content = content.replace(old_open_lb, new_open_lb)

# 3. Lightbox template: add :class="{ 'ui-hidden': isUiHidden }" and image @click="toggleUi"
old_lb_modal = """        <div
          v-if="activeLightboxIndex !== null && currentLightboxItem"
          class="lightbox-modal"
          @touchstart="onTouchStart"
          @touchend="onTouchEnd"
        >"""

new_lb_modal = """        <div
          v-if="activeLightboxIndex !== null && currentLightboxItem"
          class="lightbox-modal"
          :class="{ 'ui-hidden': isUiHidden }"
          @touchstart="onTouchStart"
          @touchend="onTouchEnd"
        >"""
content = content.replace(old_lb_modal, new_lb_modal)

# Add Toggle Full Photo / Hide UI button in lightbox-actions
old_lb_actions = """              <!-- Info toggle -->
              <button
                type="button"
                class="lightbox-btn"
                :class="{ 'is-active': showInfo }"
                title="Photo Details"
                @click="showInfo = !showInfo"
              >"""

new_lb_actions = """              <!-- Toggle Full Photo (Hide UI) -->
              <button
                type="button"
                class="lightbox-btn"
                :class="{ 'is-active': isUiHidden }"
                :title="isUiHidden ? 'Show Controls' : 'View Full Photo (Hide UI)'"
                @click="toggleUi"
              >
                <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"/>
                </svg>
                <span style="font-size: 13px;">Full</span>
              </button>

              <!-- Info toggle -->
              <button
                type="button"
                class="lightbox-btn"
                :class="{ 'is-active': showInfo }"
                title="Photo Details"
                @click="showInfo = !showInfo"
              >"""
content = content.replace(old_lb_actions, new_lb_actions)

# Main image click to toggle UI
old_main_img = """              <img
                :src="safePhotoUrl(currentLightboxItem.originalUrl || currentLightboxItem.previewUrl, currentLightboxItem.filename)"
                :alt="currentLightboxItem.filename"
                class="lightbox-main-img"
                @load="onLightboxImageLoad($event, currentLightboxItem)"
              />"""

new_main_img = """              <img
                :src="safePhotoUrl(currentLightboxItem.originalUrl || currentLightboxItem.previewUrl, currentLightboxItem.filename)"
                :alt="currentLightboxItem.filename"
                class="lightbox-main-img"
                :class="{ 'is-fullscreen-img': isUiHidden }"
                title="Click image to toggle full photo view"
                @load="onLightboxImageLoad($event, currentLightboxItem)"
                @click="toggleUi"
              />"""
content = content.replace(old_main_img, new_main_img)

# 4. Remove ALL Red Colors
content = content.replace("color: #fca5a5;", "color: #ffffff;")
content = content.replace("background: rgba(239, 68, 68, 0.35);", "background: rgba(255, 198, 41, 0.25);")
content = content.replace("box-shadow: 0 2px 10px rgba(239, 68, 68, 0.35);", "box-shadow: 0 2px 10px rgba(255, 198, 41, 0.35);")
content = content.replace("background: rgba(239, 68, 68, 0.25);", "background: rgba(255, 198, 41, 0.25);")
content = content.replace("border-color: rgba(239, 68, 68, 0.6);", "border-color: #FFC629;")

# Selfie error banner:
old_err_banner = """.aifs-selfie-error-banner {
  padding: 12px 14px;
  border-radius: 14px;
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #dc2626;
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.dark .aifs-selfie-error-banner,
html.dark .aifs-selfie-error-banner {
  background: rgba(69, 10, 10, 0.5);
  border-color: rgba(220, 38, 38, 0.4);
  color: #f87171;
}"""

new_err_banner = """.aifs-selfie-error-banner {
  padding: 12px 14px;
  border-radius: 14px;
  background: #F8F6F0;
  border: 1px solid #E8E5DE;
  color: #171717;
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.dark .aifs-selfie-error-banner,
html.dark .aifs-selfie-error-banner {
  background: #181817;
  border-color: #292927;
  color: #F5F5F4;
}"""
content = content.replace(old_err_banner, new_err_banner)

# Replace blue #38bdf8 in quick-exif-mp
content = content.replace("color: #38bdf8;", "color: #FFC629;")
# Replace blue in lightbox-face-frame
content = content.replace("border: 2px solid rgba(56, 189, 248, 0.85);", "border: 2px solid rgba(255, 198, 41, 0.9);")
content = content.replace("background: rgba(56, 189, 248, 0.12);", "background: rgba(255, 198, 41, 0.15);")
content = content.replace("0 0 16px rgba(56, 189, 248, 0.3)", "0 0 16px rgba(255, 198, 41, 0.35)")

# 5. UI-Hidden CSS rules for Lightbox
ui_hidden_css = """
/* UI Hidden / Immersive Full Photo Mode */
.lightbox-modal.ui-hidden .lightbox-topbar,
.lightbox-modal.ui-hidden .lightbox-bottombar,
.lightbox-modal.ui-hidden .lightbox-floating-info-pill,
.lightbox-modal.ui-hidden .lightbox-faces-pill-container,
.lightbox-modal.ui-hidden .lightbox-nav-btn {
  opacity: 0 !important;
  pointer-events: none !important;
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.lightbox-topbar,
.lightbox-bottombar,
.lightbox-floating-info-pill,
.lightbox-faces-pill-container,
.lightbox-nav-btn {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.lightbox-main-img {
  cursor: pointer;
  transition: max-width 0.25s ease, max-height 0.25s ease;
}

.lightbox-modal.ui-hidden .lightbox-main-img {
  max-width: 100vw !important;
  max-height: 100vh !important;
  border-radius: 0 !important;
  box-shadow: none !important;
}

.lightbox-floating-info-pill {
  position: absolute;
  left: 20px;
  bottom: 54px;
  z-index: 40;
  display: flex;
  align-items: center;
  gap: 8px;
}

.lightbox-faces-pill-container {
  position: absolute;
  bottom: 54px;
  left: 0;
  right: 0;
  z-index: 45;
  display: flex;
  justify-content: center;
  pointer-events: none;
  padding: 0 16px;
}
"""

if ".lightbox-modal.ui-hidden" not in content:
    content += ui_hidden_css

# 6. Replace All Vietnamese Strings & Remove All Emojis
replacements = [
    # Toasts & Script strings
    ("showToast('Đã sao chép liên kết album!');", "showToast('Album link copied to clipboard!');"),
    ("showToast('Chưa có ảnh nào được yêu thích! Hãy thả tim một số ảnh trước.');", "showToast('No photos favorited yet. Add some photos to favorites first.');"),
    ("showToast('Chưa có ảnh nào được yêu thích! Hãy thả tim (❤️) một số ảnh trước.');", "showToast('No photos favorited yet. Add some photos to favorites first.');"),
    ("showToast(`Đã thêm ${selectedItemIds.value.size} ảnh vào mục yêu thích ❤️`);", "showToast(`Added ${selectedItemIds.value.size} photos to favorites.`);"),
    ("showToast(`Đã xuất ${favoriteItems.length} ảnh yêu thích ra file CSV!`);", "showToast(`Exported ${favoriteItems.length} favorited photos to CSV!`);"),
    ("showToast('Chưa chọn ảnh nào để xuất CSV!');", "showToast('No photos selected to export!');"),
    ("showToast(`Đã xuất ${selectedItems.length} ảnh đã chọn ra file CSV!`);", "showToast(`Exported ${selectedItems.length} selected photos to CSV!`);"),
    ("aiSearchStep.value = 'Đang tải ảnh selfie lên...';", "aiSearchStep.value = 'Uploading selfie photo...';"),
    ("aiSearchStep.value = 'AI đang quét & so khớp khuôn mặt...';", "aiSearchStep.value = 'AI is scanning and matching faces...';"),
    ("`Tìm kiếm thất bại: HTTP ${res.status}`", "`Face search failed: HTTP ${res.status}`"),
    ("showToast(`Đã tìm thấy ${count} ảnh có bạn!`);", "showToast(`Found ${count} photos of you!`);"),
    ("aiSearchError.value = 'Không tìm thấy khuôn mặt tương ứng trong album này.';", "aiSearchError.value = 'No matching faces found in this album.';"),
    ("aiSearchError.value = err?.message || 'Có lỗi xảy ra khi tìm kiếm khuôn mặt';", "aiSearchError.value = err?.message || 'Error occurred during face search.';"),
    
    # CSV Headers
    ("'Tên file'", "'Filename'"),
    ("'Dung lượng'", "'Size'"),
    ("'Dung lượng (Bytes)'", "'Size (Bytes)'"),
    ("'Kích thước'", "'Dimensions'"),
    ("'Chiều rộng (px)'", "'Width (px)'"),
    ("'Chiều cao (px)'", "'Height (px)'"),
    ("'Thiết bị (Camera)'", "'Camera'"),
    ("'Ống kính (Lens)'", "'Lens'"),
    ("'Khẩu độ'", "'Aperture'"),
    ("'Tốc độ chụp'", "'Shutter Speed'"),
    ("'Tiêu cự'", "'Focal Length'"),
    ("'Ngày chụp'", "'Taken At'"),
    ("'Đường dẫn xem ảnh'", "'Preview URL'"),
    ("'Đường dẫn gốc'", "'Original URL'"),

    # Splash & Nav
    ("Đang chuẩn bị bộ sưu tập ảnh...", "Preparing photo collection..."),
    ("{{ album.items.length }} ảnh", "{{ album.items.length }} {{ album.items.length === 1 ? 'photo' : 'photos' }}"),
    ("❤️ {{ totalFavorites }} yêu thích", "{{ totalFavorites }} favorited"),
    ("✓ {{ selectedItemIds.size }} đã chọn", "{{ selectedItemIds.size }} selected"),
    ("✨ {{ displayItems.length }} ảnh tìm thấy", "{{ displayItems.length }} photos found"),
    ("Tất cả ({{ album.items.length }})", "All ({{ album.items.length }})"),
    ("<span>Yêu thích</span>", "<span>Favorites</span>"),
    ('title="Đang lọc theo khuôn mặt - Bấm để xem tất cả"', 'title="Filtered by face - Click to view all"'),
    ("<span>Tìm thấy ({{ displayItems.length }})</span>", "<span>Found ({{ displayItems.length }})</span>"),
    ("<span class=\"nav-selected-badge\">{{ selectedItemIds.size }} đã chọn</span>", "<span class=\"nav-selected-badge\">{{ selectedItemIds.size }} selected</span>"),
    ('title="Chọn nhiều ảnh để tải xuống"', 'title="Select multiple photos to download"'),
    ("<span>Chọn</span>", "<span>Select</span>"),
    ('title="Tùy chọn khác"', 'title="More options"'),
    ('aria-label="Tùy chọn khác"', 'aria-label="More options"'),
    ("{{ copiedLink ? 'Đã sao chép liên kết!' : 'Chia sẻ liên kết album' }}", "{{ copiedLink ? 'Link copied!' : 'Share album link' }}"),
    ("Mã QR xem trên điện thoại", "QR code for mobile"),
    ("Tải toàn bộ album (ZIP)", "Download full album (ZIP)"),
    ("Tải các ảnh đã thích", "Download favorited photos"),
    ("Công cụ Photographer", "Photographer Tools"),
    ("Xuất CSV ảnh yêu thích", "Export favorited photos (CSV)"),
    ("Tùy chỉnh hiển thị", "View Options"),
    ("Mật độ:", "Density:"),
    (">Chuẩn<", ">Standard<"),
    (">Lớn<", ">Large<"),
    (">Nhỏ<", ">Compact<"),
    ("Khung ảnh:", "Aspect Ratio:"),
    (">Tỉ lệ gốc<", ">Original<"),
    (">Vuông 1:1<", ">Square 1:1<"),
    ("Xem danh sách file FileX", "Switch to file list view"),
    ("Giao diện: {{ isDark ? 'Tối (Night)' : 'Sáng (Day)' }}", "Theme: {{ isDark ? 'Dark (Night)' : 'Light (Day)' }}"),

    # AI Status Banner
    ("AI Face Indexing đang tiến hành", "AI Face Indexing in progress"),
    ("Đã xử lý {{ aifsStatus.indexed_photos ?? 0 }} / {{ aifsStatus.total_photos ?? 0 }} ảnh", "Processed {{ aifsStatus.indexed_photos ?? 0 }} / {{ aifsStatus.total_photos ?? 0 }} photos"),
    ("{{ aifsStatus.detected_faces }} khuôn mặt", "{{ aifsStatus.detected_faces }} faces"),
    ("{{ aifsStatus.clusters_count }} người", "{{ aifsStatus.clusters_count }} people"),

    # AI Face Hero Section
    ("Tìm ảnh bằng khuôn mặt", "Find photos by face"),
    ("Tải ảnh selfie để AI tự động tìm kiếm tất cả các ảnh có mặt bạn", "Take or upload a selfie for AI to automatically find all your photos in this album"),
    ("<span>Chụp hoặc tải ảnh selfie</span>", "<span>Take or upload selfie</span>"),
    ("Hoặc chạm vào khuôn mặt trong album ({{ clusteredFaces.length }} người):", "Or click on a person in this album ({{ clusteredFaces.length }} people):"),
    ("Xem tất cả ảnh", "View all photos"),

    # Active Face Filter Banner
    ("Kết quả tìm ảnh theo khuôn mặt", "Face Filter Results"),
    ("{{ displayItems.length }} ảnh", "{{ displayItems.length }} {{ displayItems.length === 1 ? 'photo' : 'photos' }}"),
    ("Hiển thị tất cả ảnh có mặt người này trong album", "Showing all photos featuring this person in the album"),
    ("<span>Tải toàn bộ ({{ displayItems.length }})</span>", "<span>Download all ({{ displayItems.length }})</span>"),

    # Empty States
    ("Không tìm thấy ảnh phù hợp với khuôn mặt này.", "No photos match this face filter."),
    ("Chưa có ảnh nào được thêm vào danh sách yêu thích.", "No photos have been added to favorites yet."),
    ("Không có ảnh nào trong bộ sưu tập này.", "No photos found in this collection."),
    ("Danh sách ảnh đã thích · {{ displayItems.length }} ảnh", "Favorited Photos · {{ displayItems.length }} {{ displayItems.length === 1 ? 'photo' : 'photos' }}"),
    ("<span>Xuất CSV yêu thích</span>", "<span>Export CSV</span>"),

    # Photo Card Badges & Tooltips
    ('title="Tỉ lệ khớp khuôn mặt"', 'title="Face similarity match"'),
    ("% Khớp", "% Match"),
    (":title=\"isIOS ? 'Lưu vào ảnh iOS' : 'Tải ảnh này về máy'\"", ":title=\"isIOS ? 'Save to iOS Photos' : 'Download photo'\""),
    (":title=\"favoritesMap[item.id] ? 'Đã yêu thích' : 'Thêm vào yêu thích'\"", ":title=\"favoritesMap[item.id] ? 'Favorited' : 'Add to favorites'\""),

    # Lightbox
    ('title="Bật/Tắt khung khuôn mặt nhận diện (Face Frames)"', 'title="Toggle Face Detection Frames"'),
    ("Khung mặt ({{ currentItemFaceBoxes.length }})", "Face Frames ({{ currentItemFaceBoxes.length }})"),
    ('title="Ảnh trước (Mũi tên trái)"', 'title="Previous photo (Left arrow)"'),
    ('title="Ảnh tiếp theo (Mũi tên phải)"', 'title="Next photo (Right arrow)"'),
    ("Bấm để xem tất cả ảnh", "Click to filter photos"),
    ('title="Đóng"', 'title="Close"'),

    # Face Selfie Search Modal
    ("Tải ảnh selfie để tìm ảnh có bạn trong album", "Upload a selfie to find all photos of you in this album"),
    ("<span>Chụp ảnh selfie ngay</span>", "<span>Take a selfie now</span>"),
    ("<span>Chọn ảnh từ thư viện máy</span>", "<span>Choose photo from library</span>"),
    ("<strong>Bảo mật riêng tư:</strong> Ảnh selfie chỉ được dùng để nhận diện và so khớp tạm thời trong phiên làm việc này, không lưu trữ vĩnh viễn trên máy chủ.", "<strong>Privacy Protection:</strong> Your selfie is only used for temporary on-device face matching and is never permanently stored on the server."),
    ("🔒", ""), # Remove lock emoji
    ("❤️", ""), # Remove heart emoji
    ("✨", ""), # Remove sparkles emoji
    ("✓", ""), # Remove check emoji
    ("Ảnh selfie đã chọn", "Selected selfie"),
    ("Khuôn mặt sẽ được đối chiếu với toàn bộ ảnh trong album này.", "Your face will be matched against all photos in this album."),
    ("<span>Đổi ảnh khác</span>", "<span>Choose another photo</span>"),
    ("Chờ một khoảng 1-2 giây...", "Please wait a few seconds..."),
    ("Bắt đầu tìm kiếm", "Start Search"),

    # Selection Action Bar
    ("{{ selectedItemIds.size }} / {{ displayItems.length }} đã chọn", "{{ selectedItemIds.size }} / {{ displayItems.length }} selected"),
    ("{{ selectedItemIds.size === displayItems.length ? 'Bỏ chọn hết' : 'Chọn tất cả' }}", "{{ selectedItemIds.size === displayItems.length ? 'Deselect All' : 'Select All' }}"),
    ('title="Thả tim cho tất cả ảnh đang chọn"', 'title="Add selected photos to favorites"'),
    ("<span>Thả tim</span>", "<span>Favorite</span>"),
    ('title="Xuất danh sách ảnh đã chọn sang file CSV"', 'title="Export selected photos to CSV"'),
    ("<span>Xuất CSV</span>", "<span>Export CSV</span>"),
    ("<span>{{ isIOS ? 'Lưu Photos' : 'Tải xuống' }}</span>", "<span>{{ isIOS ? 'Save Photos' : 'Download' }}</span>"),
    ('title="Thoát chế độ chọn"', 'title="Exit selection mode"'),
]

for old, new in replacements:
    if old in content:
        content = content.replace(old, new)
        # print replaced

# Write back
with open(path, 'w', encoding='utf-8') as f:
    f.write(content)

print("PublicAlbumGallery.vue updated successfully!")
