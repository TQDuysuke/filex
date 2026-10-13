/**
 * Các kiểu template ảnh bìa (Cover) được hỗ trợ
 * - 'haduanh': Bìa toàn màn hình cao cấp chuẩn phong cách Haduanh (Line mạ thanh mảnh, text in hoa đậm, nút View Gallery góc phải)
 * - 'editorial': Bìa căn giữa nghệ thuật phong cách tạp chí cổ điển (Font Serif)
 * - 'minimal': Bìa tối giản dạng khung bo tròn ở giữa
 * - 'split': Bìa chia 2 cột (Cột thông tin bên trái, cột ảnh dọc bên phải)
 * - 'direct': Không hiển thị hero cover, hiển thị thẳng vào gallery
 */
export type CoverTemplate = 'haduanh' | 'editorial' | 'minimal' | 'split' | 'direct';

export interface ShareGalleryItem {
  id: string;
  position?: number;
  filename: string;
  type: 'IMAGE' | 'VIDEO' | string;
  mimeType?: string;
  size?: number;
  width?: number | null;
  height?: number | null;
  duration?: number | null;
  dateTaken?: string | null;
  thumbnailUrl?: string | null;
  previewUrl?: string | null;
  originalUrl?: string | null;
  urls?: {
    original?: string | null;
    preview?: string | null;
    thumbnail?: string | null;
  };
  media?: {
    id: string;
    filename: string;
    type: string;
    width?: number | null;
    height?: number | null;
    urls?: {
      original?: string | null;
      preview?: string | null;
      thumbnail?: string | null;
    };
    thumbnailUrl?: string | null;
    previewUrl?: string | null;
  };
  isFavorite?: boolean;
  favoriteCount?: number;
}

export interface ShareAlbumData {
  id: string;
  title: string;
  description?: string | null;
  clientName?: string | null;
  eventDate?: string | null;
  location?: string | null;
  albumType?: string;
  coverMediaId?: string | null;
  coverStyle?: CoverTemplate | string | null;
  coverUrl?: string | null;
  itemCount?: number;
  items?: ShareGalleryItem[];
}

export interface ShareInfo {
  token: string;
  expiresAt?: string | null;
  permissions?: string[];
}

export interface ShareGalleryData {
  type?: 'ALBUM' | 'MEDIA';
  share?: ShareInfo;
  permissions?: string[];
  album?: ShareAlbumData | null;
  item?: ShareGalleryItem | null;
  media?: {
    id: string;
    filename: string;
    urls?: {
      original?: string | null;
      preview?: string | null;
    };
  } | null;
  totalFavorites?: number;
}

export interface ToggleFavoriteResult {
  favorited: boolean;
  totalFavorites: number;
}
