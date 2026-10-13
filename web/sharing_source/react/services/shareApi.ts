import axios from 'axios';
import { ShareGalleryData, ToggleFavoriteResult } from '../types/share.types';

// Cấu hình URL mặc định có thể lấy từ biến môi trường của dự án đích hoặc fallback
export const DEFAULT_API_BASE_URL =
  (typeof process !== 'undefined' && process.env?.REACT_APP_API_BASE_URL) ||
  (typeof process !== 'undefined' && process.env?.NEXT_PUBLIC_API_BASE_URL) ||
  (typeof import.meta !== 'undefined' && (import.meta as any).env?.VITE_API_BASE_URL) ||
  '';

/**
 * Xử lý và chuẩn hóa URL media (MinIO, S3 hoặc internal host) để hiển thị mượt mà trên browser
 */
export function resolveMediaUrl(rawUrl?: string | null, customOrigin?: string): string {
  if (!rawUrl) return '';
  try {
    const origin = customOrigin || (typeof window !== 'undefined' ? window.location.origin : '');
    const url = new URL(rawUrl, origin || 'http://localhost');

    // Chuyển hostname nội bộ docker hoặc port S3 sang domain public nếu cần
    if (url.hostname === 'minio' || url.port === '9000' || url.port === '3000') {
      if (origin) {
        return `${origin}${url.pathname}${url.search}`;
      }
    }
    return url.href;
  } catch {
    return rawUrl;
  }
}

/**
 * Lấy dữ liệu công khai của bộ sưu tập hoặc ảnh chia sẻ qua token hoặc file JSON
 */
export async function fetchSharedContent(
  token: string,
  apiBaseUrl: string = DEFAULT_API_BASE_URL,
  jsonUrl?: string
): Promise<ShareGalleryData> {
  if (jsonUrl) {
    const response = await axios.get(jsonUrl);
    return response.data?.data ? response.data.data : response.data;
  }

  // 1. Thử gọi API backend (nếu có API)
  try {
    const url = `${apiBaseUrl}/api/v1/shares/${token}`;
    const response = await axios.get(url, {
      headers: {
        Accept: 'application/json',
      },
    });
    return response.data?.data ? response.data.data : response.data;
  } catch (apiErr) {
    // 2. Fallback: Nếu không có API backend, đọc trực tiếp file JSON manifest
    const fallbackCandidates = [
      `/s/${token}/f/album_manifest.json?download=1`,
      `/s/${token}/f/album.json?download=1`,
    ];
    for (const cand of fallbackCandidates) {
      try {
        const response = await axios.get(cand);
        if (response.data) {
          return response.data?.data ? response.data.data : response.data;
        }
      } catch {
        // thử ứng viên tiếp theo
      }
    }
    throw apiErr;
  }
}

/**
 * Khách thả tim / bỏ thích một ảnh trong bộ sưu tập công khai
 */
export async function toggleMediaFavorite(
  token: string,
  mediaId: string,
  apiBaseUrl: string = DEFAULT_API_BASE_URL
): Promise<ToggleFavoriteResult> {
  const url = `${apiBaseUrl}/api/v1/shares/${token}/favorites/${mediaId}`;
  const response = await axios.post(url);

  return response.data?.data ? response.data.data : response.data;
}
