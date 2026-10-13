/**
 * PUBLIC SHARE GALLERY - VANILLA JAVASCRIPT LOGIC
 * Tương thích toàn bộ trình duyệt hiện đại, không cần build tool hay framework
 */

(function () {
  // Cấu hình Base URL của API (Để rỗng nếu chạy cùng domain với backend)
  const API_BASE_URL = window.SHARE_API_BASE_URL || '';

  // State
  let galleryData = null;
  let itemsList = [];
  let favoritesMap = {};
  let currentFilter = 'ALL'; // 'ALL' | 'FAVORITES'
  let activeLightboxIndex = null;

  // Lấy token từ URL: Hỗ trợ /share/abcxyz hoặc ?token=abcxyz
  function getShareToken() {
    const pathMatch = window.location.pathname.match(/\/share\/([a-zA-Z0-9_-]+)/);
    if (pathMatch && pathMatch[1]) return pathMatch[1];
    const searchParams = new URLSearchParams(window.location.search);
    return searchParams.get('token') || '';
  }

  // Toast Notification
  function showToast(message) {
    const existing = document.querySelector('.share-toast');
    if (existing) existing.remove();

    const toast = document.createElement('div');
    toast.className = 'share-toast';
    toast.textContent = message;
    document.body.appendChild(toast);

    setTimeout(() => {
      toast.style.opacity = '0';
      toast.style.transition = 'opacity 0.3s ease';
      setTimeout(() => toast.remove(), 300);
    }, 2200);
  }

  // Khởi tạo và fetch dữ liệu
  async function init() {
    const token = getShareToken();
    const loadingEl = document.getElementById('loadingState');
    const errorEl = document.getElementById('errorState');
    const mainEl = document.getElementById('galleryWrapper');

    try {
      let data = null;
      const searchParams = new URLSearchParams(window.location.search);
      const jsonParam = searchParams.get('json');

      if (jsonParam) {
        // Nếu truyền trực tiếp đường dẫn file JSON qua tham số ?json=
        const res = await fetch(jsonParam);
        if (res.ok) {
          const json = await res.json();
          data = json.data || json;
        }
      }

      if (!data && token) {
        // 1. Thử gọi API backend
        try {
          const res = await fetch(`${API_BASE_URL}/api/v1/shares/${token}`, {
            headers: { Accept: 'application/json' },
          });
          if (res.ok) {
            const json = await res.json();
            data = json.data || json;
          }
        } catch {
          // bỏ qua lỗi để thử fallback
        }

        // 2. Fallback: Nếu không có API backend, nạp trực tiếp từ file JSON manifest
        if (!data) {
          const jsonCandidates = [
            `/s/${token}/f/album_manifest.json?download=1`,
            `/s/${token}/f/album.json?download=1`,
            `./album_manifest.json`,
            `./album.json`,
            `../mock_data/album_share_response.json`,
          ];
          for (const cand of jsonCandidates) {
            try {
              const res = await fetch(cand);
              if (res.ok) {
                const json = await res.json();
                data = json.data || json;
                if (data) break;
              }
            } catch {
              // thử tiếp
            }
          }
        }
      } else if (!data && window.__MOCK_SHARE_DATA__) {
        data = window.__MOCK_SHARE_DATA__.data || window.__MOCK_SHARE_DATA__;
      } else if (!data) {
        // Thử fetch file mock json cục bộ
        const mockRes = await fetch('../mock_data/album_share_response.json');
        if (mockRes.ok) {
          const json = await mockRes.json();
          data = json.data || json;
        } else {
          throw new Error('Vui lòng cung cấp mã chia sẻ trên URL (Ví dụ: ?token=xxx hoặc ?json=path/to/album.json)');
        }
      }

      if (!data) {
        throw new Error('Không thể tải bộ ảnh từ file JSON hoặc API chia sẻ.');
      }

      galleryData = data;
      const album = data.album || (data.type === 'ALBUM' ? data : null);
      itemsList = album ? album.items || [] : (data.item ? [data.item] : []);

      // Khôi phục favorites từ localStorage
      const favKey = 'album_fav_' + (token || album?.id || 'default');
      try {
        const savedFavs = localStorage.getItem(favKey);
        if (savedFavs) {
          favoritesMap = JSON.parse(savedFavs);
        }
      } catch {
        // ignore
      }

      // Gộp thêm trạng thái isFavorite từ items ban đầu nếu chưa có trong map
      itemsList.forEach((it) => {
        if (it.id && it.isFavorite && favoritesMap[it.id] === undefined) {
          favoritesMap[it.id] = true;
        }
      });

      // Render
      renderCover(album);
      renderNav(album);
      renderGrid();

      if (loadingEl) loadingEl.style.display = 'none';
      if (mainEl) mainEl.style.display = 'block';
    } catch (err) {
      if (loadingEl) loadingEl.style.display = 'none';
      if (errorEl) {
        errorEl.style.display = 'flex';
        const msgEl = document.getElementById('errorMessage');
        if (msgEl) msgEl.textContent = err.message;
      }
    }
  }

  // Render Cover (Mặc định Style Haduanh Luxury)
  function renderCover(album) {
    if (!album) return;
    const coverSection = document.getElementById('heroCover');
    if (!coverSection) return;

    const coverImg = document.getElementById('heroCoverImg');
    const titleEl = document.getElementById('heroTitle');
    const clientEl = document.getElementById('heroClient');
    const locationEl = document.getElementById('heroLocation');

    const firstImg = itemsList[0];
    const coverSrc = album.coverUrl || (firstImg ? (firstImg.originalUrl || firstImg.previewUrl) : '');

    if (coverImg && coverSrc) coverImg.src = coverSrc;
    if (titleEl) titleEl.textContent = album.title;
    if (clientEl) clientEl.textContent = album.clientName || '';
    if (locationEl) locationEl.innerHTML = `${album.location || 'COLLECTION'}<br>${album.description || ''}`;
  }

  // Render Navbar
  function renderNav(album) {
    const navTitle = document.getElementById('navTitle');
    const navMeta = document.getElementById('navMeta');
    const btnAll = document.getElementById('btnFilterAll');
    const btnFav = document.getElementById('btnFilterFav');

    const titleText = album ? album.title : 'Bộ sưu tập ảnh';
    if (navTitle) navTitle.textContent = titleText;

    updateMetaCounts();

    if (btnAll) {
      btnAll.addEventListener('click', () => {
        currentFilter = 'ALL';
        btnAll.classList.add('active');
        btnFav?.classList.remove('active');
        renderGrid();
      });
    }

    if (btnFav) {
      btnFav.addEventListener('click', () => {
        currentFilter = 'FAVORITES';
        btnFav.classList.add('active');
        btnAll?.classList.remove('active');
        renderGrid();
      });
    }
  }

  function updateMetaCounts() {
    const navMeta = document.getElementById('navMeta');
    const btnAll = document.getElementById('btnFilterAll');
    const btnFav = document.getElementById('btnFilterFav');
    const favCount = Object.values(favoritesMap).filter(Boolean).length;

    if (navMeta) {
      navMeta.textContent = `${itemsList.length} tác phẩm · ${favCount} đã thích`;
    }
    if (btnAll) btnAll.textContent = `Tất cả (${itemsList.length})`;
    if (btnFav) {
      btnFav.innerHTML = `<span class="material-symbols-outlined" style="font-size:16px;">favorite</span> Đã thích (${favCount})`;
    }
  }

  // Render Masonry Grid
  function renderGrid() {
    const gridEl = document.getElementById('masonryGrid');
    const emptyEl = document.getElementById('emptyState');
    if (!gridEl) return;

    gridEl.innerHTML = '';

    const displayItems = itemsList.filter((it) => {
      if (currentFilter === 'FAVORITES') {
        return !!favoritesMap[it.id];
      }
      return true;
    });

    if (displayItems.length === 0) {
      if (emptyEl) emptyEl.style.display = 'block';
      gridEl.style.display = 'none';
      return;
    }

    if (emptyEl) emptyEl.style.display = 'none';
    gridEl.style.display = 'block';

    displayItems.forEach((item, index) => {
      const card = document.createElement('div');
      card.className = 'photo-card';

      const thumbUrl = item.previewUrl || item.thumbnailUrl || item.originalUrl || '';
      const isFav = !!favoritesMap[item.id];

      card.innerHTML = `
        <img src="${thumbUrl}" alt="${item.filename || 'Photo'}" loading="lazy" />
        <button type="button" class="btn-heart" title="${isFav ? 'Đã thích' : 'Thích'}">
          <span class="material-symbols-outlined ${isFav ? 'filled' : ''}">favorite</span>
        </button>
      `;

      // Click card -> Mở Lightbox
      card.addEventListener('click', (e) => {
        if (e.target.closest('.btn-heart')) return;
        openLightbox(displayItems, index);
      });

      // Click Heart -> Thả tim
      const heartBtn = card.querySelector('.btn-heart');
      if (heartBtn) {
        heartBtn.addEventListener('click', (e) => {
          e.stopPropagation();
          toggleFavorite(item.id, heartBtn);
        });
      }

      gridEl.appendChild(card);
    });
  }

  // Toggle Favorite
  async function toggleFavorite(mediaId, heartBtnEl) {
    const token = getShareToken();
    const current = !!favoritesMap[mediaId];
    const newStatus = !current;

    // Optimistic UI
    favoritesMap[mediaId] = newStatus;
    updateMetaCounts();

    if (heartBtnEl) {
      const icon = heartBtnEl.querySelector('.material-symbols-outlined');
      if (newStatus) {
        icon.classList.add('filled');
      } else {
        icon.classList.remove('filled');
      }
    }

    // Nếu đang ở filter FAVORITES và bỏ thích thì render lại
    if (currentFilter === 'FAVORITES' && !newStatus) {
      renderGrid();
    }

    showToast(newStatus ? 'Đã thích ảnh' : 'Đã bỏ thích');

    // Lưu vào localStorage
    const favKey = 'album_fav_' + (token || galleryData?.album?.id || 'default');
    try {
      localStorage.setItem(favKey, JSON.stringify(favoritesMap));
    } catch {
      // ignore
    }

    if (token && API_BASE_URL) {
      try {
        await fetch(`${API_BASE_URL}/api/v1/shares/${token}/favorites/${mediaId}`, {
          method: 'POST',
        });
      } catch {
        // Giữ trạng thái local mà không revert khi chạy độc lập với file JSON
      }
    }
  }

  // Lightbox
  let lightboxItems = [];
  function openLightbox(items, index) {
    lightboxItems = items;
    activeLightboxIndex = index;

    const modal = document.getElementById('lightboxModal');
    if (!modal) return;

    modal.style.display = 'flex';
    updateLightboxContent();
  }

  function closeLightbox() {
    activeLightboxIndex = null;
    const modal = document.getElementById('lightboxModal');
    if (modal) modal.style.display = 'none';
  }

  function updateLightboxContent() {
    if (activeLightboxIndex === null || !lightboxItems[activeLightboxIndex]) return;
    const current = lightboxItems[activeLightboxIndex];

    const imgEl = document.getElementById('lightboxImg');
    const counterEl = document.getElementById('lightboxCounter');
    const nameEl = document.getElementById('lightboxFilename');
    const downloadLink = document.getElementById('lightboxDownload');
    const heartBtn = document.getElementById('lightboxHeartBtn');

    const src = current.originalUrl || current.previewUrl || '';
    if (imgEl) imgEl.src = src;
    if (counterEl) counterEl.textContent = `${activeLightboxIndex + 1} / ${lightboxItems.length}`;
    if (nameEl) nameEl.textContent = current.filename || '';
    if (downloadLink) downloadLink.href = src;

    if (heartBtn) {
      const isFav = !!favoritesMap[current.id];
      const icon = heartBtn.querySelector('.material-symbols-outlined');
      if (isFav) icon.classList.add('filled');
      else icon.classList.remove('filled');
    }
  }

  // Bàn phím điều hướng Lightbox
  window.addEventListener('keydown', (e) => {
    if (activeLightboxIndex === null) return;
    if (e.key === 'Escape') closeLightbox();
    if (e.key === 'ArrowLeft') {
      activeLightboxIndex = activeLightboxIndex > 0 ? activeLightboxIndex - 1 : lightboxItems.length - 1;
      updateLightboxContent();
    }
    if (e.key === 'ArrowRight') {
      activeLightboxIndex = activeLightboxIndex < lightboxItems.length - 1 ? activeLightboxIndex + 1 : 0;
      updateLightboxContent();
    }
  });

  // Gắn sự kiện các nút Lightbox
  document.addEventListener('DOMContentLoaded', () => {
    const closeBtn = document.getElementById('lightboxClose');
    const prevBtn = document.getElementById('lightboxPrev');
    const nextBtn = document.getElementById('lightboxNext');
    const heartBtn = document.getElementById('lightboxHeartBtn');
    const scrollBtn = document.getElementById('btnScrollGallery');

    if (closeBtn) closeBtn.addEventListener('click', closeLightbox);
    if (prevBtn) {
      prevBtn.addEventListener('click', () => {
        activeLightboxIndex = activeLightboxIndex > 0 ? activeLightboxIndex - 1 : lightboxItems.length - 1;
        updateLightboxContent();
      });
    }
    if (nextBtn) {
      nextBtn.addEventListener('click', () => {
        activeLightboxIndex = activeLightboxIndex < lightboxItems.length - 1 ? activeLightboxIndex + 1 : 0;
        updateLightboxContent();
      });
    }
    if (heartBtn) {
      heartBtn.addEventListener('click', () => {
        if (activeLightboxIndex !== null && lightboxItems[activeLightboxIndex]) {
          toggleFavorite(lightboxItems[activeLightboxIndex].id);
          updateLightboxContent();
        }
      });
    }
    if (scrollBtn) {
      scrollBtn.addEventListener('click', () => {
        document.getElementById('galleryNav')?.scrollIntoView({ behavior: 'smooth' });
      });
    }

    init();
  });
})();
