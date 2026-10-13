import React, { useState, useEffect, useMemo, useCallback } from 'react';
import {
  Heart,
  Download,
  ChevronLeft,
  ChevronRight,
  X,
  Calendar,
  MapPin,
  ArrowDown,
  Sparkles,
} from 'lucide-react';
import {
  ShareGalleryData,
  ShareGalleryItem,
  CoverTemplate,
} from '../types/share.types';
import {
  fetchSharedContent,
  toggleMediaFavorite,
  resolveMediaUrl,
  DEFAULT_API_BASE_URL,
} from '../services/shareApi';
import '../styles/share-gallery.css';

export interface PublicSharePageProps {
  /**
   * Token chia sẻ (Nếu không truyền, component sẽ tự lấy từ pathname hoặc URL param)
   */
  token?: string;
  /**
   * Base URL của API backend (Ví dụ: 'https://api.yourdomain.com' hoặc rỗng nếu dùng cùng origin)
   */
  apiBaseUrl?: string;
  /**
   * Dữ liệu mock tĩnh nếu muốn preview offline không cần gọi API
   */
  initialData?: ShareGalleryData;
  /**
   * Callback khi người dùng thích hoặc bỏ thích một ảnh
   */
  onFavoriteChange?: (mediaId: string, isFavorited: boolean) => void;
  /**
   * Custom toast handler nếu không dùng sonner mặc định
   */
  notify?: {
    success: (msg: string) => void;
    error: (msg: string) => void;
  };
}

export const PublicSharePage: React.FC<PublicSharePageProps> = ({
  token: propToken,
  apiBaseUrl = DEFAULT_API_BASE_URL,
  initialData,
  onFavoriteChange,
  notify,
}) => {
  // 1. Xác định Token từ props hoặc từ URL hiện tại (/share/:token)
  const effectiveToken = useMemo(() => {
    if (propToken) return propToken;
    if (typeof window !== 'undefined') {
      const match = window.location.pathname.match(/\/share\/([a-zA-Z0-9_-]+)/);
      if (match && match[1]) return match[1];
      const searchParams = new URLSearchParams(window.location.search);
      const queryToken = searchParams.get('token');
      if (queryToken) return queryToken;
    }
    return '';
  }, [propToken]);

  const [data, setData] = useState<ShareGalleryData | null>(initialData || null);
  const [loading, setLoading] = useState<boolean>(!initialData);
  const [error, setError] = useState<string | null>(null);

  // Template cover: 'haduanh' | 'editorial' | 'minimal' | 'split' | 'direct'
  const [coverTemplate, setCoverTemplate] = useState<CoverTemplate>('haduanh');

  // Filter Mode: Tất cả ('ALL') hoặc Chỉ xem ảnh đã thích ('FAVORITES')
  const [filterMode, setFilterMode] = useState<'ALL' | 'FAVORITES'>('ALL');

  // Lightbox Index
  const [activeLightboxIndex, setActiveLightboxIndex] = useState<number | null>(null);

  // Bản đồ trạng thái yêu thích: { [mediaId]: boolean }
  const [favoritesMap, setFavoritesMap] = useState<Record<string, boolean>>({});

  // Helper thông báo
  const showToast = useCallback(
    (type: 'success' | 'error', msg: string) => {
      if (notify && notify[type]) {
        notify[type](msg);
      } else {
        // Fallback nhẹ nếu chưa cài thư viện Toast ngoài
        const el = document.createElement('div');
        el.className = `fixed bottom-5 right-5 z-50 px-4 py-2.5 rounded-lg text-sm text-white shadow-xl transition-all ${
          type === 'success' ? 'bg-neutral-900 border border-neutral-700' : 'bg-red-900 border border-red-700'
        }`;
        el.innerText = msg;
        document.body.appendChild(el);
        setTimeout(() => {
          el.style.opacity = '0';
          setTimeout(() => el.remove(), 300);
        }, 2200);
      }
    },
    [notify]
  );

  // 2. Fetch dữ liệu từ API
  useEffect(() => {
    if (initialData) {
      setData(initialData);
      applyShareMetadata(initialData);
      return;
    }

    if (!effectiveToken) {
      setError('Không tìm thấy mã chia sẻ (token).');
      setLoading(false);
      return;
    }

    let isMounted = true;
    const loadContent = async () => {
      setLoading(true);
      setError(null);
      try {
        const result = await fetchSharedContent(effectiveToken, apiBaseUrl);
        if (isMounted) {
          setData(result);
          applyShareMetadata(result);
        }
      } catch (err: any) {
        if (isMounted) {
          const errMsg =
            err.response?.data?.error?.message ||
            err.message ||
            'Liên kết chia sẻ không tồn tại hoặc đã hết hạn.';
          setError(errMsg);
        }
      } finally {
        if (isMounted) setLoading(false);
      }
    };

    loadContent();
    return () => {
      isMounted = false;
    };
  }, [effectiveToken, apiBaseUrl, initialData]);

  // Khởi tạo style cover và danh sách favorites ban đầu
  const applyShareMetadata = (shareData: ShareGalleryData) => {
    // Ưu tiên album.coverStyle, fallback theo permissions
    const perms = shareData.share?.permissions || shareData.permissions || [];
    const stylePerm = perms
      .find((p) => p.startsWith('style:'))
      ?.replace('style:', '') as CoverTemplate | undefined;
    const albumStyle = (shareData.album?.coverStyle as CoverTemplate) || stylePerm || 'haduanh';
    setCoverTemplate(albumStyle);

    // Điền trạng thái favorites từ backend
    const favs: Record<string, boolean> = {};
    if (shareData.album?.items) {
      shareData.album.items.forEach((item) => {
        const id = item.id || item.media?.id;
        if (id && item.isFavorite) favs[id] = true;
      });
    }
    setFavoritesMap(favs);
  };

  // 3. Xử lý Thả tim (Favorite)
  const handleToggleFavorite = async (mediaId: string, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    if (!mediaId) return;

    const currentStatus = !!favoritesMap[mediaId];
    const newStatus = !currentStatus;

    // Optimistic UI Update ngay lập tức
    setFavoritesMap((prev) => ({ ...prev, [mediaId]: newStatus }));
    if (onFavoriteChange) onFavoriteChange(mediaId, newStatus);

    if (!effectiveToken) {
      showToast('success', newStatus ? 'Đã thích ảnh' : 'Đã bỏ thích');
      return;
    }

    try {
      const res = await toggleMediaFavorite(effectiveToken, mediaId, apiBaseUrl);
      const isFav = res?.favorited ?? newStatus;
      setFavoritesMap((prev) => ({ ...prev, [mediaId]: isFav }));
      showToast('success', isFav ? 'Đã thích ảnh' : 'Đã bỏ thích ảnh');
    } catch {
      // Revert lại trạng thái cũ nếu lỗi
      setFavoritesMap((prev) => ({ ...prev, [mediaId]: currentStatus }));
      showToast('error', 'Không thể cập nhật lượt thích');
    }
  };

  // 4. Danh sách ảnh & Lọc
  const album = data?.album;
  const rawItems: ShareGalleryItem[] = useMemo(() => {
    if (album?.items) return album.items;
    if (data?.item) return [data.item];
    return [];
  }, [album, data]);

  const filteredItems = useMemo(() => {
    return rawItems.filter((item) => {
      const mediaId = item.id || item.media?.id || '';
      if (filterMode === 'FAVORITES') {
        return !!favoritesMap[mediaId];
      }
      return true;
    });
  }, [rawItems, filterMode, favoritesMap]);

  const totalLikedCount = Object.values(favoritesMap).filter(Boolean).length;

  // 5. Điều hướng bàn phím cho Lightbox (Esc, Left, Right)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (activeLightboxIndex === null) return;
      if (e.key === 'Escape') setActiveLightboxIndex(null);
      if (e.key === 'ArrowLeft') {
        setActiveLightboxIndex((prev) =>
          prev !== null && prev > 0 ? prev - 1 : filteredItems.length - 1
        );
      }
      if (e.key === 'ArrowRight') {
        setActiveLightboxIndex((prev) =>
          prev !== null && prev < filteredItems.length - 1 ? prev + 1 : 0
        );
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [activeLightboxIndex, filteredItems.length]);

  // Loading Screen
  if (loading) {
    return (
      <div className="min-h-screen bg-[#fafafa] flex flex-col items-center justify-center">
        <div className="w-10 h-10 border-2 border-neutral-300 border-t-neutral-900 rounded-full animate-spin" />
        <p className="mt-4 text-xs uppercase tracking-widest text-neutral-500 font-sans">
          Đang tải bộ sưu tập...
        </p>
      </div>
    );
  }

  // Error Screen
  if (error || !data) {
    return (
      <div className="min-h-screen bg-[#fafafa] flex flex-col items-center justify-center p-6 text-center text-neutral-800">
        <div className="w-16 h-16 rounded-full bg-neutral-200 flex items-center justify-center text-neutral-600 mb-4 shadow-sm">
          <X className="w-8 h-8" />
        </div>
        <h2 className="text-2xl font-serif font-medium tracking-tight">Không tìm thấy bộ sưu tập</h2>
        <p className="text-sm text-neutral-500 max-w-md mt-2 leading-relaxed">
          {error || 'Liên kết chia sẻ này có thể đã hết hạn hoặc bị thu hồi bởi tác giả.'}
        </p>
      </div>
    );
  }

  // Cover image fallback
  const firstItem = rawItems[0];
  const coverSrc = resolveMediaUrl(
    album?.coverUrl ||
      firstItem?.originalUrl ||
      firstItem?.previewUrl ||
      firstItem?.thumbnailUrl ||
      firstItem?.media?.urls?.original ||
      firstItem?.media?.previewUrl
  );

  return (
    <div className="min-h-screen bg-[#fafafa] text-neutral-900 font-sans selection:bg-neutral-900 selection:text-white">
      {/* ========================================================================= */}
      {/* 1. HERO COVER SECTION (Tuỳ theo template: haduanh, editorial, minimal, split) */}
      {/* ========================================================================= */}

      {/* TEMPLATE 1: HADUANH LUXURY EDITORIAL (Chuẩn phong cách tạp chí cao cấp) */}
      {album && coverTemplate === 'haduanh' && (
        <section className="relative h-screen w-full flex items-center justify-between overflow-hidden bg-neutral-950 select-none">
          {coverSrc && (
            <img
              src={coverSrc}
              alt={album.title}
              className="absolute inset-0 w-full h-full object-cover object-center filter brightness-[0.92] contrast-[1.03]"
            />
          )}

          {/* Cinematic Vignette */}
          <div className="absolute inset-0 bg-gradient-to-r from-black/55 via-black/25 to-transparent" />
          <div className="absolute inset-0 bg-gradient-to-t from-black/65 via-transparent to-black/35" />

          {/* Đường line mảnh trên cùng */}
          <div className="absolute top-10 sm:top-12 inset-x-0 w-[90%] max-w-[1700px] mx-auto border-t border-white/40 pointer-events-none" />

          {/* Title & Client Name */}
          <div className="relative z-10 w-[90%] max-w-[1700px] mx-auto px-4 sm:px-8">
            <h1 className="text-3xl sm:text-5xl md:text-6xl font-sans font-bold tracking-tight text-white uppercase max-w-2xl leading-[1.15] drop-shadow-md">
              {album.title}
            </h1>
            {album.clientName && (
              <p className="text-sm sm:text-base text-white/90 uppercase tracking-widest font-medium mt-3 drop-shadow">
                {album.clientName}
              </p>
            )}
          </div>

          {/* Đường line mảnh dưới cùng */}
          <div className="absolute bottom-20 sm:bottom-24 inset-x-0 w-[90%] max-w-[1700px] mx-auto border-b border-white/40 pointer-events-none" />

          {/* Bottom Bar: Địa điểm & Nút Xem Gallery */}
          <div className="absolute bottom-6 sm:bottom-8 inset-x-0 w-[90%] max-w-[1700px] mx-auto px-4 sm:px-8 flex items-center justify-between z-10">
            <span className="text-[11px] sm:text-xs uppercase tracking-[0.25em] font-semibold text-white/90 drop-shadow">
              {album.location || 'EDITORIAL GALLERY'} <br />
              {album.description || ''}
            </span>

            <button
              onClick={() => {
                document.getElementById('galleryContent')?.scrollIntoView({ behavior: 'smooth' });
              }}
              className="px-6 py-2.5 rounded-sm border border-white/80 hover:border-white bg-transparent hover:bg-white text-white hover:text-black text-xs uppercase tracking-[0.2em] font-semibold transition-all duration-300 shadow-sm"
            >
              View Gallery
            </button>
          </div>
        </section>
      )}

      {/* TEMPLATE 2: EDITORIAL CENTERED (Căn giữa nghệ thuật) */}
      {album && coverTemplate === 'editorial' && (
        <section className="relative h-[90vh] md:h-screen w-full flex items-center justify-center overflow-hidden bg-neutral-950">
          {coverSrc && (
            <img
              src={coverSrc}
              alt={album.title}
              className="absolute inset-0 w-full h-full object-cover object-center opacity-80 filter brightness-90"
            />
          )}

          <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-black/30 to-black/40" />

          <div className="relative z-10 text-center text-white px-6 max-w-4xl space-y-4">
            <span className="inline-block text-xs uppercase tracking-[0.3em] font-light text-neutral-300">
              - {album.clientName || 'COLLECTION'} -
            </span>
            <h1 className="text-4xl sm:text-6xl md:text-7xl font-serif font-light tracking-tight leading-tight">
              {album.title}
            </h1>
            {album.description && (
              <p className="text-base sm:text-xl font-light text-neutral-200 italic font-serif">
                {album.description}
              </p>
            )}

            <div className="flex items-center justify-center gap-6 text-xs text-neutral-300 pt-4 font-light tracking-widest uppercase">
              {album.eventDate && (
                <div className="flex items-center gap-1.5">
                  <Calendar className="w-3.5 h-3.5" />
                  <span>{new Date(album.eventDate).toLocaleDateString('vi-VN')}</span>
                </div>
              )}
              {album.location && (
                <div className="flex items-center gap-1.5">
                  <MapPin className="w-3.5 h-3.5" />
                  <span>{album.location}</span>
                </div>
              )}
            </div>

            <div className="pt-8">
              <button
                onClick={() => {
                  document.getElementById('galleryContent')?.scrollIntoView({ behavior: 'smooth' });
                }}
                className="inline-flex items-center gap-2 px-6 py-3 rounded-full bg-white/20 hover:bg-white/30 backdrop-blur-md text-white text-xs tracking-widest uppercase transition-all duration-300 border border-white/30"
              >
                <span>Xem bộ ảnh</span>
                <ArrowDown className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </section>
      )}

      {/* TEMPLATE 3: MINIMALIST (Tối giản viền bo) */}
      {album && coverTemplate === 'minimal' && (
        <section className="w-[85%] max-w-[1400px] mx-auto pt-16 pb-12 text-center space-y-8">
          <div className="max-w-3xl mx-auto space-y-3">
            <span className="text-[11px] uppercase tracking-[0.3em] font-medium text-neutral-400">
              Bộ sưu tập hình ảnh
            </span>
            <h1 className="text-4xl sm:text-5xl md:text-6xl font-serif font-light text-neutral-900 tracking-tight">
              {album.title}
            </h1>
            {album.clientName && (
              <p className="text-lg text-neutral-500 font-serif italic">{album.clientName}</p>
            )}
            <div className="flex items-center justify-center gap-5 text-xs text-neutral-400 pt-2 font-light">
              {album.eventDate && <span>{new Date(album.eventDate).toLocaleDateString('vi-VN')}</span>}
              {album.location && <span>• {album.location}</span>}
            </div>
          </div>

          {coverSrc && (
            <div className="relative rounded-2xl overflow-hidden shadow-2xl max-h-[600px] bg-neutral-100 mx-auto max-w-4xl border border-neutral-200">
              <img src={coverSrc} alt={album.title} className="w-full h-full object-cover max-h-[600px]" />
            </div>
          )}
        </section>
      )}

      {/* TEMPLATE 4: SPLIT MAGAZINE (Chia 2 cột) */}
      {album && coverTemplate === 'split' && (
        <section className="w-[85%] max-w-[1400px] mx-auto py-16 grid grid-cols-1 md:grid-cols-12 gap-10 items-center">
          <div className="md:col-span-6 space-y-5">
            <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-neutral-100 text-[11px] uppercase tracking-widest text-neutral-600 font-medium border border-neutral-200">
              <Sparkles className="w-3 h-3" />
              Khách hàng: {album.clientName}
            </div>
            <h1 className="text-4xl sm:text-5xl md:text-6xl font-serif font-light text-neutral-900 tracking-tight leading-tight">
              {album.title}
            </h1>
            {album.description && (
              <p className="text-sm text-neutral-600 font-light leading-relaxed max-w-md">
                {album.description}
              </p>
            )}
            <div className="space-y-1.5 pt-4 text-xs text-neutral-500 font-light border-t border-neutral-200">
              {album.eventDate && <div><strong>Thời gian:</strong> {new Date(album.eventDate).toLocaleDateString('vi-VN')}</div>}
              {album.location && <div><strong>Địa điểm:</strong> {album.location}</div>}
            </div>
          </div>

          <div className="md:col-span-6">
            {coverSrc && (
              <div className="rounded-2xl overflow-hidden shadow-2xl aspect-[4/5] bg-neutral-100 border border-neutral-200">
                <img src={coverSrc} alt={album.title} className="w-full h-full object-cover" />
              </div>
            )}
          </div>
        </section>
      )}

      {/* ========================================================================= */}
      {/* 2. STICKY NAVBAR (Tên album, số lượng ảnh, bộ lọc Tất cả / Đã thích)       */}
      {/* ========================================================================= */}
      <nav className="sticky top-0 z-40 bg-[#fafafa]/95 backdrop-blur-md border-b border-neutral-200/80 transition-all duration-200">
        <div className="w-[85%] max-w-[1600px] mx-auto py-3.5 flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div>
              <h2 className="text-base sm:text-lg font-serif font-medium text-neutral-900 truncate">
                {album?.title || data.item?.filename || 'Bộ sưu tập ảnh'}
              </h2>
              <p className="text-[11px] text-neutral-500 font-light">
                {rawItems.length} tác phẩm · {totalLikedCount} đã thích
              </p>
            </div>
          </div>

          {/* Bộ lọc Tất cả / Đã thích */}
          <div className="flex items-center bg-neutral-100 p-1 rounded-full border border-neutral-200 text-xs">
            <button
              onClick={() => setFilterMode('ALL')}
              className={`px-3.5 py-1 rounded-full text-xs font-medium transition-all ${
                filterMode === 'ALL'
                  ? 'bg-neutral-900 text-white shadow-sm'
                  : 'text-neutral-600 hover:text-neutral-900'
              }`}
            >
              Tất cả ({rawItems.length})
            </button>
            <button
              onClick={() => setFilterMode('FAVORITES')}
              className={`px-3.5 py-1 rounded-full text-xs font-medium flex items-center gap-1.5 transition-all ${
                filterMode === 'FAVORITES'
                  ? 'bg-neutral-900 text-white shadow-sm'
                  : 'text-neutral-600 hover:text-neutral-900'
              }`}
            >
              <Heart className="w-3.5 h-3.5" />
              <span>Đã thích ({totalLikedCount})</span>
            </button>
          </div>
        </div>
      </nav>

      {/* ========================================================================= */}
      {/* 3. MASONRY GALLERY GRID (Giữ nguyên tỉ lệ tự nhiên 100%, không bị crop)  */}
      {/* ========================================================================= */}
      <main id="galleryContent" className="w-[85%] max-w-[1600px] mx-auto py-10">
        {filteredItems.length === 0 ? (
          <div className="text-center py-20 text-neutral-400">
            <Heart className="w-12 h-12 mx-auto mb-3 text-neutral-300" />
            <p className="text-base font-serif">Chưa có ảnh nào được thích trong bộ sưu tập này.</p>
          </div>
        ) : (
          <div className="columns-1 sm:columns-2 md:columns-3 lg:columns-4 gap-4 space-y-4">
            {filteredItems.map((item, index) => {
              const mediaId = item.id || item.media?.id || '';
              const filename = item.filename || item.media?.filename || `Ảnh ${index + 1}`;
              const isLiked = !!favoritesMap[mediaId];
              const thumbSrc = resolveMediaUrl(
                item.previewUrl ||
                  item.thumbnailUrl ||
                  item.originalUrl ||
                  item.urls?.preview ||
                  item.urls?.thumbnail ||
                  item.media?.previewUrl ||
                  item.media?.thumbnailUrl
              );

              return (
                <div
                  key={mediaId || index}
                  onClick={() => setActiveLightboxIndex(index)}
                  className="break-inside-avoid relative bg-neutral-200 overflow-hidden cursor-pointer rounded-lg shadow-sm hover:shadow-lg transition-all duration-300 group mb-4"
                >
                  {/* Ảnh với tỉ lệ tự nhiên (w-full h-auto) */}
                  <img
                    src={thumbSrc}
                    alt={filename}
                    loading="lazy"
                    className="w-full h-auto object-cover block transition-transform duration-500 group-hover:scale-[1.02]"
                  />

                  {/* Nút thả tim góc trên phải */}
                  {mediaId && (
                    <button
                      type="button"
                      onClick={(e) => handleToggleFavorite(mediaId, e)}
                      title={isLiked ? 'Đã thích' : 'Thích'}
                      className="absolute top-3 right-3 p-1 text-white heart-interactive photo-heart-shadow focus:outline-none"
                    >
                      {isLiked ? (
                        <Heart className="w-6 h-6 fill-white text-black stroke-[1.5]" />
                      ) : (
                        <Heart className="w-6 h-6 text-white stroke-[2]" />
                      )}
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </main>

      {/* ========================================================================= */}
      {/* 4. FULLSCREEN LIGHTBOX MODAL (Xem ảnh gốc toàn màn hình, chuyển ảnh)      */}
      {/* ========================================================================= */}
      {activeLightboxIndex !== null && filteredItems[activeLightboxIndex] && (
        <div className="fixed inset-0 z-50 bg-black/95 flex flex-col justify-between text-white backdrop-blur-md lightbox-animate select-none">
          {/* Top Bar */}
          <div className="px-6 py-4 flex items-center justify-between border-b border-neutral-800">
            <span className="text-xs text-neutral-400 font-mono">
              {activeLightboxIndex + 1} / {filteredItems.length}
            </span>

            <div className="flex items-center gap-4">
              {/* Thả tim ngay trong Lightbox */}
              {(() => {
                const currentItem = filteredItems[activeLightboxIndex];
                const currentMediaId = currentItem.id || currentItem.media?.id || '';
                return (
                  <button
                    type="button"
                    onClick={() => handleToggleFavorite(currentMediaId)}
                    className="flex items-center gap-1.5 text-xs text-neutral-300 hover:text-white transition-colors"
                  >
                    {favoritesMap[currentMediaId] ? (
                      <Heart className="w-5 h-5 fill-white text-white" />
                    ) : (
                      <Heart className="w-5 h-5 text-white" />
                    )}
                    <span>{favoritesMap[currentMediaId] ? 'Đã thích' : 'Thích'}</span>
                  </button>
                );
              })()}

              {/* Tải ảnh gốc */}
              <a
                href={resolveMediaUrl(
                  filteredItems[activeLightboxIndex].originalUrl ||
                    filteredItems[activeLightboxIndex].previewUrl ||
                    filteredItems[activeLightboxIndex].urls?.original ||
                    filteredItems[activeLightboxIndex].media?.previewUrl
                )}
                target="_blank"
                rel="noreferrer"
                download
                className="p-2 rounded-full hover:bg-neutral-800 text-neutral-300 hover:text-white transition-colors"
                title="Tải ảnh gốc"
              >
                <Download className="w-5 h-5" />
              </a>

              {/* Đóng Lightbox */}
              <button
                type="button"
                onClick={() => setActiveLightboxIndex(null)}
                className="p-2 rounded-full hover:bg-neutral-800 text-neutral-300 hover:text-white transition-colors"
              >
                <X className="w-6 h-6" />
              </button>
            </div>
          </div>

          {/* Vùng hiển thị ảnh chính */}
          <div className="flex-1 relative flex items-center justify-center p-4 min-h-0">
            <img
              src={resolveMediaUrl(
                filteredItems[activeLightboxIndex].originalUrl ||
                  filteredItems[activeLightboxIndex].previewUrl ||
                  filteredItems[activeLightboxIndex].urls?.original ||
                  filteredItems[activeLightboxIndex].media?.previewUrl
              )}
              alt={
                filteredItems[activeLightboxIndex].filename ||
                filteredItems[activeLightboxIndex].media?.filename ||
                'Photo'
              }
              className="max-w-full max-h-full object-contain select-none"
            />

            {/* Phím mũi tên lùi ảnh */}
            <button
              type="button"
              onClick={() =>
                setActiveLightboxIndex((prev) =>
                  prev !== null && prev > 0 ? prev - 1 : filteredItems.length - 1
                )}
              className="absolute left-4 p-3 rounded-full bg-black/40 hover:bg-black/80 text-white backdrop-blur-sm transition-all"
            >
              <ChevronLeft className="w-6 h-6" />
            </button>

            {/* Phím mũi tên tiến ảnh */}
            <button
              type="button"
              onClick={() =>
                setActiveLightboxIndex((prev) =>
                  prev !== null && prev < filteredItems.length - 1 ? prev + 1 : 0
                )}
              className="absolute right-4 p-3 rounded-full bg-black/40 hover:bg-black/80 text-white backdrop-blur-sm transition-all"
            >
              <ChevronRight className="w-6 h-6" />
            </button>
          </div>

          {/* Bottom Bar: Tên file */}
          <div className="py-3 px-6 text-center text-xs text-neutral-400 border-t border-neutral-800 truncate">
            {filteredItems[activeLightboxIndex].filename ||
              filteredItems[activeLightboxIndex].media?.filename}
          </div>
        </div>
      )}

      {/* Footer */}
      <footer className="py-12 border-t border-neutral-200/80 text-center text-xs text-neutral-400 font-light">
        <p>© {new Date().getFullYear()} Gallery Platform — All Rights Reserved.</p>
      </footer>
    </div>
  );
};
