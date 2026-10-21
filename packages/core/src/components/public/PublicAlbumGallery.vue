<script setup lang="ts">
/**
 * PublicAlbumGallery — Luxury Photo Album Showcase for public share links.
 * Exact implementation matching `web/sharing_source` (Vanilla & React templates):
 * - Full viewport edge-to-edge luxury editorial layout
 * - 4 Cover templates: Haduanh Luxury Hero, Editorial Serif, Minimal Rounded, Split Magazine
 * - Sticky navigation bar with title, metadata, and filter pills (All / Favorites)
 * - Multi-column masonry photo grid with natural aspect ratios (no cropping)
 * - Heart favorites with localStorage persistence
 * - Full-screen lightbox with keyboard navigation (Esc, ArrowLeft, ArrowRight)
 * - Toast notification and quick toggle back to standard file list view
 */
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue';
import type { AlbumItem, AlbumData } from '../../types/Public';
import { fetchAndParseExif, type ExifMetadata } from '../../lib/exif';
import IOSAlbumSaveModal from './IOSAlbumSaveModal.vue';
import PeopleSelector, { type ClusteredFace } from './PeopleSelector.vue';
import QRCodeModal from './QRCodeModal.vue';
import MultiDownloadWidget from './MultiDownloadWidget.vue';

const props = defineProps<{
  album: AlbumData;
  token?: string;
  zipUrl?: string;
  locale?: string;
}>();

const emit = defineEmits<{
  (e: 'switchToFileView'): void;
}>();

// iOS Detection & Download Mechanism
function detectIsIOS(): boolean {
  if (typeof window === 'undefined' || typeof navigator === 'undefined') return false;
  const ua = navigator.userAgent || '';
  const isAppleMobile = /iPhone|iPad|iPod/i.test(ua);
  const isIPadOS =
    (navigator.platform === 'MacIntel' || ua.includes('Macintosh')) &&
    Boolean(navigator.maxTouchPoints && navigator.maxTouchPoints > 1);
  return isAppleMobile || isIPadOS;
}

// State
const filterMode = ref<'ALL' | 'FAVORITES' | 'AI_MATCHES'>('ALL');
const activeLightboxIndex = ref<number | null>(null);
const favoritesMap = ref<Record<string, boolean>>({});
const toastMsg = ref('');
let toastTimer: ReturnType<typeof setTimeout> | undefined;
const isIOS = ref(false);
const isDownloading = ref(false);
const readyFiles = ref<Record<string, File>>({});
const isSelectMode = ref(false);
const selectedItemIds = ref<Set<string>>(new Set());
const showIosSaveModal = ref(false);
const showQrModal = ref(false);
const showMultiDl = ref(false);
const showInfo = ref(false);
const showMoreMenu = ref(false);
const copiedLink = ref(false);
const isDark = ref(false);
const viewMode = ref<'standard' | 'large' | 'compact'>('standard');
const aspectMode = ref<'natural' | 'square'>('natural');
const cameraInputRef = ref<HTMLInputElement | null>(null);
const aiSearchStep = ref('');

function applyThemeClasses(dark: boolean) {
  if (typeof document === 'undefined') return;
  const html = document.documentElement;
  const body = document.body;
  if (dark) {
    html.classList.add('dark');
    html.classList.add('fe--theme-dark');
    html.classList.remove('fe--theme-light');
    body.classList.add('dark');
    body.classList.add('fe--theme-dark');
    body.classList.remove('fe--theme-light');
    html.style.colorScheme = 'dark';
    body.style.backgroundColor = '#0E0E0D';
    body.style.color = '#F5F5F4';
  } else {
    html.classList.remove('dark');
    html.classList.remove('fe--theme-dark');
    html.classList.add('fe--theme-light');
    body.classList.remove('dark');
    body.classList.remove('fe--theme-dark');
    body.classList.add('fe--theme-light');
    html.style.colorScheme = 'light';
    body.style.backgroundColor = '#FFFDF8';
    body.style.color = '#171717';
  }
}

function toggleDarkMode() {
  isDark.value = !isDark.value;
  try {
    localStorage.setItem('filex.theme', isDark.value ? 'dark' : 'light');
    localStorage.setItem('filex_theme_preference', isDark.value ? 'dark' : 'light');
    localStorage.setItem('theme', isDark.value ? 'dark' : 'light');
  } catch {}
  applyThemeClasses(isDark.value);
}

function triggerCamera() {
  cameraInputRef.value?.click();
}

function triggerGalleryUpload() {
  selfieInputRef.value?.click();
}

function onSelfieFileChosen(e: Event) {
  const target = e.target as HTMLInputElement;
  if (target.files && target.files[0]) {
    setSelfieFile(target.files[0]);
  }
}

function handleCopyAlbumLink() {
  if (typeof window !== 'undefined') {
    void navigator.clipboard.writeText(window.location.href);
    copiedLink.value = true;
    showToast('Album link copied to clipboard!');
    setTimeout(() => {
      copiedLink.value = false;
    }, 2500);
  }
}

function handleDownloadFavorites() {
  const favoriteItems = props.album.items.filter((item) => !!favoritesMap.value[item.id]);
  if (favoriteItems.length === 0) {
    showToast('No photos favorited yet. Add some photos to favorites first.');
    return;
  }
  selectedItemIds.value = new Set(favoriteItems.map((i) => i.id));
  handleBatchAction();
}

function onClickOutsideMenu(e: MouseEvent) {
  if (showMoreMenu.value) {
    const target = e.target as HTMLElement;
    if (!target.closest('.nav-more-wrapper')) {
      showMoreMenu.value = false;
    }
  }
}

// Clustered Faces State
const clusteredFaces = ref<ClusteredFace[]>([]);
const selectedFaceId = ref<number | string | null>(null);
const loadingFaces = ref(false);

const activeFaceCropUrl = computed(() => {
  if (selectedFaceId.value === null) return '';
  const found = clusteredFaces.value.find((f) => String(f.face_id) === String(selectedFaceId.value));
  return found?.crop_url || '';
});

const token = computed<string>(() => {
  if (props.token) return props.token;
  if (typeof window !== 'undefined') {
    const match = window.location.pathname.match(/\/s\/([a-zA-Z0-9_\-]+)/);
    if (match) return match[1];
  }
  return '';
});
const currentUrl = computed(() => (typeof window !== 'undefined' ? window.location.href : ''));

function safePhotoUrl(rawUrl: string | undefined | null, filename?: string, isDownload = false): string {
  const currentToken = token.value;
  if (rawUrl && rawUrl.includes('/s/')) {
    const match = rawUrl.match(/\/s\/[a-zA-Z0-9_\-]+\/f\/(.+)$/);
    if (match && currentToken) {
      const cleanPath = match[1].replace(/\?.*$/, '');
      const query = isDownload ? '?download=1' : '';
      return `/s/${currentToken}/f/${cleanPath}${query}`;
    }
  }
  if (rawUrl && (rawUrl.startsWith('https://') || rawUrl.startsWith('http://')) && !rawUrl.includes('/s/')) {
    return rawUrl;
  }
  if (filename && currentToken) {
    const encoded = filename.split('/').map(encodeURIComponent).join('/');
    const query = isDownload ? '?download=1' : '';
    return `/s/${currentToken}/f/${encoded}${query}`;
  }
  return rawUrl || '';
}

// AI Face Search State
const showAiModal = ref(false);
const selfieInputRef = ref<HTMLInputElement | null>(null);
const aiSelfieFile = ref<File | null>(null);
const aiSelfiePreview = ref<string>('');
const aiSearching = ref(false);
const aiSearchError = ref('');
const aiMatchesMap = ref<Record<string, { similarity: number; confidence: number }>>({});
export interface AifsAlbumStatus {
  album_id?: number;
  title?: string;
  status: string;
  total_photos: number;
  indexed_photos: number;
  detected_faces?: number;
  total_faces?: number;
  clusters_count?: number;
  percent_complete?: number;
}
const aifsStatus = ref<AifsAlbumStatus | null>(null);
let statusPollTimer: ReturnType<typeof setInterval> | undefined;

const aiMatchCount = computed(() => Object.keys(aiMatchesMap.value).length);

const selectedPhotos = computed(() => {
  return props.album.items.filter((item) => selectedItemIds.value.has(item.id));
});

function toggleSelectMode() {
  isSelectMode.value = !isSelectMode.value;
  if (!isSelectMode.value) {
    selectedItemIds.value = new Set();
  }
}

function toggleSelectItem(id: string, e?: Event) {
  if (e) e.stopPropagation();
  const next = new Set(selectedItemIds.value);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  selectedItemIds.value = next;
}

function selectAllItems() {
  if (selectedItemIds.value.size === displayItems.value.length) {
    selectedItemIds.value = new Set();
  } else {
    selectedItemIds.value = new Set(displayItems.value.map((i) => i.id));
  }
}

function favoriteSelectedItems() {
  if (selectedItemIds.value.size === 0) return;
  const next = { ...favoritesMap.value };
  selectedItemIds.value.forEach((id) => {
    next[id] = true;
  });
  favoritesMap.value = next;
  saveFavorites();
  showToast(`Added ${selectedItemIds.value.size} photos to favorites.`);
}

function buildCSVFromItems(items: AlbumItem[], filenamePrefix: string) {
  if (items.length === 0) return;

  const escapeCSV = (val: any): string => {
    if (val === null || val === undefined) return '""';
    const str = String(val).replace(/"/g, '""');
    return `"${str}"`;
  };

  const headers = [
    'STT',
    'Filename',
    'Size',
    'Size (Bytes)',
    'Dimensions',
    'Width (px)',
    'Height (px)',
    'Megapixels',
    'Camera',
    'Lens',
    'Aperture',
    'Shutter Speed',
    'ISO',
    'Focal Length',
    'Taken At',
    'Preview URL',
    'Original URL',
  ];

  const rows = items.map((item, index) => {
    const mp = getMegapixels(item.width, item.height);
    const sizeFmt = formatFileSize(item.size);
    const dimensions = item.width && item.height ? `${item.width}x${item.height}` : '';
    const dateTaken = item.taken_at ? new Date(item.taken_at).toLocaleString('vi-VN') : '';
    const previewUrl = item.previewUrl || item.thumbnailUrl || '';
    const origUrl = item.originalUrl || previewUrl;

    return [
      escapeCSV(index + 1),
      escapeCSV(item.filename),
      escapeCSV(sizeFmt),
      escapeCSV(item.size ?? ''),
      escapeCSV(dimensions),
      escapeCSV(item.width ?? ''),
      escapeCSV(item.height ?? ''),
      escapeCSV(mp),
      escapeCSV(item.camera ?? ''),
      escapeCSV(item.lens ?? ''),
      escapeCSV(item.aperture ?? ''),
      escapeCSV(item.shutter_speed ?? ''),
      escapeCSV(item.iso ?? ''),
      escapeCSV(item.focal_length ?? ''),
      escapeCSV(dateTaken),
      escapeCSV(previewUrl),
      escapeCSV(origUrl),
    ].join(',');
  });

  const csvContent = '\uFEFF' + [headers.join(','), ...rows].join('\r\n');
  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  const safeTitle = (props.album.title || 'Album').replace(/[/\\?%*:|"<>]/g, '_');
  const dateStr = new Date().toISOString().slice(0, 10);
  a.href = url;
  a.download = `${safeTitle}_${filenamePrefix}_${dateStr}.csv`;
  document.body.appendChild(a);
  a.click();
  setTimeout(() => {
    if (document.body.contains(a)) document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }, 1000);
}

function exportFavoritesToCSV() {
  const favoriteItems = props.album.items.filter((item) => !!favoritesMap.value[item.id]);
  if (favoriteItems.length === 0) {
    showToast('No photos favorited yet. Add some photos to favorites first.');
    return;
  }
  buildCSVFromItems(favoriteItems, 'Favorites');
  showToast(`Exported ${favoriteItems.length} favorited photos to CSV!`);
}

function exportSelectedToCSV() {
  const selectedItems = props.album.items.filter((item) => selectedItemIds.value.has(item.id));
  if (selectedItems.length === 0) {
    showToast('No photos selected to export!');
    return;
  }
  buildCSVFromItems(selectedItems, 'Selected');
  showToast(`Exported ${selectedItems.length} selected photos to CSV!`);
}

// Touch Swipe navigation for mobile
const touchStartX = ref<number | null>(null);
const touchStartY = ref<number | null>(null);

function onTouchStart(e: TouchEvent) {
  touchStartX.value = e.touches[0].clientX;
  touchStartY.value = e.touches[0].clientY;
}

function onTouchEnd(e: TouchEvent) {
  if (touchStartX.value === null || touchStartY.value === null) return;
  const deltaX = e.changedTouches[0].clientX - touchStartX.value;
  const deltaY = e.changedTouches[0].clientY - touchStartY.value;

  if (Math.abs(deltaX) > 45 && Math.abs(deltaX) > Math.abs(deltaY)) {
    if (deltaX > 0) prevLightbox();
    else nextLightbox();
  } else if (deltaY > 90 && Math.abs(deltaY) > Math.abs(deltaX)) {
    closeLightbox();
  }
  touchStartX.value = null;
  touchStartY.value = null;
}

async function preloadPhotoFile(item: AlbumItem): Promise<File | null> {
  if (readyFiles.value[item.id]) return readyFiles.value[item.id];
  const url = safePhotoUrl(item.originalUrl || item.previewUrl || item.thumbnailUrl, item.filename);
  if (!url) return null;
  try {
    const res = await fetch(url);
    if (!res.ok) return null;
    const blob = await res.blob();
    const ext = blob.type.includes('png') ? '.png' : '.jpg';
    const cleanFilename = item.filename && item.filename.includes('.')
      ? item.filename
      : `${item.filename || 'photo'}${ext}`;
    const mimeType = blob.type.startsWith('image/') ? blob.type : (ext === '.png' ? 'image/png' : 'image/jpeg');
    const file = new File([blob], cleanFilename, { type: mimeType });
    readyFiles.value[item.id] = file;
    return file;
  } catch {
    return null;
  }
}

function triggerDirectDownload(photo: AlbumItem) {
  const safeFilename = photo.filename || `photo_${photo.id}.jpg`;
  const url = safePhotoUrl(photo.originalUrl || photo.previewUrl || photo.thumbnailUrl, photo.filename, true);
  if (!url) return;
  const a = document.createElement('a');
  a.href = url;
  a.download = safeFilename;
  document.body.appendChild(a);
  a.click();
  setTimeout(() => {
    if (document.body.contains(a)) document.body.removeChild(a);
  }, 1000);
}

function copyPhotoLink(item: AlbumItem) {
  const relUrl = safePhotoUrl(item.originalUrl || item.previewUrl, item.filename);
  const url = typeof window !== 'undefined' ? `${window.location.origin}${relUrl}` : relUrl;
  void navigator.clipboard.writeText(url);
  showToast('Link copied');
}

function handleBatchAction() {
  if (isIOS.value) {
    showIosSaveModal.value = true;
  } else {
    showMultiDl.value = true;
  }
}

function getMegapixels(w?: number | null, h?: number | null): string {
  if (!w || !h) return '';
  const mp = (w * h) / 1000000;
  return `${mp.toFixed(1)} MP`;
}

function formatFileSize(bytes?: number | null): string {
  if (!bytes || bytes <= 0) return '';
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
}

// iOS Native Share (Save to Photos) vs Desktop Direct Download
async function downloadPhoto(item: AlbumItem, e?: Event) {
  if (e) e.stopPropagation();
  if (isDownloading.value) return;
  const url = safePhotoUrl(item.originalUrl || item.previewUrl || item.thumbnailUrl, item.filename, true);
  if (!url) return;

  if (isIOS.value && typeof navigator !== 'undefined' && navigator.share) {
    const preloaded = readyFiles.value[item.id];
    if (preloaded) {
      try {
        await navigator.share({
          files: [preloaded],
        });
        showToast('Saved to Photos');
        return;
      } catch (err: any) {
        if (err.name === 'AbortError') return;
      }
    }

    try {
      isDownloading.value = true;
      const file = await preloadPhotoFile(item);
      if (file) {
        await navigator.share({
          files: [file],
        });
        showToast('Saved to Photos');
        return;
      }
    } catch (err: any) {
      if (err.name === 'AbortError') return;
      console.warn('iOS Web Share failed, falling back to direct download:', err);
    } finally {
      isDownloading.value = false;
    }
  }

  // Standard direct file download
  try {
    isDownloading.value = true;
    triggerDirectDownload(item);
    showToast('Download started');
  } finally {
    isDownloading.value = false;
  }
}

function triggerSelfieInput() {
  selfieInputRef.value?.click();
}

function onSelfieChange(e: Event) {
  const target = e.target as HTMLInputElement;
  if (target.files && target.files[0]) {
    setSelfieFile(target.files[0]);
  }
}

function onSelfieDrop(e: DragEvent) {
  if (e.dataTransfer?.files && e.dataTransfer.files[0]) {
    setSelfieFile(e.dataTransfer.files[0]);
  }
}

function setSelfieFile(file: File) {
  aiSelfieFile.value = file;
  aiSearchError.value = '';
  const reader = new FileReader();
  reader.onload = (e) => {
    aiSelfiePreview.value = String(e.target?.result || '');
  };
  reader.readAsDataURL(file);
}

function clearSelfie() {
  aiSelfieFile.value = null;
  aiSelfiePreview.value = '';
  aiSearchError.value = '';
  aiSearchStep.value = '';
  if (selfieInputRef.value) selfieInputRef.value.value = '';
  if (cameraInputRef.value) cameraInputRef.value.value = '';
}

async function runSelfieSearch() {
  if (!aiSelfieFile.value || !token.value) return;
  aiSearching.value = true;
  aiSearchError.value = '';
  aiSearchStep.value = 'Uploading selfie photo...';
  try {
    const formData = new FormData();
    formData.append('file', aiSelfieFile.value);
    aiSearchStep.value = 'AI is scanning and matching faces...';
    const res = await fetch(`/api/public/s/${token.value}/face-search`, {
      method: 'POST',
      body: formData,
    });
    if (!res.ok) {
      const errJson = await res.json().catch(() => null);
      throw new Error(errJson?.error || `Face search failed: HTTP ${res.status}`);
    }
    const data = await res.json();
    const map: Record<string, { similarity: number; confidence: number }> = {};
    if (Array.isArray(data.matches)) {
      for (const m of data.matches) {
        if (m.filename) {
          map[m.filename] = {
            similarity: m.similarity ?? 0.85,
            confidence: m.confidence ?? 0.9,
          };
        }
      }
    }
    aiMatchesMap.value = map;
    const count = Object.keys(map).length;
    if (count > 0) {
      filterMode.value = 'AI_MATCHES';
      showAiModal.value = false;
      showToast(`Found ${count} photos of you!`);
      scrollToGallery();
    } else {
      aiSearchError.value = 'No matching faces found in this album.';
    }
  } catch (err: any) {
    aiSearchError.value = err?.message || 'Error occurred during face search.';
  } finally {
    aiSearching.value = false;
    aiSearchStep.value = '';
  }
}

async function fetchClusteredFaces() {
  if (!token.value) return;
  loadingFaces.value = true;
  try {
    const res = await fetch(`/api/public/s/${token.value}/faces`);
    if (res.ok) {
      const data = await res.json();
      clusteredFaces.value = Array.isArray(data.faces) ? data.faces : [];
    }
  } catch (err) {
    console.warn('Failed to fetch clustered faces:', err);
  } finally {
    loadingFaces.value = false;
  }
}

async function handleSelectFace(faceId: number | string | null) {
  if (selectedFaceId.value === faceId || faceId === null) {
    clearFaceFilter();
    return;
  }
  selectedFaceId.value = faceId;
  aiSearching.value = true;
  try {
    const res = await fetch(`/api/public/s/${token.value}/face-search?face_id=${faceId}`, {
      method: 'POST',
    });
    if (res.ok) {
      const data = await res.json();
      const map: Record<string, { similarity: number; confidence: number }> = {};
      if (Array.isArray(data.matches)) {
        for (const m of data.matches) {
          if (m.filename) {
            map[m.filename] = {
              similarity: m.similarity ?? 0.85,
              confidence: m.confidence ?? 0.9,
            };
          }
        }
      }
      aiMatchesMap.value = map;
      filterMode.value = 'AI_MATCHES';
      showToast(`Filtered ${Object.keys(map).length} photos for person`);
      scrollToGallery();
    }
  } catch (err) {
    console.warn('Face search by ID failed:', err);
  } finally {
    aiSearching.value = false;
  }
}

function clearFaceFilter() {
  selectedFaceId.value = null;
  filterMode.value = 'ALL';
  aiMatchesMap.value = {};
}

function handleSelectFaceFromLightbox(faceId: number | string) {
  closeLightbox();
  void handleSelectFace(faceId);
}

async function fetchAifsStatus() {
  if (!token.value) return;
  try {
    const res = await fetch(`/api/public/s/${token.value}/aifs/status`);
    if (res.ok) {
      const prevStatus = aifsStatus.value?.status;
      const data = await res.json();
      aifsStatus.value = data;
      if ((prevStatus === 'indexing' || prevStatus === 'pending') && (data.status === 'ready' || data.status === 'indexed')) {
        showToast('AI face indexing complete');
        void fetchClusteredFaces();
      } else if ((data.status === 'ready' || data.status === 'indexed') && clusteredFaces.value.length === 0) {
        void fetchClusteredFaces();
      }
    }
  } catch {
    // ignore
  }
}

// Storage key for favorites
const storageKey = computed(() => `filex_album_fav_${token.value || props.album.id || 'default'}`);

function loadFavorites() {
  try {
    const raw = localStorage.getItem(storageKey.value);
    if (raw) {
      favoritesMap.value = JSON.parse(raw);
      return;
    }
  } catch {
    // ignore
  }
  const map: Record<string, boolean> = {};
  props.album.items.forEach((item) => {
    if (item.isFavorite) map[item.id] = true;
  });
  favoritesMap.value = map;
}

function saveFavorites() {
  try {
    localStorage.setItem(storageKey.value, JSON.stringify(favoritesMap.value));
  } catch {
    // ignore
  }
}

function showToast(msg: string) {
  toastMsg.value = msg;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toastMsg.value = '';
  }, 2200);
}

const displayItems = computed(() => {
  if (filterMode.value === 'FAVORITES') {
    return props.album.items.filter((item) => !!favoritesMap.value[item.id]);
  }
  if (filterMode.value === 'AI_MATCHES') {
    return props.album.items
      .filter((item) => !!aiMatchesMap.value[item.filename])
      .sort((a, b) => {
        const simA = aiMatchesMap.value[a.filename]?.similarity || 0;
        const simB = aiMatchesMap.value[b.filename]?.similarity || 0;
        return simB - simA;
      });
  }
  return props.album.items;
});

const totalFavorites = computed(() => {
  return Object.values(favoritesMap.value).filter(Boolean).length;
});

const coverImageSrc = computed(() => {
  if (props.album.coverUrl) return safePhotoUrl(props.album.coverUrl);
  const first = props.album.items[0];
  return first ? safePhotoUrl(first.originalUrl || first.previewUrl || first.thumbnailUrl, first.filename) : '';
});

const coverStyle = computed(() => {
  const s = props.album.coverStyle?.toLowerCase() || 'haduanh';
  if (['haduanh', 'editorial', 'minimal', 'split'].includes(s)) return s;
  return 'haduanh';
});

function toggleFavorite(itemId: string, e?: Event) {
  if (e) e.stopPropagation();
  const current = !!favoritesMap.value[itemId];
  favoritesMap.value = {
    ...favoritesMap.value,
    [itemId]: !current,
  };
  saveFavorites();
  showToast(!current ? 'Added to favorites' : 'Removed from favorites');
}

function scrollToGallery() {
  const el = document.getElementById('feAlbumNav');
  if (el) el.scrollIntoView({ behavior: 'smooth' });
}

// Lightbox navigation
function openLightbox(index: number) {
  activeLightboxIndex.value = index;
  isUiHidden.value = false;
}

function closeLightbox() {
  activeLightboxIndex.value = null;
}

function prevLightbox() {
  if (activeLightboxIndex.value === null) return;
  const len = displayItems.value.length;
  if (len === 0) return;
  activeLightboxIndex.value = (activeLightboxIndex.value - 1 + len) % len;
}

function nextLightbox() {
  if (activeLightboxIndex.value === null) return;
  const len = displayItems.value.length;
  if (len === 0) return;
  activeLightboxIndex.value = (activeLightboxIndex.value + 1) % len;
}

const exifCache = ref<Record<string, ExifMetadata>>({});
const loadingExif = ref(false);
const showAllFacesPill = ref(false);

const clusteredFacesList = computed(() => {
  return clusteredFaces.value.map((cf) => ({
    id: cf.face_id,
    crop_url: cf.crop_url,
  }));
});

function applyExifToItem(item: AlbumItem, meta: ExifMetadata) {
  if (meta.camera && !item.camera) item.camera = meta.camera;
  if (meta.lens && !item.lens) item.lens = meta.lens;
  if (meta.aperture && !item.aperture) item.aperture = meta.aperture;
  if (meta.shutter_speed && !item.shutter_speed) item.shutter_speed = meta.shutter_speed;
  if (meta.iso && !item.iso) item.iso = meta.iso;
  if (meta.focal_length && !item.focal_length) item.focal_length = meta.focal_length;
  if (meta.taken_at && !item.taken_at) item.taken_at = meta.taken_at;
  if (meta.width && !item.width) item.width = meta.width;
  if (meta.height && !item.height) item.height = meta.height;
}

async function loadPhotoExif(item: AlbumItem) {
  if (!item) return;
  const id = item.id;
  if (exifCache.value[id]) {
    applyExifToItem(item, exifCache.value[id]);
    return;
  }
  const url = safePhotoUrl(item.originalUrl || item.previewUrl || item.thumbnailUrl, item.filename);
  if (!url) return;
  loadingExif.value = true;
  try {
    const meta = await fetchAndParseExif(url);
    if (meta) {
      exifCache.value = { ...exifCache.value, [id]: meta };
      applyExifToItem(item, meta);
    }
  } catch (err) {
    console.warn('Failed to load EXIF for item:', item.filename, err);
  } finally {
    loadingExif.value = false;
  }
}

function onLightboxImageLoad(e: Event, item: AlbumItem | null) {
  if (!item) return;
  const img = e.target as HTMLImageElement;
  if (img && img.naturalWidth && img.naturalHeight) {
    if (!item.width) item.width = img.naturalWidth;
    if (!item.height) item.height = img.naturalHeight;
  }
}

const currentLightboxItem = computed(() => {
  if (activeLightboxIndex.value === null) return null;
  return displayItems.value[activeLightboxIndex.value] || null;
});

const currentExif = computed<ExifMetadata | null>(() => {
  const item = currentLightboxItem.value;
  if (!item) return null;
  return exifCache.value[item.id] || null;
});

const currentPhotoFaces = computed(() => {
  const item = currentLightboxItem.value;
  if (!item) return [];
  if (Array.isArray(item.faces) && item.faces.length > 0) return item.faces;
  return clusteredFaces.value
    .filter((cf) => {
      if (cf.photo_filenames && cf.photo_filenames.length > 0) {
        return cf.photo_filenames.includes(item.filename);
      }
      return cf.filename && cf.filename === item.filename;
    })
    .map((cf) => ({
      id: cf.face_id,
      crop_url: cf.crop_url,
    }));
});

const showFaceFrames = ref(true);

interface FaceBoxDisplay {
  id: number | string;
  style: Record<string, string>;
}

const currentItemFaceBoxes = computed<FaceBoxDisplay[]>(() => {
  const item = currentLightboxItem.value;
  if (!item) return [];

  const result: FaceBoxDisplay[] = [];
  if (Array.isArray(item.faces)) {
    for (const f of item.faces) {
      if (f.bbox) {
        const { x, y, width, height } = f.bbox;
        const isNorm = width <= 1.05 && height <= 1.05;
        const imgW = isNorm ? 1 : (item.width || 1000);
        const imgH = isNorm ? 1 : (item.height || 1000);

        const leftPct = (x / imgW) * 100;
        const topPct = (y / imgH) * 100;
        const widthPct = (width / imgW) * 100;
        const heightPct = (height / imgH) * 100;

        result.push({
          id: f.id,
          style: {
            left: `${leftPct.toFixed(2)}%`,
            top: `${topPct.toFixed(2)}%`,
            width: `${widthPct.toFixed(2)}%`,
            height: `${heightPct.toFixed(2)}%`,
          },
        });
      }
    }
  }
  return result;
});

watch(currentLightboxItem, (item) => {
  if (item) {
    showAllFacesPill.value = false;
    void loadPhotoExif(item);
    if (isIOS.value) {
      void preloadPhotoFile(item);
      if (activeLightboxIndex.value !== null) {
        const len = displayItems.value.length;
        if (len > 0) {
          const nextItem = displayItems.value[(activeLightboxIndex.value + 1) % len];
          const prevItem = displayItems.value[(activeLightboxIndex.value - 1 + len) % len];
          if (nextItem) void preloadPhotoFile(nextItem);
          if (prevItem) void preloadPhotoFile(prevItem);
        }
      }
    }
  }
}, { immediate: true });

function onKeydown(e: KeyboardEvent) {
  if (activeLightboxIndex.value === null) return;
  if (e.key === 'Escape') {
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
  }
}

const isLoadingGallery = ref(true);
const isUiHidden = ref(false);

function toggleUi() {
  isUiHidden.value = !isUiHidden.value;
}

onMounted(() => {
  isIOS.value = detectIsIOS();
  loadFavorites();

  // Smooth Preload for Cover Image & First Photos (Zero Layout Jitter)
  const preloadUrls: string[] = [];
  if (coverImageSrc.value) preloadUrls.push(coverImageSrc.value);
  props.album.items.slice(0, 16).forEach((item) => {
    const u = safePhotoUrl(item.previewUrl || item.thumbnailUrl, item.filename);
    if (u) preloadUrls.push(u);
  });

  const preloadPromise = Promise.allSettled(
    preloadUrls.map(
      (url) =>
        new Promise<void>((resolve) => {
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
        }),
    ),
  );

  // Guaranteed minimum loading duration to ensure smooth, stable initial render
  const minTimer = new Promise<void>((resolve) => setTimeout(resolve, 2000));
  void Promise.race([
    Promise.all([preloadPromise, minTimer]),
    new Promise<void>((resolve) => setTimeout(resolve, 4000)),
  ]).then(() => {
    isLoadingGallery.value = false;
  });
  if (typeof document !== 'undefined') {
    const saved = localStorage.getItem('filex_theme_preference') || localStorage.getItem('theme');
    if (saved === 'dark') {
      isDark.value = true;
    } else if (saved === 'light') {
      isDark.value = false;
    } else {
      isDark.value = document.documentElement.classList.contains('dark') ||
                     document.documentElement.classList.contains('fe--theme-dark') ||
                     window.matchMedia('(prefers-color-scheme: dark)').matches;
    }
    applyThemeClasses(isDark.value);
  }
  window.addEventListener('keydown', onKeydown);
  window.addEventListener('click', onClickOutsideMenu);
  void fetchAifsStatus();
  void fetchClusteredFaces();
  statusPollTimer = setInterval(() => {
    if (aifsStatus.value?.status === 'indexing' || aifsStatus.value?.status === 'pending') {
      void fetchAifsStatus();
    }
  }, 2500);
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown);
  window.removeEventListener('click', onClickOutsideMenu);
  if (toastTimer) clearTimeout(toastTimer);
  if (statusPollTimer) clearInterval(statusPollTimer);
});

watch(() => props.album, () => {
  loadFavorites();
});
</script>

<template>
  <div class="fe-album-root" :class="{ dark: isDark }">
    <!-- LUXURY LOADING SPLASH SCREEN (Zero Layout Jitter) -->
    <transition name="fe-splash-fade">
      <div v-if="isLoadingGallery" class="fe-gallery-splash" :class="{ dark: isDark }">
        <div class="splash-content">
          <div class="splash-logo-box">
            <img
              v-if="album.custom_logo_url || album.customLogoUrl"
              :src="album.custom_logo_url || album.customLogoUrl"
              :alt="album.title || album.name || 'Logo'"
              class="splash-logo-img"
            />
            <div v-else class="splash-logo-text">
              <span>{{ album.studio_name || album.studioName || 'TQD' }}</span>
              <span class="splash-gold-dot">.</span>
            </div>
          </div>
          <h2 class="splash-title">{{ album.title || album.name || 'Photo Album' }}</h2>
          <p v-if="album.clientName || album.client_name" class="splash-client">{{ album.clientName || album.client_name }}</p>
          <div class="splash-gold-spinner"></div>
          <div class="splash-subtext">Preparing photo collection...</div>
        </div>
      </div>
    </transition>
    <!-- Toast Notification -->
    <transition name="fe-album-fade">
      <div v-if="toastMsg" class="fe-album-toast" role="status">
        {{ toastMsg }}
      </div>
    </transition>

    <!-- ========================================================================= -->
    <!-- 1. HERO COVER SECTION (coverStyle: haduanh, editorial, minimal, split) -->
    <!-- ========================================================================= -->

    <!-- TEMPLATE 1: HADUANH LUXURY HERO (Luxury editorial hero style) -->
    <header v-if="coverStyle === 'haduanh'" class="hero-haduanh">
      <img v-if="coverImageSrc" class="hero-bg" :src="coverImageSrc" :alt="album.title" />
      <div class="hero-overlay-vignette"></div>

      <!-- Top Line -->
      <div class="top-line"></div>

      <!-- Center Content -->
      <div class="hero-center-content">
        <h1 class="hero-title">{{ album.title }}</h1>
        <p v-if="album.clientName" class="hero-subtitle">{{ album.clientName }}</p>
      </div>

      <!-- Bottom Line -->
      <div class="bottom-line"></div>

      <!-- Bottom Bar: Location, Description & Scroll button -->
      <div class="hero-bottom-bar">
        <div class="hero-location-tag">
          <span>{{ album.location || 'EDITORIAL GALLERY' }}</span>
          <span v-if="album.eventDate"> · {{ album.eventDate }}</span>
          <div v-if="album.description" style="margin-top: 4px; opacity: 0.85; text-transform: none; letter-spacing: normal;">
            {{ album.description }}
          </div>
        </div>
        <button type="button" class="btn-view-gallery" @click="scrollToGallery">
          View Gallery
        </button>
      </div>
    </header>

    <!-- TEMPLATE 2: EDITORIAL SERIF (Centered artistic serif) -->
    <header v-else-if="coverStyle === 'editorial'" class="hero-editorial">
      <img v-if="coverImageSrc" class="hero-editorial-bg" :src="coverImageSrc" :alt="album.title" />
      <div class="hero-editorial-overlay"></div>
      <div class="hero-editorial-content">
        <span class="hero-editorial-badge">- {{ album.clientName || 'COLLECTION' }} -</span>
        <h1 class="hero-editorial-title">{{ album.title }}</h1>
        <p v-if="album.description" class="hero-editorial-desc">{{ album.description }}</p>
        <div class="hero-editorial-meta">
          <span v-if="album.eventDate">{{ album.eventDate }}</span>
          <span v-if="album.location">• {{ album.location }}</span>
        </div>
        <div style="margin-top: 2rem;">
          <button type="button" class="btn-editorial-explore" @click="scrollToGallery">
            <span>Explore Collection</span>
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 5v14M19 12l-7 7-7-7" />
            </svg>
          </button>
        </div>
      </div>
    </header>

    <!-- TEMPLATE 3: MINIMALIST (Clean modern frame) -->
    <header v-else-if="coverStyle === 'minimal'" class="hero-minimal">
      <div class="hero-minimal-header">
        <span class="hero-minimal-badge">PHOTO COLLECTION</span>
        <h1 class="hero-minimal-title">{{ album.title }}</h1>
        <p v-if="album.clientName" class="hero-minimal-client">{{ album.clientName }}</p>
        <div class="hero-minimal-meta">
          <span v-if="album.eventDate">{{ album.eventDate }}</span>
          <span v-if="album.location">• {{ album.location }}</span>
        </div>
      </div>
      <div v-if="coverImageSrc" class="hero-minimal-card">
        <img :src="coverImageSrc" :alt="album.title" />
      </div>
      <div style="margin-top: 2rem; text-align: center;">
        <button type="button" class="btn-minimal-scroll" @click="scrollToGallery">
          View {{ album.items.length }} photos
        </button>
      </div>
    </header>

    <!-- TEMPLATE 4: SPLIT MAGAZINE (Two-column) -->
    <header v-else class="hero-split">
      <div class="hero-split-left">
        <span class="hero-split-badge">Client: {{ album.clientName || 'VIP Guest' }}</span>
        <h1 class="hero-split-title">{{ album.title }}</h1>
        <p v-if="album.description" class="hero-split-desc">{{ album.description }}</p>
        <div class="hero-split-meta">
          <div v-if="album.eventDate"><strong>Date:</strong> {{ album.eventDate }}</div>
          <div v-if="album.location"><strong>Location:</strong> {{ album.location }}</div>
        </div>
        <div style="margin-top: 2rem;">
          <button type="button" class="btn-split-scroll" @click="scrollToGallery">
            Explore Collection
          </button>
        </div>
      </div>
      <div class="hero-split-right">
        <div v-if="coverImageSrc" class="hero-split-imgwrap">
          <img :src="coverImageSrc" :alt="album.title" />
        </div>
      </div>
    </header>

    <!-- ========================================================================= -->
    <!-- 2. STICKY NAVBAR (Album title, All/Favorites filter, Select button, More menu) -->
    <!-- ========================================================================= -->
    <nav id="feAlbumNav" class="gallery-nav">
      <div class="gallery-nav-inner">
        <!-- Left: Album title, meta & filter pills -->
        <div class="gallery-nav-left">
          <div class="gallery-nav-title-group">
            <h2 class="gallery-nav-title">{{ album.title }}</h2>
            <p class="gallery-nav-meta">
              {{ album.items.length }} {{ album.items.length === 1 ? 'photo' : 'photos' }}
              <template v-if="totalFavorites > 0">
                · <span class="gallery-nav-fav-count">{{ totalFavorites }} favorited</span>
              </template>
              <template v-if="isSelectMode && selectedItemIds.size > 0">
                · <span class="gallery-nav-select-count">{{ selectedItemIds.size }} selected</span>
              </template>
              <template v-if="selectedFaceId !== null">
                · <span class="gallery-nav-ai-count">{{ displayItems.length }} {{ displayItems.length === 1 ? 'photo' : 'photos' }} found</span>
              </template>
            </p>
          </div>

          <!-- Filter Pills: All / Favorites / Found -->
          <div class="filter-pills">
            <button
              type="button"
              class="filter-btn"
              :class="{ active: filterMode === 'ALL' && selectedFaceId === null }"
              @click="clearFaceFilter(); filterMode = 'ALL'"
            >
              All ({{ album.items.length }})
            </button>
            <button
              type="button"
              class="filter-btn filter-btn--fav"
              :class="{ active: filterMode === 'FAVORITES' }"
              @click="filterMode = 'FAVORITES'"
            >
              <svg viewBox="0 0 24 24" width="14" height="14" :fill="filterMode === 'FAVORITES' ? (isDark ? '#FFFFFF' : '#171717') : 'none'" stroke="currentColor" stroke-width="2">
                <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
              </svg>
              <span>Favorites</span>
              <span class="filter-fav-badge">{{ totalFavorites }}</span>
            </button>
            <button
              v-if="selectedFaceId !== null || aiMatchCount > 0"
              type="button"
              class="filter-btn filter-btn--ai active"
              title="Filtered by face - Click to view all"
              @click="clearFaceFilter"
            >
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="7" r="4"/>
                <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
              </svg>
              <span>Found ({{ displayItems.length }})</span>
              <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.5">
                <line x1="18" y1="6" x2="6" y2="18"/>
                <line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
          </div>
        </div>

        <!-- Right: Action buttons (Select & "...") -->
        <div class="gallery-nav-actions">
          <!-- When in Select Mode -->
          <template v-if="isSelectMode">
            <span class="nav-selected-badge">{{ selectedItemIds.size }} selected</span>
            <button
              type="button"
              class="nav-btn-done"
              @click="toggleSelectMode"
            >
              Xong
            </button>
          </template>

          <!-- Normal mode: Select button -->
          <template v-else>
            <button
              type="button"
              class="nav-btn-select"
              title="Select multiple photos to download"
              @click="toggleSelectMode"
            >
              <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="9 11 12 14 22 4"/>
                <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/>
              </svg>
              <span>Select</span>
            </button>
          </template>

          <!-- The "..." More Menu (Advanced features & photographer tools) -->
          <div class="nav-more-wrapper">
            <button
              type="button"
              class="nav-btn-more"
              :class="{ active: showMoreMenu }"
              title="More options"
              aria-label="More options"
              @click.stop="showMoreMenu = !showMoreMenu"
            >
              <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                <circle cx="5" cy="12" r="2"/>
                <circle cx="12" cy="12" r="2"/>
                <circle cx="19" cy="12" r="2"/>
              </svg>
            </button>

            <!-- Dropdown Menu -->
            <transition name="fe-menu-pop">
              <div v-if="showMoreMenu" class="nav-more-dropdown" @click.stop>
                <!-- Share album link -->
                <button
                  type="button"
                  class="dropdown-item"
                  @click="handleCopyAlbumLink(); showMoreMenu = false"
                >
                  <div class="dropdown-item-left">
                    <svg v-if="copiedLink" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="#22c55e" stroke-width="2.5">
                      <polyline points="20 6 9 17 4 12"/>
                    </svg>
                    <svg v-else viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
                      <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
                    </svg>
                    <span>{{ copiedLink ? 'Link copied!' : 'Share album link' }}</span>
                  </div>
                </button>

                <!-- QR code for mobile -->
                <button
                  type="button"
                  class="dropdown-item"
                  @click="showQrModal = true; showMoreMenu = false"
                >
                  <div class="dropdown-item-left">
                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="3" y="3" width="7" height="7"></rect>
                      <rect x="14" y="3" width="7" height="7"></rect>
                      <rect x="14" y="14" width="7" height="7"></rect>
                      <rect x="3" y="14" width="7" height="7"></rect>
                    </svg>
                    <span>QR code for mobile</span>
                  </div>
                </button>

                <!-- Download full album (ZIP) -->
                <a
                  v-if="zipUrl"
                  :href="zipUrl"
                  class="dropdown-item"
                  download
                  @click="showMoreMenu = false"
                >
                  <div class="dropdown-item-left">
                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
                    </svg>
                    <span>Download full album (ZIP)</span>
                  </div>
                  <span class="dropdown-badge font-mono">({{ album.items.length }})</span>
                </a>

                <!-- Download favorited photos -->
                <button
                  v-if="totalFavorites > 0"
                  type="button"
                  class="dropdown-item dropdown-item--fav"
                  @click="handleDownloadFavorites(); showMoreMenu = false"
                >
                  <div class="dropdown-item-left">
                    <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" stroke="currentColor" stroke-width="2">
                      <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
                    </svg>
                    <span>Download favorited photos</span>
                  </div>
                  <span class="dropdown-badge font-mono">({{ totalFavorites }})</span>
                </button>

                <!-- ADVANCED PHOTOGRAPHER TOOL: Export favorited photos (CSV) -->
                <div class="dropdown-divider"></div>
                <div class="dropdown-section-title">Photographer Tools</div>

                <button
                  type="button"
                  class="dropdown-item dropdown-item--photographer"
                  :title="totalFavorites > 0 ? `Export CSV of ${totalFavorites} favorited photos` : 'Export favorited photos list to CSV'"
                  @click="exportFavoritesToCSV(); showMoreMenu = false"
                >
                  <div class="dropdown-item-left">
                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="#eab308" stroke-width="2">
                      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                      <polyline points="14 2 14 8 20 8"/>
                      <line x1="8" y1="13" x2="16" y2="13"/>
                      <line x1="8" y1="17" x2="16" y2="17"/>
                      <polyline points="10 9 9 9 8 9"/>
                    </svg>
                    <span>Export favorited photos (CSV)</span>
                  </div>
                  <span v-if="totalFavorites > 0" class="dropdown-badge font-mono">({{ totalFavorites }})</span>
                </button>

                <!-- View Options -->
                <div class="dropdown-divider"></div>
                <div class="dropdown-section-title">View Options</div>

                <div class="dropdown-config-row">
                  <span class="dropdown-config-label">Density:</span>
                  <div class="dropdown-toggle-group">
                    <button
                      type="button"
                      class="dropdown-toggle-btn"
                      :class="{ active: viewMode === 'standard' }"
                      @click="viewMode = 'standard'"
                    >
                      Standard
                    </button>
                    <button
                      type="button"
                      class="dropdown-toggle-btn"
                      :class="{ active: viewMode === 'large' }"
                      @click="viewMode = 'large'"
                    >
                      Large
                    </button>
                    <button
                      type="button"
                      class="dropdown-toggle-btn"
                      :class="{ active: viewMode === 'compact' }"
                      @click="viewMode = 'compact'"
                    >
                      Compact
                    </button>
                  </div>
                </div>

                <div class="dropdown-config-row">
                  <span class="dropdown-config-label">Aspect Ratio:</span>
                  <div class="dropdown-toggle-group">
                    <button
                      type="button"
                      class="dropdown-toggle-btn"
                      :class="{ active: aspectMode === 'natural' }"
                      @click="aspectMode = 'natural'"
                    >
                      Original
                    </button>
                    <button
                      type="button"
                      class="dropdown-toggle-btn"
                      :class="{ active: aspectMode === 'square' }"
                      @click="aspectMode = 'square'"
                    >
                      Square 1:1
                    </button>
                  </div>
                </div>

                <!-- Switch to file list view -->
                <div class="dropdown-divider"></div>
                <button
                  type="button"
                  class="dropdown-item"
                  @click="emit('switchToFileView'); showMoreMenu = false"
                >
                  <div class="dropdown-item-left">
                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                      <line x1="8" y1="6" x2="21" y2="6"/>
                      <line x1="8" y1="12" x2="21" y2="12"/>
                      <line x1="8" y1="18" x2="21" y2="18"/>
                      <line x1="3" y1="6" x2="3.01" y2="6"/>
                      <line x1="3" y1="12" x2="3.01" y2="12"/>
                      <line x1="3" y1="18" x2="3.01" y2="18"/>
                    </svg>
                    <span>Switch to file list view</span>
                  </div>
                </button>

                <!-- Day / Night Theme Toggle -->
                <div class="dropdown-divider"></div>
                <div class="dropdown-theme-toggle" @click="toggleDarkMode">
                  <div class="dropdown-theme-left">
                    <svg v-if="isDark" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
                    </svg>
                    <svg v-else viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="5"></circle>
                      <line x1="12" y1="1" x2="12" y2="3"></line>
                      <line x1="12" y1="21" x2="12" y2="23"></line>
                      <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
                      <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
                      <line x1="1" y1="12" x2="3" y2="12"></line>
                      <line x1="21" y1="12" x2="23" y2="12"></line>
                      <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
                      <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
                    </svg>
                    <span>Theme: {{ isDark ? 'Dark (Night)' : 'Light (Day)' }}</span>
                  </div>
                  <div class="dropdown-theme-switch" :class="{ on: isDark }">
                    <span class="dropdown-theme-knob"></span>
                  </div>
                </div>
              </div>
            </transition>
          </div>
        </div>
      </div>
    </nav>

    <!-- AI Face Indexing Live Banner & Report -->
    <div
      v-if="aifsStatus && (aifsStatus.status === 'indexing' || aifsStatus.status === 'pending')"
      class="gallery-indexing-banner"
    >
      <div class="indexing-banner-inner">
        <div class="indexing-banner-spinner"></div>
        <div class="indexing-banner-text">
          <span class="indexing-banner-heading">AI Face Indexing in progress</span>
          <span class="indexing-banner-sub">
            Processed {{ aifsStatus.indexed_photos ?? 0 }} / {{ aifsStatus.total_photos ?? 0 }} photos
            <template v-if="aifsStatus.detected_faces"> · {{ aifsStatus.detected_faces }} faces</template>
            <template v-if="aifsStatus.clusters_count"> · {{ aifsStatus.clusters_count }} people</template>
          </span>
        </div>
        <span class="indexing-banner-percent">{{ Math.round(aifsStatus.percent_complete ?? 0) }}%</span>
      </div>
      <div class="indexing-banner-bar">
        <div class="indexing-banner-bar-fill" :style="{ width: `${Math.min(100, Math.round(aifsStatus.percent_complete ?? 0))}%` }"></div>
      </div>
    </div>

    <!-- ========================================================================= -->
    <!-- 3.1 AI FACE HERO BANNER - AIFS LUXURY -->
    <!-- ========================================================================= -->
    <section class="gallery-face-hero-container">
      <div class="aifs-face-hero-card">
        <div class="aifs-face-hero-top">
          <div class="aifs-face-hero-info">
            <div class="aifs-face-hero-icon-ring">
              <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2.2">
                <path d="M7 3H5a2 2 0 0 0-2 2v2M17 3h2a2 2 0 0 1 2 2v2M7 21H5a2 2 0 0 1-2-2v-2M17 21h2a2 2 0 0 0 2-2v-2"/>
                <circle cx="12" cy="12" r="3"/>
                <path d="M12 16a4 4 0 0 1-4-4"/>
              </svg>
            </div>
            <div>
              <div class="aifs-face-hero-title-row">
                <h3 class="aifs-face-hero-title">Find photos by face</h3>
                <span class="aifs-face-hero-badge">AI Face</span>
              </div>
              <p class="aifs-face-hero-desc">
                Take or upload a selfie for AI to automatically find all your photos in this album
              </p>
            </div>
          </div>

          <button
            type="button"
            class="btn-aifs-selfie-cta"
            @click="showAiModal = true"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
              <circle cx="12" cy="13" r="4"/>
            </svg>
            <span>Take or upload selfie</span>
          </button>
        </div>

        <!-- Clustered Faces Carousel if available -->
        <div v-if="clusteredFaces.length > 0 || loadingFaces" class="aifs-face-hero-people">
          <div class="aifs-face-hero-people-header">
            <span class="aifs-face-hero-people-label">
              Or click on a person in this album ({{ clusteredFaces.length }} people):
            </span>
            <button
              v-if="selectedFaceId !== null"
              type="button"
              class="btn-aifs-clear-link"
              @click="clearFaceFilter"
            >
              View all photos
            </button>
          </div>
          <PeopleSelector
            :faces="clusteredFaces"
            :selected-face-id="selectedFaceId"
            :loading="loadingFaces"
            :hide-header="true"
            @select="handleSelectFace"
          />
        </div>
      </div>
    </section>

    <!-- ========================================================================= -->
    <!-- 3.2 ACTIVE FACE FILTER BANNER (Face filter active) -->
    <!-- ========================================================================= -->
    <div v-if="selectedFaceId !== null" class="aifs-active-face-banner">
      <div class="active-face-left">
        <img
          v-if="activeFaceCropUrl"
          :src="activeFaceCropUrl"
          alt="Face"
          class="active-face-thumb"
        />
        <div v-else class="active-face-thumb-placeholder">
          <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="7" r="4"/>
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
          </svg>
        </div>
        <div>
          <div class="active-face-title-row">
            <span class="active-face-title">Face Filter Results</span>
            <span class="active-face-badge">{{ displayItems.length }} {{ displayItems.length === 1 ? 'photo' : 'photos' }}</span>
          </div>
          <p class="active-face-sub">Showing all photos featuring this person in the album</p>
        </div>
      </div>

      <div class="active-face-right">
        <button
          v-if="displayItems.length > 0"
          type="button"
          class="btn-aifs-download-active"
          @click="selectedItemIds = new Set(displayItems.map(i => i.id)); handleBatchAction()"
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
          </svg>
          <span>Download all ({{ displayItems.length }})</span>
        </button>
        <button
          type="button"
          class="btn-aifs-clear-active"
          @click="clearFaceFilter"
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="18" y1="6" x2="6" y2="18"/>
            <line x1="6" y1="6" x2="18" y2="18"/>
          </svg>
          <span>View all photos</span>
        </button>
      </div>
    </div>

    <!-- ========================================================================= -->
    <!-- 3. MASONRY GALLERY GRID (Natural aspect ratio, uncropped)                  -->
    <!-- ========================================================================= -->
    <main class="gallery-main">
      <div v-if="displayItems.length === 0" class="gallery-empty-state">
        <svg viewBox="0 0 24 24" width="48" height="48" fill="none" stroke="#cbd5e1" stroke-width="1.5">
          <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
        </svg>
        <p v-if="filterMode === 'AI_MATCHES'">No photos match this face filter.</p>
        <p v-else-if="filterMode === 'FAVORITES'">No photos have been added to favorites yet.</p>
        <p v-else>No photos found in this collection.</p>
        <button type="button" class="btn-reset-filter" @click="filterMode = 'ALL'">
          View all photos
        </button>
      </div>

      <!-- Favorites action banner when viewing favorites -->
      <div v-else-if="filterMode === 'FAVORITES'" class="gallery-favorites-header">
        <div class="favorites-header-info">
          <span class="favorites-header-icon">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="#FFC629" stroke="#FFC629">
              <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
            </svg>
          </span>
          <span class="favorites-header-text">Favorited Photos · {{ displayItems.length }} {{ displayItems.length === 1 ? 'photo' : 'photos' }}</span>
        </div>
        <button type="button" class="btn-fav-export-csv" @click="exportFavoritesToCSV">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
            <line x1="8" y1="13" x2="16" y2="13"/>
            <line x1="8" y1="17" x2="16" y2="17"/>
          </svg>
          <span>Export CSV</span>
        </button>
      </div>

      <div v-if="displayItems.length > 0" :class="['masonry-columns', `view-${viewMode}`, `aspect-${aspectMode}`]">
        <div
          v-for="(item, index) in displayItems"
          :key="item.id || index"
          class="photo-card"
          :class="{ 'is-selected': selectedItemIds.has(item.id), 'card-square': aspectMode === 'square', 'in-select-mode': isSelectMode }"
          @click="isSelectMode ? toggleSelectItem(item.id, $event) : openLightbox(index)"
        >
          <!-- Frame Border Container -->
          <div class="photo-card-frame">
            <!-- Selection indicator in select mode -->
            <div
              v-if="isSelectMode"
              class="photo-select-checkbox"
              :class="{ 'is-checked': selectedItemIds.has(item.id) }"
              @click.stop="toggleSelectItem(item.id, $event)"
            >
              <svg v-if="selectedItemIds.has(item.id)" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="#ffffff" stroke-width="3">
                <polyline points="20 6 9 17 4 12"/>
              </svg>
              <span v-else class="photo-select-checkbox-dot"></span>
            </div>

            <!-- AI similarity match badge -->
            <div v-if="aiMatchesMap[item.filename]" class="ai-match-badge" title="Face similarity match">
              <svg viewBox="0 0 24 24" width="11" height="11" fill="currentColor">
                <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"/>
              </svg>
              <span>{{ Math.round((aiMatchesMap[item.filename].similarity || 0.85) * 100) }}% Match</span>
            </div>

            <!-- Natural aspect ratio image -->
            <img
              :src="safePhotoUrl(item.thumbnailUrl || item.previewUrl || item.originalUrl, item.filename)"
              :alt="item.filename"
              loading="lazy"
              class="photo-card-img"
            />

            <!-- Subtle Vignette Scrim for Action Clarity -->
            <div class="photo-card-scrim"></div>

            <!-- Bottom info badge (filename & MP) -->
            <div class="photo-card-info-badge">
              <span class="photo-card-info-name">{{ item.filename }}</span>
              <span v-if="getMegapixels(item.width, item.height)" class="photo-card-info-mp">
                {{ getMegapixels(item.width, item.height) }}
              </span>
            </div>

            <!-- Quick Download / Save to Photos button -->
            <button
              type="button"
              class="btn-download-quick photo-action-shadow"
              :title="isIOS ? 'Save to iOS Photos' : 'Download photo'"
              @click.stop="downloadPhoto(item, $event)"
            >
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="#ffffff" stroke-width="2.5">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
              </svg>
            </button>

            <!-- Heart favorite button -->
            <button
              type="button"
              class="btn-heart photo-heart-shadow heart-interactive"
              :class="{ 'is-favorited': favoritesMap[item.id] }"
              :title="favoritesMap[item.id] ? 'Favorited' : 'Add to favorites'"
              @click.stop="toggleFavorite(item.id, $event)"
            >
              <svg
                viewBox="0 0 24 24"
                width="22"
                height="22"
                :fill="favoritesMap[item.id] ? '#ffffff' : 'rgba(0,0,0,0.3)'"
                :stroke="favoritesMap[item.id] ? '#ffffff' : '#ffffff'"
                stroke-width="2"
              >
                <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
              </svg>
            </button>
          </div>
        </div>
      </div>
    </main>

    <!-- ========================================================================= -->
    <!-- FOOTER                                                                    -->
    <!-- ========================================================================= -->
    <footer class="gallery-footer">
      <p>© Gallery Platform — All Rights Reserved.</p>
    </footer>

    <!-- ========================================================================= -->
    <!-- 4. FULLSCREEN LIGHTBOX MODAL                                              -->
    <!-- ========================================================================= -->
    <teleport to="body">
      <transition name="fe-album-fade">
        <div
          v-if="activeLightboxIndex !== null && currentLightboxItem"
          class="lightbox-modal"
          :class="{ 'ui-hidden': isUiHidden }"
          @touchstart="onTouchStart"
          @touchend="onTouchEnd"
        >
          <!-- Top bar -->
          <div class="lightbox-topbar">
            <span class="lightbox-counter">
              {{ (activeLightboxIndex ?? 0) + 1 }} / {{ displayItems.length }}
            </span>

            <div class="lightbox-actions">
              <button
                type="button"
                class="lightbox-btn"
                :title="favoritesMap[currentLightboxItem.id] ? 'Favorited' : 'Favorite'"
                @click="toggleFavorite(currentLightboxItem.id)"
              >
                <svg
                  viewBox="0 0 24 24"
                  width="20"
                  height="20"
                  :fill="favoritesMap[currentLightboxItem.id] ? '#ffffff' : 'none'"
                  :stroke="favoritesMap[currentLightboxItem.id] ? '#ffffff' : 'currentColor'"
                  stroke-width="2"
                >
                  <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
                </svg>
                <span style="font-size: 13px;">{{ favoritesMap[currentLightboxItem.id] ? 'Favorited' : 'Favorite' }}</span>
              </button>

              <!-- Copy Link -->
              <button
                type="button"
                class="lightbox-btn"
                title="Copy Link"
                @click="currentLightboxItem && copyPhotoLink(currentLightboxItem)"
              >
                <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
                  <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
                </svg>
                <span style="font-size: 13px;">Copy Link</span>
              </button>

              <!-- Toggle Full Photo (Hide UI) -->
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
              >
                <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="12" r="10"/>
                  <line x1="12" y1="16" x2="12" y2="12"/>
                  <line x1="12" y1="8" x2="12.01" y2="8"/>
                </svg>
                <span style="font-size: 13px;">Info</span>
              </button>

              <!-- Toggle Face Frames Button -->
              <button
                v-if="currentItemFaceBoxes.length > 0"
                type="button"
                class="lightbox-btn"
                :class="{ 'is-active': showFaceFrames }"
                title="Toggle Face Detection Frames"
                @click="showFaceFrames = !showFaceFrames"
              >
                <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M7 3H5a2 2 0 0 0-2 2v2M17 3h2a2 2 0 0 1 2 2v2M7 21H5a2 2 0 0 1-2-2v-2M17 21h2a2 2 0 0 0 2-2v-2"/>
                  <circle cx="12" cy="12" r="3"/>
                </svg>
                <span style="font-size: 13px;">Face Frames ({{ currentItemFaceBoxes.length }})</span>
              </button>

              <button
                type="button"
                class="lightbox-btn"
                :title="isIOS ? 'Save to Photos' : 'Download'"
                :disabled="isDownloading"
                @click="currentLightboxItem && downloadPhoto(currentLightboxItem)"
              >
                <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
                </svg>
                <span style="font-size: 13px;">{{ isIOS ? 'Save to Photos' : 'Download' }}</span>
              </button>

              <button type="button" class="lightbox-btn" title="Close (Esc)" @click="closeLightbox">
                <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="18" y1="6" x2="6" y2="18"/>
                  <line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
              </button>
            </div>
          </div>

          <!-- Middle image & nav -->
          <div class="lightbox-center" @click.self="closeLightbox">
            <button type="button" class="lightbox-nav-btn left" title="Previous photo (Left arrow)" @click.stop="prevLightbox">
              <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="15 18 9 12 15 6"/>
              </svg>
            </button>

            <div class="lightbox-stage">
              <img
                :src="safePhotoUrl(currentLightboxItem.originalUrl || currentLightboxItem.previewUrl, currentLightboxItem.filename)"
                :alt="currentLightboxItem.filename"
                class="lightbox-main-img"
                :class="{ 'is-fullscreen-img': isUiHidden }"
                title="Click image to toggle full photo view"
                @load="onLightboxImageLoad($event, currentLightboxItem)"
                @click="toggleUi"
              />

              <!-- Face Bounding Box Frames -->
              <div v-if="showFaceFrames && currentItemFaceBoxes.length > 0" class="lightbox-face-overlay">
                <div
                  v-for="box in currentItemFaceBoxes"
                  :key="box.id"
                  class="lightbox-face-frame"
                  :style="box.style"
                  :title="`Person #${box.id} · Click to filter photos`"
                  @click.stop="handleSelectFaceFromLightbox(box.id)"
                >
                  <span class="face-frame-corner tl"></span>
                  <span class="face-frame-corner tr"></span>
                  <span class="face-frame-corner bl"></span>
                  <span class="face-frame-corner br"></span>
                  <span class="face-frame-chip">Person #{{ box.id }}</span>
                </div>
              </div>
            </div>

            <button type="button" class="lightbox-nav-btn right" title="Next photo (Right arrow)" @click.stop="nextLightbox">
              <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="9 18 15 12 9 6"/>
              </svg>
            </button>
          </div>

          <!-- Floating Quick Info Pill on Desktop (Bottom Left) -->
          <div v-if="currentLightboxItem" class="lightbox-floating-info-pill">
            <button
              type="button"
              class="lightbox-quick-exif-btn"
              :class="{ 'is-active': showInfo }"
              @click.stop="showInfo = !showInfo"
              title="Click to view full photo details & EXIF"
            >
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
                <circle cx="12" cy="13" r="4"/>
              </svg>
              <span class="quick-exif-main">{{ currentExif?.camera || currentLightboxItem.camera || 'Photo details' }}</span>
              <template v-if="currentExif?.taken_at || currentLightboxItem.taken_at">
                <span class="quick-exif-sep">•</span>
                <span>{{ new Date(currentExif?.taken_at || currentLightboxItem.taken_at!).toLocaleDateString() }}</span>
              </template>
              <template v-if="getMegapixels(currentExif?.width || currentLightboxItem.width, currentExif?.height || currentLightboxItem.height)">
                <span class="quick-exif-sep">•</span>
                <span class="quick-exif-mp">{{ getMegapixels(currentExif?.width || currentLightboxItem.width, currentExif?.height || currentLightboxItem.height) }}</span>
              </template>
              <span class="quick-exif-action">Details →</span>
            </button>
          </div>

          <!-- Floating Bottom Quick Face Filter Bar (AIFS Style) -->
          <div
            v-if="currentPhotoFaces.length > 0 || (clusteredFaces.length > 0 && showAllFacesPill)"
            class="lightbox-faces-pill-container"
          >
            <div class="lightbox-faces-pill">
              <span class="lightbox-faces-pill-label">
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
                  <circle cx="9" cy="7" r="4"/>
                  <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
                  <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
                </svg>
                <span>{{ currentPhotoFaces.length > 0 && !showAllFacesPill ? 'In photo:' : 'Album People:' }}</span>
              </span>

              <div class="lightbox-faces-avatars">
                <button
                  v-for="face in (showAllFacesPill ? clusteredFacesList : currentPhotoFaces)"
                  :key="face.id"
                  type="button"
                  class="lightbox-face-avatar-btn"
                  :title="`Click to filter all photos for Person #${face.id}`"
                  @click.stop="handleSelectFaceFromLightbox(face.id)"
                >
                  <img
                    :src="face.crop_url || (token ? `/api/public/s/${token}/faces/${face.id}/crop` : '')"
                    alt="Face"
                  />
                </button>
              </div>

              <button
                v-if="clusteredFaces.length > currentPhotoFaces.length && currentPhotoFaces.length > 0"
                type="button"
                class="lightbox-faces-toggle-all-btn"
                @click.stop="showAllFacesPill = !showAllFacesPill"
                :title="showAllFacesPill ? 'Show faces in this photo only' : 'Show all people in album'"
              >
                {{ showAllFacesPill ? 'In photo' : `All (${clusteredFaces.length})` }}
              </button>
            </div>
          </div>
          <div
            v-else-if="clusteredFaces.length > 0"
            class="lightbox-faces-pill-container"
          >
            <div class="lightbox-faces-pill">
              <span class="lightbox-faces-pill-label">
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
                  <circle cx="9" cy="7" r="4"/>
                </svg>
                <span>Album People:</span>
              </span>
              <div class="lightbox-faces-avatars">
                <button
                  v-for="face in clusteredFacesList"
                  :key="face.id"
                  type="button"
                  class="lightbox-face-avatar-btn"
                  :title="`Click to filter all photos for Person #${face.id}`"
                  @click.stop="handleSelectFaceFromLightbox(face.id)"
                >
                  <img
                    :src="face.crop_url || (token ? `/api/public/s/${token}/faces/${face.id}/crop` : '')"
                    alt="Face"
                  />
                </button>
              </div>
            </div>
          </div>

          <!-- Bottom bar -->
          <div class="lightbox-bottombar">
            <span>{{ currentLightboxItem.filename }}</span>
            <div class="lightbox-info-details">
              <span v-if="currentLightboxItem.camera">
                {{ currentLightboxItem.camera }} ·
              </span>
              <span v-if="currentLightboxItem.width && currentLightboxItem.height">
                {{ currentLightboxItem.width }} × {{ currentLightboxItem.height }}
              </span>
              <span v-if="currentLightboxItem.size">
                · {{ (currentLightboxItem.size / (1024 * 1024)).toFixed(1) }} MB
              </span>
            </div>
          </div>

          <!-- EXIF & Details Drawer -->
          <transition name="fe-slide-right">
            <div v-if="showInfo" class="lightbox-exif-drawer">
              <div class="lightbox-exif-header">
                <div class="lightbox-exif-title">
                  <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
                    <circle cx="12" cy="13" r="4"/>
                  </svg>
                  <span>Photo Details</span>
                </div>
                <button type="button" class="btn-close-drawer" @click="showInfo = false" title="Close details">
                  <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                    <line x1="18" y1="6" x2="6" y2="18"/>
                    <line x1="6" y1="6" x2="18" y2="18"/>
                  </svg>
                </button>
              </div>

              <div class="lightbox-exif-body">
                <!-- Reading EXIF spinner -->
                <div v-if="loadingExif" class="exif-section" style="opacity: 0.8; font-size: 12px; display: flex; align-items: center; gap: 8px;">
                  <span class="indexing-banner-spinner" style="width: 14px; height: 14px;"></span>
                  <span>Loading photo EXIF metadata...</span>
                </div>

                <!-- Faces in this photo -->
                <div v-if="currentPhotoFaces.length > 0" class="exif-section">
                  <div class="exif-section-title">
                    <span>People in Photo ({{ currentPhotoFaces.length }})</span>
                  </div>
                  <div class="exif-faces-list">
                    <div
                      v-for="f in currentPhotoFaces"
                      :key="f.id"
                      class="exif-face-chip"
                      @click="handleSelectFaceFromLightbox(f.id)"
                    >
                      <img
                        :src="f.crop_url || (token ? `/api/public/s/${token}/faces/${f.id}/crop` : '')"
                        alt="Face"
                        class="exif-face-avatar"
                      />
                      <span class="exif-face-label">Person #{{ f.id }}</span>
                      <span class="exif-face-action">Filter</span>
                    </div>
                  </div>
                </div>

                <!-- Camera Info -->
                <div v-if="(currentExif?.camera || currentLightboxItem.camera) || (currentExif?.lens || currentLightboxItem.lens) || (currentExif?.taken_at || currentLightboxItem.taken_at)" class="exif-section">
                  <div class="exif-section-title">Camera & Gear</div>
                  <div v-if="currentExif?.camera || currentLightboxItem.camera" class="exif-row">
                    <span class="exif-label">Camera</span>
                    <span class="exif-value">{{ currentExif?.camera || currentLightboxItem.camera }}</span>
                  </div>
                  <div v-if="currentExif?.lens || currentLightboxItem.lens" class="exif-row">
                    <span class="exif-label">Lens</span>
                    <span class="exif-value">{{ currentExif?.lens || currentLightboxItem.lens }}</span>
                  </div>
                  <div v-if="currentExif?.taken_at || currentLightboxItem.taken_at" class="exif-row">
                    <span class="exif-label">Date Taken</span>
                    <span class="exif-value">{{ new Date(currentExif?.taken_at || currentLightboxItem.taken_at!).toLocaleString('en-US', { dateStyle: 'medium', timeStyle: 'short' }) }}</span>
                  </div>
                </div>

                <!-- Exposure Parameters -->
                <div v-if="(currentExif?.aperture || currentLightboxItem.aperture) || (currentExif?.shutter_speed || currentLightboxItem.shutter_speed) || (currentExif?.iso || currentLightboxItem.iso) || (currentExif?.focal_length || currentLightboxItem.focal_length)" class="exif-section">
                  <div class="exif-section-title">Exposure</div>
                  <div class="exif-params-grid">
                    <div v-if="currentExif?.aperture || currentLightboxItem.aperture" class="exif-param-box">
                      <span class="exif-param-name">Aperture</span>
                      <span class="exif-param-val">{{ currentExif?.aperture || currentLightboxItem.aperture }}</span>
                    </div>
                    <div v-if="currentExif?.shutter_speed || currentLightboxItem.shutter_speed" class="exif-param-box">
                      <span class="exif-param-name">Shutter</span>
                      <span class="exif-param-val">{{ currentExif?.shutter_speed || currentLightboxItem.shutter_speed }}</span>
                    </div>
                    <div v-if="currentExif?.iso || currentLightboxItem.iso" class="exif-param-box">
                      <span class="exif-param-name">ISO</span>
                      <span class="exif-param-val">{{ currentExif?.iso || currentLightboxItem.iso }}</span>
                    </div>
                    <div v-if="currentExif?.focal_length || currentLightboxItem.focal_length" class="exif-param-box">
                      <span class="exif-param-name">Focal Length</span>
                      <span class="exif-param-val">{{ currentExif?.focal_length || currentLightboxItem.focal_length }}</span>
                    </div>
                  </div>
                </div>

                <!-- File details -->
                <div class="exif-section">
                  <div class="exif-section-title">File Details</div>
                  <div class="exif-row">
                    <span class="exif-label">Filename</span>
                    <span class="exif-value filename-wrap">{{ currentLightboxItem.filename }}</span>
                  </div>
                  <div v-if="(currentExif?.width || currentLightboxItem.width) && (currentExif?.height || currentLightboxItem.height)" class="exif-row">
                    <span class="exif-label">Dimensions</span>
                    <span class="exif-value">{{ currentExif?.width || currentLightboxItem.width }} × {{ currentExif?.height || currentLightboxItem.height }} px {{ getMegapixels(currentExif?.width || currentLightboxItem.width, currentExif?.height || currentLightboxItem.height) ? `(${getMegapixels(currentExif?.width || currentLightboxItem.width, currentExif?.height || currentLightboxItem.height)})` : '' }}</span>
                  </div>
                  <div v-if="currentLightboxItem.size" class="exif-row">
                    <span class="exif-label">Size</span>
                    <span class="exif-value">{{ formatFileSize(currentLightboxItem.size) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </transition>
        </div>
      </transition>
    </teleport>

    <!-- 5. AI FACE SEARCH MODAL (AIFS LUXURY GOLD) -->
    <teleport to="body">
      <transition name="fe-album-fade">
        <div
          v-if="showAiModal"
          class="aifs-selfie-modal-backdrop"
          @click="showAiModal = false"
        >
          <div
            class="aifs-selfie-modal-card"
            @click.stop
          >
            <!-- Close Button -->
            <button
              type="button"
              class="aifs-selfie-modal-close"
              @click="showAiModal = false"
              title="Close"
            >
              <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"/>
                <line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>

            <!-- Hidden file inputs -->
            <input
              ref="cameraInputRef"
              type="file"
              accept="image/*"
              capture="user"
              style="display: none;"
              @change="onSelfieFileChosen"
            />
            <input
              ref="selfieInputRef"
              type="file"
              accept="image/*"
              style="display: none;"
              @change="onSelfieFileChosen"
            />

            <!-- Modal Header -->
            <div class="aifs-selfie-modal-header">
              <div class="aifs-selfie-modal-icon-ring">
                <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2.2">
                  <path d="M7 3H5a2 2 0 0 0-2 2v2M17 3h2a2 2 0 0 1 2 2v2M7 21H5a2 2 0 0 1-2-2v-2M17 21h2a2 2 0 0 0 2-2v-2"/>
                  <circle cx="12" cy="12" r="3"/>
                  <path d="M12 16a4 4 0 0 1-4-4"/>
                </svg>
              </div>
              <div>
                <h3 class="aifs-selfie-modal-title">Find photos by face</h3>
                <p class="aifs-selfie-modal-subtitle">Upload a selfie to find all photos of you in this album</p>
              </div>
            </div>

            <!-- Options: Not selected yet -->
            <div v-if="!aiSelfiePreview" class="aifs-selfie-options">
              <!-- Option 1: Take selfie now -->
              <button
                type="button"
                class="btn-selfie-camera"
                @click="triggerCamera"
              >
                <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2.5">
                  <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
                  <circle cx="12" cy="13" r="4"/>
                </svg>
                <span>Take a selfie now</span>
              </button>

              <!-- Option 2: Choose photo from library -->
              <button
                type="button"
                class="btn-selfie-gallery"
                @click="triggerGalleryUpload"
              >
                <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"/>
                  <path d="M12 12v9"/>
                  <path d="m16 16-4-4-4 4"/>
                </svg>
                <span>Choose photo from library</span>
              </button>

              <!-- Privacy note -->
              <div class="aifs-selfie-privacy-note">
                <strong>Privacy Protection:</strong> Your selfie is only used for temporary on-device face matching in memory and is never permanently stored on the server.
              </div>
            </div>

            <!-- Option 2: Selfie preview selected -->
            <div v-else class="aifs-selfie-preview-section">
              <div class="aifs-selfie-preview-card">
                <img :src="aiSelfiePreview" alt="Selfie" class="aifs-selfie-preview-img" />
                <div class="aifs-selfie-preview-info">
                  <h4 class="aifs-selfie-preview-heading">Selected selfie</h4>
                  <p class="aifs-selfie-preview-desc">Your face will be matched against all photos in this album.</p>
                  <button
                    type="button"
                    :disabled="aiSearching"
                    class="btn-selfie-change"
                    @click="clearSelfie"
                  >
                    <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
                    </svg>
                    <span>Choose another photo</span>
                  </button>
                </div>
              </div>

              <!-- Searching Progress -->
              <div v-if="aiSearching" class="aifs-selfie-searching-box">
                <div class="aifs-selfie-spinner"></div>
                <p class="aifs-selfie-searching-step">{{ aiSearchStep || 'AI is scanning and matching faces...' }}</p>
                <p class="aifs-selfie-searching-hint">Please wait a few seconds...</p>
              </div>

              <!-- Search Submit Button -->
              <button
                v-else
                type="button"
                class="btn-selfie-submit"
                @click="runSelfieSearch"
              >
                Start Search
              </button>
            </div>

            <!-- Error alert -->
            <div v-if="aiSearchError" class="aifs-selfie-error-banner">
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"/>
                <line x1="12" y1="8" x2="12" y2="12"/>
                <line x1="12" y1="16" x2="12.01" y2="16"/>
              </svg>
              <span>{{ aiSearchError }}</span>
            </div>
          </div>
        </div>
      </transition>
    </teleport>

    <!-- 6. FLOATING BATCH SELECTION BAR -->
    <teleport to="body">
      <transition name="fe-album-fade">
        <div v-if="isSelectMode && displayItems.length > 0" class="gallery-selection-bar">
          <div class="selection-bar-left">
            <span class="selection-count">{{ selectedItemIds.size }} / {{ displayItems.length }} selected</span>
          </div>
          <div class="selection-actions">
            <button type="button" class="btn-selection-action" @click="selectAllItems">
              {{ selectedItemIds.size === displayItems.length ? 'Deselect All' : 'Select All' }}
            </button>
            <button
              type="button"
              class="btn-selection-action btn-selection-action--fav"
              :disabled="selectedItemIds.size === 0"
              title="Add selected photos to favorites"
              @click="favoriteSelectedItems"
            >
              <svg viewBox="0 0 24 24" width="13" height="13" fill="#ffffff" stroke="#ffffff" stroke-width="2">
                <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
              </svg>
              <span>Favorite</span>
            </button>
            <button
              type="button"
              class="btn-selection-action btn-selection-action--csv"
              :disabled="selectedItemIds.size === 0"
              title="Export selected photos to CSV"
              @click="exportSelectedToCSV"
            >
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                <polyline points="14 2 14 8 20 8"/>
                <line x1="8" y1="13" x2="16" y2="13"/>
                <line x1="8" y1="17" x2="16" y2="17"/>
              </svg>
              <span>Export CSV</span>
            </button>
            <button
              type="button"
              class="btn-selection-action btn-selection-action--primary"
              :disabled="selectedItemIds.size === 0"
              @click="handleBatchAction"
            >
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
              </svg>
              <span>{{ isIOS ? 'Save Photos' : 'Download' }}</span>
            </button>
            <button
              type="button"
              class="btn-selection-action btn-selection-action--close"
              title="Exit selection mode"
              @click="toggleSelectMode"
            >
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.5">
                <line x1="18" y1="6" x2="6" y2="18"/>
                <line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
          </div>
        </div>
      </transition>
    </teleport>

    <!-- 7. IOS SAVE MODAL -->
    <teleport to="body">
      <IOSAlbumSaveModal
        v-if="showIosSaveModal"
        :photos="selectedPhotos.length > 0 ? selectedPhotos : album.items"
        :token="token"
        @close="showIosSaveModal = false"
      />
    </teleport>

    <!-- 8. QR CODE MODAL -->
    <teleport to="body">
      <QRCodeModal
        v-if="showQrModal"
        :url="currentUrl"
        :title="album.title"
        @close="showQrModal = false"
      />
    </teleport>

    <!-- 9. MULTI DOWNLOAD WIDGET -->
    <teleport to="body">
      <MultiDownloadWidget
        v-if="showMultiDl"
        :photos="selectedPhotos.length > 0 ? selectedPhotos : album.items"
        @close="showMultiDl = false"
      />
    </teleport>
  </div>
</template>

<style>
/* =========================================================================
   PUBLIC SHARE GALLERY - VANILLA & REACT CSS (STANDALONE STYLESHEET)
   Matching web/sharing_source: Luxury Editorial Gallery
   ========================================================================= */

@import url('https://fonts.googleapis.com/css2?family=Cormorant+Garamond:ital,wght@0,300;0,400;0,500;0,600;1,400&family=Inter:wght@300;400;500;600;700&family=Montserrat:wght@400;600;700;800&family=Playfair+Display:ital,wght@0,400;0,500;0,600;1,400&display=swap');

.fe-album-root {
  position: fixed;
  inset: 0;
  width: 100vw;
  height: 100vh;
  z-index: 1000;
  overflow-x: hidden;
  overflow-y: auto;
  background-color: #FFFDF8 !important;
  color: #171717 !important;
  transition: background-color 0.25s ease, color 0.25s ease;
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  -webkit-font-smoothing: antialiased;
  scroll-behavior: smooth;
}

.fe-album-root.dark,
html.dark .fe-album-root,
body.dark .fe-album-root {
  background-color: #0E0E0D !important;
  color: #F5F5F4 !important;
}


/* 1. HERO COVER: STYLE HADUANH LUXURY */
.hero-haduanh {
  position: relative;
  width: 100%;
  height: 100vh;
  min-height: 580px;
  background-color: #0a0a0a;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: space-between;
  user-select: none;
}

.hero-haduanh .hero-bg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
  filter: brightness(0.92) contrast(1.03);
}

.hero-haduanh .hero-overlay-vignette {
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, rgba(0,0,0,0.6) 0%, rgba(0,0,0,0.25) 50%, transparent 100%),
              linear-gradient(0deg, rgba(0,0,0,0.7) 0%, transparent 50%, rgba(0,0,0,0.4) 100%);
  pointer-events: none;
}

.hero-haduanh .top-line {
  position: absolute;
  top: 40px;
  left: 5%;
  right: 5%;
  max-width: 1700px;
  margin: 0 auto;
  border-top: 1px solid rgba(255, 255, 255, 0.4);
  pointer-events: none;
}

.hero-haduanh .hero-center-content {
  position: relative;
  z-index: 10;
  width: 90%;
  max-width: 1700px;
  margin: 0 auto;
  padding: 0 24px;
}

.hero-haduanh .hero-title {
  font-family: 'Montserrat', 'Inter', sans-serif;
  font-size: clamp(2rem, 5vw, 4rem);
  font-weight: 800;
  letter-spacing: -0.02em;
  color: #ffffff;
  text-transform: uppercase;
  max-width: 750px;
  line-height: 1.15;
  text-shadow: 0 2px 10px rgba(0, 0, 0, 0.5);
  margin: 0;
}

.hero-haduanh .hero-subtitle {
  font-size: clamp(0.85rem, 1.5vw, 1.05rem);
  color: rgba(255, 255, 255, 0.95);
  text-transform: uppercase;
  letter-spacing: 0.25em;
  font-weight: 600;
  margin-top: 12px;
  text-shadow: 0 1px 6px rgba(0, 0, 0, 0.4);
}

.hero-haduanh .bottom-line {
  position: absolute;
  bottom: 85px;
  left: 5%;
  right: 5%;
  max-width: 1700px;
  margin: 0 auto;
  border-bottom: 1px solid rgba(255, 255, 255, 0.4);
  pointer-events: none;
}

.hero-haduanh .hero-bottom-bar {
  position: absolute;
  bottom: 28px;
  left: 5%;
  right: 5%;
  max-width: 1700px;
  margin: 0 auto;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  z-index: 10;
}

.hero-haduanh .hero-location-tag {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.25em;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.9);
  line-height: 1.5;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.5);
}

.btn-view-gallery {
  padding: 10px 24px;
  border: 1px solid rgba(255, 255, 255, 0.85);
  background: transparent;
  color: #ffffff;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.22em;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.3s ease;
  border-radius: 2px;
}

.btn-view-gallery:hover {
  background: #ffffff;
  color: #000000;
}

/* COVER TEMPLATE 2: EDITORIAL CENTERED */
.hero-editorial {
  position: relative;
  width: 100%;
  height: 90vh;
  min-height: 540px;
  background-color: #0a0a0a;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: #fff;
  padding: 2rem;
  user-select: none;
}
.hero-editorial-bg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 0.8;
  filter: brightness(0.9);
}
.hero-editorial-overlay {
  position: absolute;
  inset: 0;
  background: linear-gradient(0deg, rgba(0,0,0,0.8) 0%, rgba(0,0,0,0.3) 50%, rgba(0,0,0,0.5) 100%);
}
.hero-editorial-content {
  position: relative;
  z-index: 10;
  max-width: 800px;
}
.hero-editorial-badge {
  display: inline-block;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.3em;
  color: #cbd5e1;
  font-weight: 300;
  margin-bottom: 1rem;
}
.hero-editorial-title {
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-size: clamp(2.5rem, 6vw, 4.5rem);
  font-weight: 300;
  line-height: 1.15;
  margin: 0;
  text-shadow: 0 2px 10px rgba(0,0,0,0.5);
}
.hero-editorial-desc {
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-size: 1.25rem;
  font-style: italic;
  color: #e2e8f0;
  margin-top: 1rem;
}
.hero-editorial-meta {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1.5rem;
  font-size: 0.8rem;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: #cbd5e1;
  margin-top: 1.5rem;
}
.btn-editorial-explore {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 12px 28px;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.18);
  border: 1px solid rgba(255, 255, 255, 0.35);
  backdrop-filter: blur(8px);
  color: #fff;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.2em;
  cursor: pointer;
  transition: all 0.3s ease;
}
.btn-editorial-explore:hover {
  background: rgba(255, 255, 255, 0.32);
}

/* COVER TEMPLATE 3: MINIMALIST */
.hero-minimal {
  width: 85%;
  max-width: 1400px;
  margin: 0 auto;
  padding: 5rem 0 3rem;
  text-align: center;
}
.hero-minimal-header {
  max-width: 750px;
  margin: 0 auto 2rem;
}
.hero-minimal-badge {
  font-size: 0.7rem;
  letter-spacing: 0.3em;
  color: #94a3b8;
  text-transform: uppercase;
  font-weight: 600;
}
.hero-minimal-title {
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-size: clamp(2.5rem, 5vw, 4rem);
  font-weight: 400;
  color: #171717;
  margin: 0.5rem 0;
}
.hero-minimal-client {
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-style: italic;
  font-size: 1.25rem;
  color: #64748b;
}
.hero-minimal-meta {
  font-size: 0.8rem;
  color: #94a3b8;
  margin-top: 0.5rem;
}
.hero-minimal-card {
  max-width: 900px;
  max-height: 600px;
  margin: 0 auto;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.15);
  border: 1px solid #e2e8f0;
}
.hero-minimal-card img {
  width: 100%;
  height: 100%;
  max-height: 600px;
  object-fit: cover;
  display: block;
}
.btn-minimal-scroll {
  padding: 10px 24px;
  border-radius: 9999px;
  background: #171717;
  color: #fff;
  border: none;
  font-size: 0.8rem;
  letter-spacing: 0.05em;
  cursor: pointer;
  transition: opacity 0.2s ease;
}
.btn-minimal-scroll:hover {
  opacity: 0.85;
}

/* COVER TEMPLATE 4: SPLIT MAGAZINE */
.hero-split {
  width: 85%;
  max-width: 1400px;
  margin: 0 auto;
  padding: 4rem 0;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 2.5rem;
  align-items: center;
}
@media (max-width: 768px) {
  .hero-split {
    grid-template-columns: 1fr;
  }
}
.hero-split-left {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.hero-split-badge {
  display: inline-block;
  align-self: flex-start;
  padding: 4px 12px;
  background: #f1f5f9;
  border: 1px solid #e2e8f0;
  border-radius: 9999px;
  font-size: 0.75rem;
  letter-spacing: 0.1em;
  color: #475569;
}
.hero-split-title {
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-size: clamp(2.5rem, 5vw, 4rem);
  font-weight: 400;
  line-height: 1.15;
  color: #171717;
  margin: 0;
}
.hero-split-desc {
  font-size: 0.95rem;
  color: #64748b;
  line-height: 1.6;
}
.hero-split-meta {
  font-size: 0.85rem;
  color: #64748b;
  border-top: 1px solid #e2e8f0;
  padding-top: 1rem;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.btn-split-scroll {
  align-self: flex-start;
  padding: 12px 28px;
  border-radius: 4px;
  background: #171717;
  color: #fff;
  border: none;
  font-size: 0.8rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  cursor: pointer;
  transition: opacity 0.2s;
}
.hero-split-right {
  display: flex;
  justify-content: center;
}
.hero-split-imgwrap {
  width: 100%;
  aspect-ratio: 4/5;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 20px 40px -10px rgba(0, 0, 0, 0.15);
  border: 1px solid #e2e8f0;
}
.hero-split-imgwrap img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

/* 2. STICKY NAVBAR */
.gallery-nav {
  position: sticky;
  top: 0;
  z-index: 40;
  background-color: rgba(255, 253, 248, 0.95);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-bottom: 1px solid rgba(232, 229, 222, 0.85);
  transition: all 0.2s ease;
}

.fe-album-root.dark .gallery-nav,
.fe-album-root.dark .gallery-nav,
[data-theme='dark'] .gallery-nav {
  background-color: rgba(14, 14, 13, 0.95);
  border-bottom-color: rgba(41, 41, 39, 0.85);
}

.gallery-nav-inner {
  width: 92%;
  max-width: 1700px;
  margin: 0 auto;
  padding: 10px 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.gallery-nav-left {
  display: flex;
  align-items: center;
  gap: 20px;
  min-width: 0;
  flex: 1;
}

.gallery-nav-title-group {
  flex-shrink: 0;
  min-width: 0;
}

.gallery-nav-title {
  font-family: 'Plus Jakarta Sans', 'Inter', sans-serif;
  font-size: 1.15rem;
  font-weight: 700;
  color: #171717;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin: 0;
  letter-spacing: -0.01em;
}

.fe-album-root.dark .gallery-nav-title,
.fe-album-root.dark .gallery-nav-title,
[data-theme='dark'] .gallery-nav-title {
  color: #F5F5F4;
}

.gallery-nav-meta {
  font-size: 11.5px;
  color: #71717a;
  margin-top: 1px;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.gallery-nav-fav-count {
  color: #FFC629;
  font-weight: 600;
}

.gallery-nav-select-count {
  color: #eab308;
  font-weight: 600;
}

.gallery-nav-ai-count {
  color: #ca8a04;
  font-weight: 600;
}

.gallery-nav-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

/* Filter Pills */
.filter-pills {
  display: flex;
  align-items: center;
  background-color: #f4f1ea;
  padding: 3px;
  border-radius: 9999px;
  border: 1px solid #e8e5de;
  font-size: 12px;
}

.fe-album-root.dark .filter-pills,
.fe-album-root.dark .filter-pills,
[data-theme='dark'] .filter-pills {
  background-color: #1c1c1b;
  border-color: #2e2e2c;
}

.filter-btn {
  padding: 6px 14px;
  border-radius: 9999px;
  border: none;
  background: transparent;
  color: #686868;
  font-weight: 600;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.fe-album-root.dark .filter-btn,
.fe-album-root.dark .filter-btn,
[data-theme='dark'] .filter-btn {
  color: #a3a3a0;
}

.filter-btn:hover {
  color: #171717;
}

.fe-album-root.dark .filter-btn:hover,
.fe-album-root.dark .filter-btn:hover {
  color: #ffffff;
}

.filter-btn.active {
  background-color: #171717;
  color: #ffffff;
  box-shadow: 0 1px 3px rgba(0,0,0,0.12);
}

.fe-album-root.dark .filter-btn.active,
.fe-album-root.dark .filter-btn.active {
  background-color: #ffffff;
  color: #171717;
}

.filter-btn--fav.active {
  background: #171717 !important;
  color: #ffffff !important;
}

.filter-fav-badge {
  background: rgba(255, 198, 41, 0.2);
  color: #FFC629;
  font-size: 11px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 9999px;
}

.filter-btn--fav.active .filter-fav-badge {
  background: rgba(255, 255, 255, 0.25);
  color: #ffffff;
}

.filter-btn--ai.active {
  background: #FFC629 !important;
  color: #171717 !important;
  box-shadow: 0 2px 8px rgba(255, 198, 41, 0.35) !important;
}

/* Nav Action Buttons: Select & "..." */
.nav-btn-select {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 9999px;
  border: 1px solid #dcd7ce;
  background: #ffffff;
  color: #171717;
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}

.fe-album-root.dark .nav-btn-select,
.fe-album-root.dark .nav-btn-select {
  background: #181817;
  border-color: #292927;
  color: #f5f5f4;
}

.nav-btn-select:hover {
  background: #f7f4ed;
  border-color: #c9c4b9;
}

.fe-album-root.dark .nav-btn-select:hover,
.fe-album-root.dark .nav-btn-select:hover {
  background: #222220;
}

.nav-selected-badge {
  font-size: 12px;
  font-weight: 700;
  padding: 6px 12px;
  border-radius: 9999px;
  background: #FFC629;
  color: #171717;
}

.nav-btn-done {
  padding: 7px 18px;
  border-radius: 9999px;
  border: none;
  background: #171717;
  color: #ffffff;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s ease;
}

.fe-album-root.dark .nav-btn-done,
.fe-album-root.dark .nav-btn-done {
  background: #ffffff;
  color: #171717;
}

.nav-btn-done:hover {
  opacity: 0.9;
  transform: scale(0.98);
}

/* Navigation More Dropdown Menu ("...") */
.nav-more-wrapper {
  position: relative;
}

.nav-btn-more {
  width: 38px;
  height: 38px;
  border-radius: 50%;
  border: 1px solid #dcd7ce;
  background: #ffffff;
  color: #52525b;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}

.fe-album-root.dark .nav-btn-more,
.fe-album-root.dark .nav-btn-more {
  background: #181817;
  border-color: #292927;
  color: #a3a3a0;
}

.nav-btn-more:hover,
.nav-btn-more.active {
  background: #f7f4ed;
  color: #171717;
  border-color: #c9c4b9;
}

.fe-album-root.dark .nav-btn-more:hover,
.fe-album-root.dark .nav-btn-more.active,
.fe-album-root.dark .nav-btn-more:hover,
.fe-album-root.dark .nav-btn-more.active {
  background: #222220;
  color: #ffffff;
}

.nav-more-dropdown {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  width: 270px;
  background: #ffffff;
  border: 1px solid #e8e5de;
  border-radius: 18px;
  box-shadow: 0 20px 40px -10px rgba(0, 0, 0, 0.16), 0 0 0 1px rgba(0, 0, 0, 0.04);
  padding: 6px;
  z-index: 100;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.fe-album-root.dark .nav-more-dropdown,
.fe-album-root.dark .nav-more-dropdown {
  background: #181817;
  border-color: #292927;
  box-shadow: 0 20px 40px -10px rgba(0, 0, 0, 0.55), 0 0 0 1px rgba(255, 255, 255, 0.06);
}

.dropdown-item {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 9px 12px;
  border-radius: 10px;
  border: none;
  background: transparent;
  color: #27272a;
  font-size: 13px;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
  transition: all 0.15s ease;
  box-sizing: border-box;
}

.fe-album-root.dark .dropdown-item,
.fe-album-root.dark .dropdown-item {
  color: #e4e4e7;
}

.dropdown-item:hover {
  background: #f7f4ed;
  color: #09090b;
}

.fe-album-root.dark .dropdown-item:hover,
.fe-album-root.dark .dropdown-item:hover {
  background: #222220;
  color: #ffffff;
}

.dropdown-item-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.dropdown-badge {
  font-size: 11px;
  color: #a1a1aa;
}

.dropdown-item--fav {
  color: inherit !important;
}

.dropdown-item--photographer {
  color: #b45309 !important;
  font-weight: 600;
}

.fe-album-root.dark .dropdown-item--photographer,
.fe-album-root.dark .dropdown-item--photographer {
  color: #FFC629 !important;
}

.dropdown-divider {
  height: 1px;
  background: #f0ede6;
  margin: 4px 6px;
}

.fe-album-root.dark .dropdown-divider,
.fe-album-root.dark .dropdown-divider {
  background: #292927;
}

.dropdown-section-title {
  font-size: 10.5px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: #a1a1aa;
  padding: 4px 12px 2px;
}

.dropdown-config-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 5px 12px;
  font-size: 12.5px;
}

.dropdown-config-label {
  color: #71717a;
  font-weight: 500;
}

.dropdown-toggle-group {
  display: flex;
  background: #f4f1ea;
  padding: 2px;
  border-radius: 8px;
  gap: 2px;
}

.fe-album-root.dark .dropdown-toggle-group,
.fe-album-root.dark .dropdown-toggle-group {
  background: #222220;
}

.dropdown-toggle-btn {
  border: none;
  background: transparent;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  color: #71717a;
  cursor: pointer;
  transition: all 0.15s ease;
}

.dropdown-toggle-btn.active {
  background: #ffffff;
  color: #18181b;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.fe-album-root.dark .dropdown-toggle-btn.active,
.fe-album-root.dark .dropdown-toggle-btn.active {
  background: #383835;
  color: #ffffff;
}

.dropdown-theme-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-radius: 10px;
  cursor: pointer;
  user-select: none;
  font-size: 12.5px;
  color: #52525b;
}

.dropdown-theme-toggle:hover {
  background: #f7f4ed;
}

.fe-album-root.dark .dropdown-theme-toggle:hover,
.fe-album-root.dark .dropdown-theme-toggle:hover {
  background: #222220;
}

.dropdown-theme-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dropdown-theme-switch {
  width: 36px;
  height: 20px;
  background: #e4e4e7;
  border-radius: 20px;
  position: relative;
  transition: background-color 0.2s ease;
}

.dropdown-theme-switch.on {
  background: #FFC629;
}

.dropdown-theme-knob {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  transition: transform 0.2s ease;
}

.dropdown-theme-switch.on .dropdown-theme-knob {
  transform: translateX(16px);
}

/* AI Face Search Hero Banner (AIFS Luxury Design) */
.gallery-face-hero-container {
  width: 92%;
  max-width: 1700px;
  margin: 20px auto 0;
}

.aifs-face-hero-card {
  border-radius: 24px;
  border: 2px solid rgba(255, 198, 41, 0.45);
  background: linear-gradient(135deg, rgba(255, 198, 41, 0.12) 0%, rgba(255, 198, 41, 0.04) 50%, rgba(255, 255, 255, 0.6) 100%);
  padding: 22px 26px;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 10px 30px -10px rgba(255, 198, 41, 0.15);
  transition: all 0.25s ease;
}

.fe-album-root.dark .aifs-face-hero-card,
.fe-album-root.dark .aifs-face-hero-card,
[data-theme='dark'] .aifs-face-hero-card {
  background: linear-gradient(135deg, rgba(255, 198, 41, 0.1) 0%, rgba(255, 198, 41, 0.02) 50%, rgba(24, 24, 23, 0.8) 100%);
  border-color: rgba(255, 198, 41, 0.35);
  box-shadow: 0 10px 30px -10px rgba(0, 0, 0, 0.5);
}

.aifs-face-hero-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
}

.aifs-face-hero-info {
  display: flex;
  align-items: center;
  gap: 16px;
}

.aifs-face-hero-icon-ring {
  width: 50px;
  height: 50px;
  border-radius: 16px;
  background: #FFC629;
  color: #171717;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 14px rgba(255, 198, 41, 0.35);
  flex-shrink: 0;
}

.aifs-face-hero-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.aifs-face-hero-title {
  font-family: 'Plus Jakarta Sans', 'Inter', sans-serif;
  font-size: 17px;
  font-weight: 700;
  color: #171717;
  margin: 0;
  letter-spacing: -0.01em;
}

.fe-album-root.dark .aifs-face-hero-title,
.fe-album-root.dark .aifs-face-hero-title {
  color: #f5f5f4;
}

.aifs-face-hero-badge {
  padding: 2px 8px;
  border-radius: 9999px;
  background: rgba(255, 198, 41, 0.25);
  border: 1px solid rgba(255, 198, 41, 0.4);
  color: #b45309;
  font-size: 10px;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.fe-album-root.dark .aifs-face-hero-badge,
.fe-album-root.dark .aifs-face-hero-badge {
  color: #FFC629;
}

.aifs-face-hero-desc {
  font-size: 12.5px;
  color: #686868;
  margin: 3px 0 0;
}

.fe-album-root.dark .aifs-face-hero-desc,
.fe-album-root.dark .aifs-face-hero-desc {
  color: #a3a3a0;
}

.btn-aifs-selfie-cta {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 11px 22px;
  border-radius: 14px;
  background: linear-gradient(135deg, #FFC629 0%, #F2B91B 100%);
  color: #171717;
  font-size: 12.5px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border: none;
  cursor: pointer;
  box-shadow: 0 4px 16px rgba(255, 198, 41, 0.35);
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.btn-aifs-selfie-cta:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(255, 198, 41, 0.45);
  background: linear-gradient(135deg, #ffcf4d 0%, #f7c331 100%);
}

.btn-aifs-selfie-cta:active {
  transform: scale(0.97);
}

.aifs-face-hero-people {
  margin-top: 18px;
  padding-top: 16px;
  border-top: 1px solid rgba(255, 198, 41, 0.25);
}

.fe-album-root.dark .aifs-face-hero-people,
.fe-album-root.dark .aifs-face-hero-people {
  border-top-color: rgba(255, 255, 255, 0.08);
}

.aifs-face-hero-people-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.aifs-face-hero-people-label {
  font-size: 11.5px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: #475569;
}

.fe-album-root.dark .aifs-face-hero-people-label,
.fe-album-root.dark .aifs-face-hero-people-label {
  color: #cbd5e1;
}

.btn-aifs-clear-link {
  font-size: 11.5px;
  font-weight: 600;
  color: #b45309;
  background: none;
  border: none;
  cursor: pointer;
  text-decoration: underline;
}

.fe-album-root.dark .btn-aifs-clear-link,
.fe-album-root.dark .btn-aifs-clear-link {
  color: #FFC629;
}

/* Active Face Filter Banner */
.aifs-active-face-banner {
  width: 92%;
  max-width: 1700px;
  margin: 16px auto 0;
  border-radius: 18px;
  border: 2px solid rgba(255, 198, 41, 0.5);
  background: rgba(255, 198, 41, 0.12);
  padding: 12px 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.fe-album-root.dark .aifs-active-face-banner,
.fe-album-root.dark .aifs-active-face-banner {
  background: rgba(255, 198, 41, 0.08);
  border-color: rgba(255, 198, 41, 0.4);
}

.active-face-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.active-face-thumb {
  width: 46px;
  height: 46px;
  border-radius: 50%;
  object-fit: cover;
  border: 2px solid #FFC629;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  flex-shrink: 0;
}

.active-face-thumb-placeholder {
  width: 46px;
  height: 46px;
  border-radius: 50%;
  background: #FFC629;
  color: #171717;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.active-face-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.active-face-title {
  font-family: 'Plus Jakarta Sans', sans-serif;
  font-size: 14px;
  font-weight: 700;
  color: #171717;
}

.fe-album-root.dark .active-face-title,
.fe-album-root.dark .active-face-title {
  color: #f5f5f4;
}

.active-face-badge {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 9999px;
  background: #FFC629;
  color: #171717;
}

.active-face-sub {
  font-size: 11.5px;
  color: #686868;
  margin: 2px 0 0;
}

.fe-album-root.dark .active-face-sub,
.fe-album-root.dark .active-face-sub {
  color: #a3a3a0;
}

.active-face-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.btn-aifs-download-active {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 12px;
  background: #FFC629;
  color: #171717;
  font-size: 12px;
  font-weight: 700;
  border: none;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 2px 8px rgba(255, 198, 41, 0.3);
}

.btn-aifs-download-active:hover {
  filter: brightness(1.08);
  transform: translateY(-1px);
}

.btn-aifs-clear-active {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.08);
  color: #171717;
  font-size: 12px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  transition: all 0.2s ease;
}

.fe-album-root.dark .btn-aifs-clear-active,
.fe-album-root.dark .btn-aifs-clear-active {
  background: rgba(255, 255, 255, 0.1);
  color: #f5f5f4;
}

.btn-aifs-clear-active:hover {
  background: rgba(0, 0, 0, 0.15);
}

.fe-album-root.dark .btn-aifs-clear-active:hover,
.fe-album-root.dark .btn-aifs-clear-active:hover {
  background: rgba(255, 255, 255, 0.18);
}

/* 3. MASONRY GRID (CSS MULTI-COLUMN) */
.gallery-main {
  width: 88%;
  max-width: 1600px;
  margin: 0 auto;
  padding: 32px 0 80px;
}

.gallery-favorites-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 18px;
  margin-bottom: 24px;
  background: #F8F6F0;
  border: 1px solid #E8E5DE;
  border-radius: 14px;
}

.favorites-header-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.favorites-header-icon {
  font-size: 16px;
}

.favorites-header-text {
  font-size: 13.5px;
  font-weight: 600;
  color: #FFC629;
}

.btn-fav-export-csv {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 9999px;
  background: #10b981;
  color: #ffffff;
  border: none;
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(16, 185, 129, 0.3);
  transition: all 0.2s ease;
}

.btn-fav-export-csv:hover {
  background: #059669;
  transform: translateY(-1px);
}

.masonry-columns {
  column-count: 4;
  column-gap: 16px;
}

@media (max-width: 1200px) {
  .masonry-columns {
    column-count: 3;
  }
}

/* Photo Card & Framing */
.photo-card {
  break-inside: avoid;
  position: relative;
  margin-bottom: 16px;
  border-radius: 14px;
  overflow: hidden;
  cursor: pointer;
  transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.photo-card-frame {
  position: relative;
  width: 100%;
  height: 100%;
  border-radius: 14px;
  overflow: hidden;
  background-color: #e2e8f0;
  border: 1px solid rgba(0, 0, 0, 0.08);
  box-shadow: 0 4px 14px -2px rgba(0, 0, 0, 0.06), 0 2px 6px -1px rgba(0, 0, 0, 0.04);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.fe-album-root.dark .photo-card-frame,
[data-theme='dark'] .photo-card-frame {
  background-color: #18181b;
  border-color: rgba(255, 255, 255, 0.12);
  box-shadow: 0 4px 18px -2px rgba(0, 0, 0, 0.4);
}

.photo-card:hover .photo-card-frame {
  transform: translateY(-3px);
  border-color: rgba(139, 92, 246, 0.4);
  box-shadow: 0 16px 32px -4px rgba(0, 0, 0, 0.16), 0 4px 10px rgba(0, 0, 0, 0.06);
}

.photo-card.is-selected .photo-card-frame {
  border-color: #3b82f6 !important;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.4), 0 12px 28px rgba(59, 130, 246, 0.3) !important;
}

.photo-card-img {
  width: 100%;
  height: auto;
  display: block;
  object-fit: cover;
  transition: transform 0.5s cubic-bezier(0.16, 1, 0.3, 1);
}

.photo-card:hover .photo-card-img {
  transform: scale(1.03);
}

/* Subtle Vignette Scrim */
.photo-card-scrim {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.65) 0%, rgba(0, 0, 0, 0.1) 32%, transparent 60%);
  opacity: 0.6;
  transition: opacity 0.25s ease;
}

.photo-card:hover .photo-card-scrim {
  opacity: 1;
}

/* Bottom info badge */
.photo-card-info-badge {
  position: absolute;
  bottom: 8px;
  left: 8px;
  right: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 4px 8px;
  border-radius: 6px;
  background: rgba(15, 23, 42, 0.72);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  opacity: 0;
  transform: translateY(4px);
  transition: all 0.25s ease;
  pointer-events: none;
  z-index: 8;
}

.photo-card:hover .photo-card-info-badge {
  opacity: 1;
  transform: translateY(0);
}

.photo-card-info-name {
  font-size: 11px;
  font-weight: 500;
  color: #ffffff;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.photo-card-info-mp {
  font-size: 10.5px;
  font-family: monospace;
  color: #FFC629;
  font-weight: 600;
  flex-shrink: 0;
}

/* Quick Download Button */
.btn-download-quick {
  position: absolute;
  top: 10px;
  right: 54px;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgba(15, 23, 42, 0.55);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.18);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 12;
  opacity: 0;
  transform: translateY(-2px);
  transition: all 0.2s ease;
  box-shadow: 0 2px 8px rgba(0,0,0,0.3);
}

.photo-card:hover .btn-download-quick {
  opacity: 1;
  transform: translateY(0);
}

.btn-download-quick:hover {
  background: rgba(15, 23, 42, 0.85);
  transform: scale(1.1);
  border-color: rgba(255, 255, 255, 0.35);
}

/* Heart Button */
.btn-heart {
  position: absolute;
  top: 10px;
  right: 10px;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgba(15, 23, 42, 0.55);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.18);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 12;
  transition: all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  box-shadow: 0 2px 8px rgba(0,0,0,0.3);
}

.btn-heart:hover {
  transform: scale(1.15);
  background: rgba(15, 23, 42, 0.85);
  border-color: rgba(255, 255, 255, 0.35);
}

.btn-heart:active {
  transform: scale(0.9);
}

.btn-heart.is-favorited {
  background: rgba(255, 255, 255, 0.25);
  border-color: rgba(255, 255, 255, 0.6);
}

.photo-heart-shadow {
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.6));
}

.photo-action-shadow {
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.6));
}

.gallery-empty-state {
  text-align: center;
  padding: 80px 0;
  color: #94a3b8;
}
.gallery-empty-state p {
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-size: 1.25rem;
  margin: 12px 0 20px;
}
.btn-reset-filter {
  padding: 8px 20px;
  border-radius: 9999px;
  border: 1px solid #cbd5e1;
  background: transparent;
  color: #475569;
  font-size: 0.85rem;
  cursor: pointer;
}

/* FOOTER */
.gallery-footer {
  padding: 48px 0;
  border-top: 1px solid #e5e5e5;
  text-align: center;
  font-size: 12px;
  color: #94a3b8;
  font-weight: 300;
}

/* 4. LIGHTBOX */
.lightbox-modal {
  position: fixed;
  inset: 0;
  z-index: 99999;
  background-color: rgba(0, 0, 0, 0.95);
  backdrop-filter: blur(8px);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  color: #ffffff;
  user-select: none;
}

.lightbox-topbar {
  padding: 16px 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}
.lightbox-counter {
  font-size: 12px;
  color: #94a3b8;
  font-family: monospace;
}
.lightbox-actions {
  display: flex;
  align-items: center;
  gap: 16px;
}
.lightbox-btn {
  background: transparent;
  border: none;
  color: #cbd5e1;
  cursor: pointer;
  padding: 8px 12px;
  border-radius: 9999px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  text-decoration: none;
  transition: all 0.2s ease;
}
.lightbox-btn:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.15);
}

.lightbox-center {
  flex: 1;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  min-height: 0;
}
.lightbox-center img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.lightbox-nav-btn {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  background: rgba(0, 0, 0, 0.5);
  border: none;
  color: #ffffff;
  width: 48px;
  height: 48px;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  backdrop-filter: blur(4px);
  transition: background 0.2s ease;
}
.lightbox-nav-btn:hover {
  background: rgba(0, 0, 0, 0.85);
}
.lightbox-nav-btn.left { left: 24px; }
.lightbox-nav-btn.right { right: 24px; }

.lightbox-bottombar {
  padding: 12px 24px;
  text-align: center;
  font-size: 12px;
  color: #94a3b8;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

/* Toast */
.fe-album-toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  background: #171717;
  color: #ffffff;
  padding: 10px 18px;
  border-radius: 6px;
  font-size: 13px;
  box-shadow: 0 10px 25px rgba(0,0,0,0.25);
  z-index: 100000;
}

.fe-album-fade-enter-active,
.fe-album-fade-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}
.fe-album-fade-enter-from,
.fe-album-fade-leave-to {
  opacity: 0;
  transform: scale(0.98);
}

/* AI Button & Filter styles - Unified Luxury Gold & Obsidian */
.nav-action-btn--ai {
  background: linear-gradient(135deg, #FFC629 0%, #F2B91B 100%) !important;
  color: #171717 !important;
  border: none !important;
  box-shadow: 0 2px 8px rgba(255, 198, 41, 0.35);
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: all 0.2s ease;
}
.nav-action-btn--ai:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 14px rgba(255, 198, 41, 0.45);
}

.filter-btn--ai {
  border-color: rgba(255, 198, 41, 0.5) !important;
  color: #171717 !important;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.fe-album-root.dark .filter-btn--ai,
html.dark .filter-btn--ai {
  color: #FFC629 !important;
  border-color: rgba(255, 198, 41, 0.4) !important;
}

.filter-btn--ai.active {
  background: #FFC629 !important;
  color: #171717 !important;
  font-weight: 700;
}

.ai-match-badge {
  position: absolute;
  top: 10px;
  left: 10px;
  background: #FFC629;
  color: #171717;
  font-size: 11px;
  font-weight: 700;
  padding: 4px 8px;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  gap: 4px;
  z-index: 2;
  box-shadow: 0 2px 6px rgba(0,0,0,0.25);
}

/* AI Face Search Modal */
.ai-modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.75);
  backdrop-filter: blur(6px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 99999;
  padding: 16px;
}
.ai-modal-card {
  background: #ffffff;
  border-radius: 16px;
  width: 100%;
  max-width: 480px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.3);
  overflow: hidden;
  animation: modalPop 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes modalPop {
  from { opacity: 0; transform: scale(0.95); }
  to { opacity: 1; transform: scale(1); }
}
.ai-modal-header {
  padding: 20px 24px;
  border-bottom: 1px solid #f1f5f9;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.ai-modal-title {
  font-size: 17px;
  font-weight: 600;
  color: #0f172a;
  display: flex;
  align-items: center;
  gap: 8px;
}
.ai-modal-close {
  background: transparent;
  border: none;
  cursor: pointer;
  color: #64748b;
  padding: 4px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.ai-modal-close:hover {
  background: #f1f5f9;
  color: #0f172a;
}
.ai-modal-body {
  padding: 24px;
}
.ai-dropzone {
  border: 2px dashed #cbd5e1;
  border-radius: 12px;
  padding: 28px 20px;
  text-align: center;
  cursor: pointer;
  transition: border-color 0.2s, background 0.2s;
  background: #f8fafc;
}
.ai-dropzone:hover {
  border-color: #FFC629;
  background: rgba(255, 198, 41, 0.1);
}
.ai-preview-box {
  position: relative;
  width: 160px;
  height: 160px;
  margin: 0 auto 16px;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
}
.ai-preview-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.ai-btn-primary {
  width: 100%;
  background: linear-gradient(135deg, #FFC629 0%, #F2B91B 100%); color: #171717;
  color: #fff;
  border: none;
  border-radius: 8px;
  padding: 12px;
  font-size: 14.5px;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  transition: opacity 0.2s;
}
.ai-btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.ai-spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-radius: 50%;
  border-top-color: #fff;
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Photo Selection & Checkbox */
.photo-select-checkbox {
  position: absolute;
  top: 10px;
  left: 10px;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border: 2px solid #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 5;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.35);
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.photo-select-checkbox:hover {
  transform: scale(1.1);
  border-color: #60a5fa;
}

.photo-select-checkbox.is-checked {
  background: #3b82f6;
  border-color: #ffffff;
  box-shadow: 0 0 10px rgba(59, 130, 246, 0.7);
}

.photo-card.is-selected {
  outline: 3px solid #3b82f6;
  outline-offset: -3px;
}

/* Floating Selection Bar */
.gallery-selection-bar {
  position: fixed;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  background: rgba(18, 18, 22, 0.94);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 9999px;
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.65), 0 0 0 1px rgba(255, 255, 255, 0.08);
  color: #ffffff;
  max-width: 92vw;
}

.selection-bar-left {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-right: 6px;
  border-right: 1px solid rgba(255, 255, 255, 0.15);
}

.selection-count {
  font-size: 13px;
  font-weight: 700;
  color: #60a5fa;
  font-family: monospace;
  white-space: nowrap;
}

.selection-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-selection-action {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  font-size: 12.5px;
  font-weight: 600;
  border-radius: 9999px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: rgba(255, 255, 255, 0.08);
  color: #ffffff;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.18s cubic-bezier(0.16, 1, 0.3, 1);
}

.btn-selection-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.18);
  transform: translateY(-1px);
}

.btn-selection-action:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.btn-selection-action--fav {
  background: rgba(255, 255, 255, 0.15);
  border-color: rgba(255, 255, 255, 0.3);
  color: #ffffff;
}

.btn-selection-action--fav:hover:not(:disabled) {
  background: rgba(255, 198, 41, 0.25);
  border-color: #FFC629;
  color: #ffffff;
  box-shadow: 0 2px 10px rgba(255, 198, 41, 0.35);
}

.btn-selection-action--csv {
  background: rgba(16, 185, 129, 0.18);
  border-color: rgba(16, 185, 129, 0.4);
  color: #6ee7b7;
}

.btn-selection-action--csv:hover:not(:disabled) {
  background: #10b981;
  border-color: #10b981;
  color: #ffffff;
  box-shadow: 0 2px 10px rgba(16, 185, 129, 0.35);
}

.btn-selection-action--primary {
  background: linear-gradient(135deg, #FFC629 0%, #F2B91B 100%);
  border-color: #FFC629;
  color: #171717;
  font-weight: 700;
  box-shadow: 0 2px 10px rgba(255, 198, 41, 0.4);
}

.btn-selection-action--primary:hover:not(:disabled) {
  background: linear-gradient(135deg, #FFD04D 0%, #FFC629 100%);
  box-shadow: 0 4px 14px rgba(255, 198, 41, 0.55);
}

.btn-selection-action--close {
  padding: 6px;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.15);
  color: #94a3b8;
}

.btn-selection-action--close:hover {
  background: rgba(255, 255, 255, 0.25);
  color: #ffffff;
}

/* Lightbox Stage & Face Frames */
.lightbox-stage {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  max-width: 100%;
  max-height: 100%;
  line-height: 0;
}

.lightbox-main-img {
  max-width: calc(100vw - 120px);
  max-height: calc(100vh - 140px);
  width: auto;
  height: auto;
  object-fit: contain;
  display: block;
  user-select: none;
  -webkit-user-drag: none;
  border-radius: 4px;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.7);
}

.lightbox-face-overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.lightbox-face-frame {
  position: absolute;
  pointer-events: auto;
  border: 2px solid rgba(255, 198, 41, 0.9);
  background: rgba(255, 198, 41, 0.15);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.4), 0 0 16px rgba(255, 198, 41, 0.35);
}

.lightbox-face-frame:hover {
  border-color: #FFC629;
  background: rgba(56, 189, 248, 0.28);
  transform: scale(1.04);
  box-shadow: 0 0 0 2px #38bdf8, 0 0 24px rgba(56, 189, 248, 0.6);
  z-index: 10;
}

.face-frame-corner {
  position: absolute;
  width: 6px;
  height: 6px;
  border-color: #ffffff;
  border-style: solid;
  pointer-events: none;
}
.face-frame-corner.tl {
  top: -2px;
  left: -2px;
  border-width: 2px 0 0 2px;
  border-top-left-radius: 3px;
}
.face-frame-corner.tr {
  top: -2px;
  right: -2px;
  border-width: 2px 2px 0 0;
  border-top-right-radius: 3px;
}
.face-frame-corner.bl {
  bottom: -2px;
  left: -2px;
  border-width: 0 0 2px 2px;
  border-bottom-left-radius: 3px;
}
.face-frame-corner.br {
  bottom: -2px;
  right: -2px;
  border-width: 0 2px 2px 0;
  border-bottom-right-radius: 3px;
}

.face-frame-chip {
  position: absolute;
  top: -24px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(15, 23, 42, 0.92);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  color: #FFC629;
  border: 1px solid rgba(56, 189, 248, 0.4);
  font-size: 10.5px;
  font-weight: 700;
  padding: 1px 8px;
  border-radius: 9999px;
  white-space: nowrap;
  pointer-events: none;
  letter-spacing: 0.03em;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
}

.nav-action-btn--ios {
  background: #0284c7;
  color: #ffffff;
}

.nav-action-btn--ios:hover {
  background: #0369a1;
}

.nav-action-btn--active {
  background: #3b82f6 !important;
  color: #ffffff !important;
}

.lightbox-info-details {
  font-size: 11.5px;
  opacity: 0.75;
  font-family: monospace;
  margin-top: 2px;
}

.lightbox-btn.is-active {
  color: #3b82f6;
}

/* View Density & Aspect Ratio Controls */
.view-density-group,
.view-aspect-group {
  display: flex;
  background-color: #f1f5f9;
  padding: 3px;
  border-radius: 9999px;
  border: 1px solid #e2e8f0;
}

.density-btn {
  padding: 5px 10px;
  border-radius: 9999px;
  border: none;
  background: transparent;
  color: #64748b;
  font-size: 11.5px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.density-btn:hover {
  color: #1e293b;
}

.density-btn.active {
  background-color: #ffffff;
  color: #0f172a;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

/* Grid View Modifiers */
.masonry-columns.view-large {
  column-count: 3;
}
.masonry-columns.view-compact {
  column-count: 5;
}
@media (max-width: 1200px) {
  .masonry-columns.view-large {
    column-count: 2;
  }
  .masonry-columns.view-compact {
    column-count: 4;
  }
}
@media (max-width: 768px) {
  .masonry-columns {
    column-count: 2 !important;
    column-gap: 10px;
  }
  .masonry-columns.view-large {
    column-count: 1 !important;
  }
  .masonry-columns.view-compact {
    column-count: 2 !important;
    column-gap: 8px;
  }
}
@media (max-width: 480px) {
  .masonry-columns {
    column-count: 2 !important;
    column-gap: 8px;
  }
  .masonry-columns.view-compact {
    column-count: 2 !important;
  }
}

.masonry-columns.aspect-square .photo-card img,
.photo-card.card-square img {
  aspect-ratio: 1 / 1;
  object-fit: cover;
}

/* People Container */
.gallery-people-container {
  width: 85%;
  max-width: 1600px;
  margin: 16px auto 0 auto;
}

/* Active Face Filter Banner */
.gallery-active-filter-banner {
  width: 85%;
  max-width: 1600px;
  margin: 12px auto 0 auto;
  padding: 10px 16px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.active-filter-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.active-filter-indicator {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
}

.active-filter-text {
  font-size: 13px;
  font-weight: 500;
  color: #1e293b;
}

.btn-clear-face-filter {
  padding: 4px 12px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 9999px;
  border: 1px solid #cbd5e1;
  background: #ffffff;
  color: #475569;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-clear-face-filter:hover {
  background: #f1f5f9;
  color: #0f172a;
}

/* Lightbox EXIF Drawer */
.lightbox-exif-drawer {
  position: absolute;
  right: 16px;
  top: 65px;
  bottom: 65px;
  width: 360px;
  max-width: calc(100vw - 32px);
  z-index: 100;
  background: rgba(15, 15, 18, 0.94);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 16px;
  padding: 20px;
  overflow-y: auto;
  box-shadow: 0 20px 45px rgba(0, 0, 0, 0.6);
  color: #e2e8f0;
  display: flex;
  flex-direction: column;
}

.lightbox-exif-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  margin-bottom: 16px;
}

.lightbox-exif-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14.5px;
  font-weight: 600;
  color: #ffffff;
}

.btn-close-drawer {
  background: transparent;
  border: none;
  color: #94a3b8;
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s ease, background-color 0.15s ease;
}

.btn-close-drawer:hover {
  color: #ffffff;
  background-color: rgba(255, 255, 255, 0.1);
}

.lightbox-exif-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.exif-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-bottom: 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.exif-section:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.exif-section-title {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #94a3b8;
}

.exif-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  font-size: 12.5px;
}

.exif-label {
  color: #94a3b8;
  flex-shrink: 0;
}

.exif-value {
  color: #f1f5f9;
  font-weight: 500;
  text-align: right;
  word-break: break-word;
}

.filename-wrap {
  font-family: monospace;
  font-size: 11.5px;
}

.exif-params-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
}

.exif-param-box {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 8px 10px;
}

.exif-param-name {
  display: block;
  font-size: 10px;
  text-transform: uppercase;
  color: #94a3b8;
  letter-spacing: 0.05em;
  margin-bottom: 2px;
}

.exif-param-val {
  display: block;
  font-family: monospace;
  font-size: 13px;
  font-weight: 600;
  color: #ffffff;
}

.exif-faces-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.exif-face-chip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 10px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.exif-face-chip:hover {
  background: rgba(255, 255, 255, 0.12);
  border-color: #3b82f6;
}

.exif-face-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.exif-face-label {
  font-size: 12px;
  font-weight: 500;
  color: #ffffff;
  flex: 1;
}

.exif-face-action {
  font-size: 11px;
  color: #60a5fa;
  font-weight: 600;
}

/* Slide Right Transition */
.fe-slide-right-enter-active,
.fe-slide-right-leave-active {
  transition: all 0.22s cubic-bezier(0.16, 1, 0.3, 1);
}

.fe-slide-right-enter-from,
.fe-slide-right-leave-to {
  opacity: 0;
  transform: translateX(20px);
}

/* AI Face Indexing Banner */
.gallery-indexing-banner {
  margin: 1.25rem 2rem 0;
  padding: 1rem 1.25rem;
  background: rgba(15, 23, 42, 0.85);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 12px;
  backdrop-filter: blur(12px);
  color: #fff;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.25);
}
.indexing-banner-inner {
  display: flex;
  align-items: center;
  gap: 1rem;
}
.indexing-banner-spinner {
  width: 20px;
  height: 20px;
  border: 2px solid rgba(255, 255, 255, 0.2);
  border-top-color: #FFC629;
  border-radius: 50%;
  animation: fe-spin 0.8s linear infinite;
  flex-shrink: 0;
}
.indexing-banner-text {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.indexing-banner-heading {
  font-size: 0.875rem;
  font-weight: 600;
  letter-spacing: 0.02em;
}
.indexing-banner-sub {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.7);
}
.indexing-banner-percent {
  font-size: 0.925rem;
  font-weight: 700;
  color: #FFC629;
}
.indexing-banner-bar {
  margin-top: 0.65rem;
  height: 4px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 2px;
  overflow: hidden;
}
.indexing-banner-bar-fill {
  height: 100%;
  background: linear-gradient(90deg, #38bdf8, #818cf8);
  transition: width 0.3s ease;
}
@keyframes fe-spin {
  to {
    transform: rotate(360deg);
  }
}

/* Floating Quick Info Pill on Desktop (Bottom Left) */
.lightbox-floating-info-pill {
  position: absolute;
  left: 20px;
  bottom: 24px;
  z-index: 40;
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 640px) {
  .lightbox-floating-info-pill {
    display: none;
  }
}

.lightbox-quick-exif-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(10, 10, 12, 0.82);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 9999px;
  padding: 6px 14px;
  color: #e2e8f0;
  font-size: 12px;
  cursor: pointer;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
  transition: all 0.18s ease;
}

.lightbox-quick-exif-btn:hover,
.lightbox-quick-exif-btn.is-active {
  background: rgba(24, 24, 27, 0.95);
  border-color: rgba(255, 255, 255, 0.35);
  color: #ffffff;
}

.quick-exif-main {
  font-weight: 500;
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.quick-exif-sep {
  color: #71717a;
  font-size: 10px;
}

.quick-exif-mp {
  font-family: monospace;
  font-weight: 600;
  color: #FFC629;
}

.quick-exif-action {
  font-size: 11px;
  color: #94a3b8;
  text-decoration: underline;
  text-underline-offset: 2px;
  margin-left: 2px;
}

/* Floating Bottom Quick Face Filter Popup Bar (AIFS Style) */
.lightbox-faces-pill-container {
  position: absolute;
  bottom: 24px;
  left: 0;
  right: 0;
  z-index: 45;
  display: flex;
  justify-content: center;
  pointer-events: none;
  padding: 0 16px;
}

@media (max-width: 640px) {
  .lightbox-faces-pill-container {
    bottom: 68px; /* Above mobile bottom bar */
  }
}

.lightbox-faces-pill {
  pointer-events: auto;
  display: flex;
  align-items: center;
  gap: 10px;
  background: rgba(10, 10, 12, 0.88);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 9999px;
  padding: 6px 14px 6px 12px;
  box-shadow: 0 10px 32px rgba(0, 0, 0, 0.65);
  max-width: min(92vw, 480px);
  overflow-x: auto;
  scrollbar-width: none;
}

.lightbox-faces-pill::-webkit-scrollbar {
  display: none;
}

.lightbox-faces-pill-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  font-weight: 600;
  color: #cbd5e1;
  white-space: nowrap;
  flex-shrink: 0;
}

.lightbox-faces-avatars {
  display: flex;
  align-items: center;
  gap: 6px;
}

.lightbox-face-avatar-btn {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  overflow: hidden;
  border: 1.5px solid rgba(255, 255, 255, 0.35);
  padding: 0;
  background: #18181b;
  cursor: pointer;
  flex-shrink: 0;
  transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}

.lightbox-face-avatar-btn:hover {
  transform: scale(1.15);
  border-color: #FFC629;
  box-shadow: 0 0 12px rgba(56, 189, 248, 0.5);
}

.lightbox-face-avatar-btn img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.lightbox-faces-toggle-all-btn {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: #cbd5e1;
  font-size: 11px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 9999px;
  cursor: pointer;
  white-space: nowrap;
  flex-shrink: 0;
  transition: all 0.15s ease;
}

.lightbox-faces-toggle-all-btn:hover {
  background: rgba(255, 255, 255, 0.2);
  color: #ffffff;
}

/* =========================================================================
   COMPREHENSIVE MOBILE FRIENDLY & RESPONSIVE TWEAKS (max-width: 768px)
   ========================================================================= */
@media (max-width: 768px) {
  /* Sticky Navbar on Mobile */
  .gallery-nav-inner {
    width: 94%;
    padding: 8px 0;
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }
  .gallery-nav-title-group {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .gallery-nav-title {
    font-size: 1.15rem;
  }
  .gallery-nav-actions {
    overflow-x: auto;
    flex-wrap: nowrap;
    justify-content: flex-start;
    padding-bottom: 4px;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: none;
    gap: 8px;
  }
  .gallery-nav-actions::-webkit-scrollbar {
    display: none;
  }
  .nav-action-btn {
    padding: 6px 12px;
    font-size: 12px;
    min-height: 36px;
  }
  .filter-btn {
    padding: 5px 10px;
    font-size: 11.5px;
  }

  /* Gallery Main & People container */
  .gallery-main {
    width: 94%;
    padding: 16px 0 60px;
  }
  .gallery-people-container,
  .gallery-active-filter-banner {
    width: 94%;
  }

  /* Photo Cards */
  .photo-card {
    margin-bottom: 10px;
    border-radius: 10px;
  }
  .photo-card-frame {
    border-radius: 10px;
  }
  .photo-card-info-badge {
    bottom: 4px;
    left: 4px;
    right: 4px;
    padding: 2px 6px;
    gap: 4px;
  }
  .photo-card-filename {
    font-size: 10px;
    max-width: 100px;
  }
  .photo-card-mp {
    font-size: 9.5px;
  }
  .photo-select-checkbox {
    width: 24px;
    height: 24px;
    top: 6px;
    left: 6px;
  }
  .photo-favorite-btn {
    top: 6px;
    right: 6px;
    width: 32px;
    height: 32px;
  }

  /* Floating Selection Bar */
  .gallery-selection-bar {
    bottom: calc(16px + env(safe-area-inset-bottom));
    width: calc(100% - 24px);
    max-width: 440px;
    padding: 8px 12px;
    gap: 8px;
    justify-content: space-between;
  }
  .selection-bar-left {
    padding-right: 4px;
  }
  .selection-count {
    font-size: 11px;
    white-space: nowrap;
  }
  .selection-actions {
    gap: 6px;
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: none;
  }
  .selection-actions::-webkit-scrollbar {
    display: none;
  }
  .btn-selection-action {
    padding: 5px 10px;
    font-size: 11px;
    min-height: 32px;
  }

  /* Lightbox Controls & Stage */
  .lightbox-topbar {
    padding: 10px 12px;
    gap: 8px;
  }
  .lightbox-counter {
    font-size: 12px;
  }
  .lightbox-actions {
    gap: 6px;
  }
  .lightbox-btn {
    padding: 6px 8px;
    font-size: 12px;
    min-height: 38px;
  }
  .lightbox-btn span {
    display: none; /* Hide button text labels on mobile lightbox to avoid overflow */
  }
  .lightbox-center {
    padding: 6px;
  }
  .lightbox-main-img {
    max-width: 100vw;
    max-height: calc(100vh - 120px);
    border-radius: 0;
  }
  .lightbox-nav-btn {
    width: 40px;
    height: 40px;
  }
  .lightbox-nav-btn.left {
    left: 8px;
  }
  .lightbox-nav-btn.right {
    right: 8px;
  }

  /* EXIF Bottom Sheet on Mobile */
  .lightbox-exif-drawer {
    right: 0;
    left: 0;
    top: auto;
    bottom: 0;
    width: 100%;
    max-width: 100vw;
    max-height: 75vh;
    border-radius: 20px 20px 0 0;
    padding: 16px 16px calc(24px + env(safe-area-inset-bottom));
    box-shadow: 0 -10px 40px rgba(0, 0, 0, 0.7);
    border-right: none;
    border-left: none;
    border-bottom: none;
  }
}

/* 6. FLOATING BATCH SELECTION BAR (AIFS Luxury Design) */
.gallery-selection-bar {
  position: fixed;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  background: rgba(24, 24, 27, 0.95);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 9999px;
  padding: 8px 14px 8px 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 198, 41, 0.25);
  max-width: 90vw;
  animation: fe-slide-up-dock 0.3s cubic-bezier(0.16, 1, 0.3, 1) forwards;
}

@keyframes fe-slide-up-dock {
  from {
    opacity: 0;
    transform: translate(-50%, 20px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translate(-50%, 0) scale(1);
  }
}

.selection-bar-left {
  display: flex;
  align-items: center;
}

.selection-count {
  font-size: 13px;
  font-weight: 700;
  color: #FFC629;
  white-space: nowrap;
}

.selection-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
}

.btn-selection-action {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border-radius: 9999px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: rgba(255, 255, 255, 0.08);
  color: #ffffff;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.btn-selection-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.16);
  border-color: rgba(255, 255, 255, 0.3);
}

.btn-selection-action:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.btn-selection-action--fav {
  border-color: rgba(255, 255, 255, 0.3);
  background: rgba(255, 198, 41, 0.2);
  color: #ffffff;
}

.btn-selection-action--fav:hover:not(:disabled) {
  background: rgba(255, 198, 41, 0.25);
  border-color: #FFC629;
}

.btn-selection-action--csv {
  border-color: rgba(255, 198, 41, 0.4);
  background: rgba(255, 198, 41, 0.15);
  color: #FFC629;
}

.btn-selection-action--csv:hover:not(:disabled) {
  background: rgba(255, 198, 41, 0.25);
  border-color: rgba(255, 198, 41, 0.6);
}

.btn-selection-action--primary {
  background: linear-gradient(135deg, #FFC629 0%, #F2B91B 100%);
  color: #171717;
  font-weight: 700;
  border: none;
  box-shadow: 0 2px 10px rgba(255, 198, 41, 0.4);
}

.btn-selection-action--primary:hover:not(:disabled) {
  filter: brightness(1.1);
  transform: translateY(-1px);
}

.btn-selection-action--close {
  padding: 6px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
  border: none;
  color: #a1a1aa;
}

.btn-selection-action--close:hover {
  background: rgba(255, 255, 255, 0.2);
  color: #ffffff;
}

/* Checkbox indicator in Select Mode */
.photo-select-checkbox {
  position: absolute;
  top: 10px;
  left: 10px;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.5);
  border: 2px solid rgba(255, 255, 255, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 15;
  transition: all 0.2s ease;
  backdrop-filter: blur(4px);
}

.photo-select-checkbox.is-checked {
  background: #FFC629;
  border-color: #FFC629;
  box-shadow: 0 0 10px rgba(255, 198, 41, 0.6);
}

.photo-select-checkbox-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: transparent;
}

/* Density classes for grid */
.masonry-columns.view-standard {
  column-count: 4;
}

.masonry-columns.view-large {
  column-count: 3;
}

.masonry-columns.view-compact {
  column-count: 6;
}

@media (max-width: 1280px) {
  .masonry-columns.view-standard {
    column-count: 3;
  }
  .masonry-columns.view-compact {
    column-count: 4;
  }
}

@media (max-width: 768px) {
  .masonry-columns,
  .masonry-columns.view-standard,
  .masonry-columns.view-large,
  .masonry-columns.view-compact {
    column-count: 2 !important;
    column-gap: 8px !important;
  }
  .photo-card {
    margin-bottom: 8px !important;
    border-radius: 10px !important;
  }
  .photo-card-frame {
    border-radius: 10px !important;
  }
}

/* Aspect ratio: square */
.photo-card.card-square .photo-card-frame {
  aspect-ratio: 1 / 1;
}

.photo-card.card-square .photo-card-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>


/* =========================================================================
   AIFS LUXURY GOLD SELFIE MODAL STYLES
   ========================================================================= */
.aifs-selfie-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 99999;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  animation: aifsFadeIn 0.2s ease-out;
}

@keyframes aifsFadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.aifs-selfie-modal-card {
  position: relative;
  background: #FFFFFF;
  border-radius: 24px;
  padding: 28px;
  max-width: 480px;
  width: 100%;
  border: 1px solid #E8E5DE;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.25);
  max-height: 90vh;
  overflow-y: auto;
  animation: aifsSlideUp 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}

.dark .aifs-selfie-modal-card,
.fe-album-root.dark .aifs-selfie-modal-card,
html.dark .aifs-selfie-modal-card,
body.dark .aifs-selfie-modal-card {
  background: #181817;
  border-color: #292927;
  color: #F5F5F4;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.6);
}

@keyframes aifsSlideUp {
  from { opacity: 0; transform: translateY(16px) scale(0.98); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

.aifs-selfie-modal-close {
  position: absolute;
  top: 18px;
  right: 18px;
  padding: 8px;
  border-radius: 50%;
  background: transparent;
  border: none;
  color: #686868;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
}

.aifs-selfie-modal-close:hover {
  background: #F8F6F0;
  color: #171717;
}

.dark .aifs-selfie-modal-close:hover,
html.dark .aifs-selfie-modal-close:hover {
  background: #222220;
  color: #FFFFFF;
}

.aifs-selfie-modal-header {
  display: flex;
  align-items: center;
  gap: 14px;
  padding-right: 32px;
  margin-bottom: 22px;
}

.aifs-selfie-modal-icon-ring {
  width: 44px;
  height: 44px;
  border-radius: 14px;
  background: rgba(255, 198, 41, 0.18);
  border: 1px solid rgba(255, 198, 41, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: #d97706;
}

.dark .aifs-selfie-modal-icon-ring,
html.dark .aifs-selfie-modal-icon-ring {
  color: #FFC629;
}

.aifs-selfie-modal-title {
  font-family: 'Montserrat', 'Inter', sans-serif;
  font-size: 17px;
  font-weight: 700;
  color: #171717;
  margin: 0;
  line-height: 1.25;
}

.dark .aifs-selfie-modal-title,
html.dark .aifs-selfie-modal-title {
  color: #F5F5F4;
}

.aifs-selfie-modal-subtitle {
  font-size: 12px;
  color: #686868;
  margin: 4px 0 0;
}

.dark .aifs-selfie-modal-subtitle,
html.dark .aifs-selfie-modal-subtitle {
  color: #A3A3A0;
}

/* Modal Options */
.aifs-selfie-options {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.btn-selfie-camera {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 14px 18px;
  min-height: 48px;
  border-radius: 16px;
  background: linear-gradient(135deg, #FFC629 0%, #F2B91B 100%);
  color: #171717;
  font-weight: 700;
  font-size: 13.5px;
  border: none;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(255, 198, 41, 0.35);
  transition: all 0.2s ease;
}

.btn-selfie-camera:hover {
  background: linear-gradient(135deg, #FFD04D 0%, #FFC629 100%);
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(255, 198, 41, 0.45);
}

.btn-selfie-camera:active {
  transform: scale(0.98);
}

.btn-selfie-gallery {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 13px 18px;
  min-height: 48px;
  border-radius: 16px;
  background: #F8F6F0;
  color: #171717;
  font-weight: 600;
  font-size: 13.5px;
  border: 1px solid #E8E5DE;
  cursor: pointer;
  transition: all 0.2s ease;
}

.dark .btn-selfie-gallery,
html.dark .btn-selfie-gallery {
  background: #222220;
  color: #F5F5F4;
  border-color: #292927;
}

.btn-selfie-gallery:hover {
  background: #E8E5DE;
}

.dark .btn-selfie-gallery:hover,
html.dark .btn-selfie-gallery:hover {
  background: #2B2B28;
}

.btn-selfie-gallery:active {
  transform: scale(0.98);
}

.aifs-selfie-privacy-note {
  margin-top: 8px;
  padding: 12px 14px;
  border-radius: 14px;
  background: #F8F6F0;
  border: 1px solid #E8E5DE;
  font-size: 11.5px;
  color: #686868;
  line-height: 1.5;
}

.dark .aifs-selfie-privacy-note,
html.dark .aifs-selfie-privacy-note {
  background: rgba(34, 34, 32, 0.6);
  border-color: #292927;
  color: #A3A3A0;
}

/* Preview Section */
.aifs-selfie-preview-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.aifs-selfie-preview-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px;
  border-radius: 18px;
  background: #F8F6F0;
  border: 1px solid #E8E5DE;
}

.dark .aifs-selfie-preview-card,
html.dark .aifs-selfie-preview-card {
  background: #222220;
  border-color: #292927;
}

.aifs-selfie-preview-img {
  width: 76px;
  height: 76px;
  border-radius: 14px;
  object-fit: cover;
  border: 2px solid #FFC629;
  box-shadow: 0 4px 12px rgba(255, 198, 41, 0.25);
  flex-shrink: 0;
}

.aifs-selfie-preview-info {
  flex: 1;
  min-width: 0;
}

.aifs-selfie-preview-heading {
  font-family: 'Montserrat', 'Inter', sans-serif;
  font-size: 13px;
  font-weight: 700;
  color: #171717;
  margin: 0 0 4px;
}

.dark .aifs-selfie-preview-heading,
html.dark .aifs-selfie-preview-heading {
  color: #F5F5F4;
}

.aifs-selfie-preview-desc {
  font-size: 11px;
  color: #686868;
  margin: 0 0 8px;
  line-height: 1.4;
}

.dark .aifs-selfie-preview-desc,
html.dark .aifs-selfie-preview-desc {
  color: #A3A3A0;
}

.btn-selfie-change {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: none;
  border: none;
  padding: 0;
  font-size: 12px;
  font-weight: 600;
  color: #d97706;
  cursor: pointer;
  text-decoration: underline;
}

.dark .btn-selfie-change,
html.dark .btn-selfie-change {
  color: #FFC629;
}

.aifs-selfie-searching-box {
  padding: 24px 16px;
  border-radius: 18px;
  background: rgba(255, 198, 41, 0.1);
  border: 1px solid rgba(255, 198, 41, 0.35);
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.aifs-selfie-spinner {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: 3px solid rgba(255, 198, 41, 0.2);
  border-top-color: #FFC629;
  animation: aifsSpin 0.75s linear infinite;
}

@keyframes aifsSpin {
  to { transform: rotate(360deg); }
}

.aifs-selfie-searching-step {
  font-family: 'Montserrat', 'Inter', sans-serif;
  font-size: 13px;
  font-weight: 700;
  color: #171717;
  margin: 4px 0 0;
}

.dark .aifs-selfie-searching-step,
html.dark .aifs-selfie-searching-step {
  color: #F5F5F4;
}

.aifs-selfie-searching-hint {
  font-size: 11px;
  color: #686868;
  margin: 0;
}

.btn-selfie-submit {
  width: 100%;
  padding: 14px 18px;
  min-height: 48px;
  border-radius: 16px;
  background: linear-gradient(135deg, #FFC629 0%, #F2B91B 100%);
  color: #171717;
  font-weight: 700;
  font-size: 14px;
  border: none;
  cursor: pointer;
  box-shadow: 0 4px 14px rgba(255, 198, 41, 0.35);
  transition: all 0.2s ease;
}

.btn-selfie-submit:hover {
  background: linear-gradient(135deg, #FFD04D 0%, #FFC629 100%);
  transform: translateY(-1px);
}

.btn-selfie-submit:active {
  transform: scale(0.98);
}

.aifs-selfie-error-banner {
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
}


/* =========================================================================
   LUXURY PRELOAD SPLASH SCREEN STYLES
   ========================================================================= */
.fe-gallery-splash {
  position: fixed;
  inset: 0;
  z-index: 999999;
  background-color: #FFFDF8;
  color: #171717;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: 'Inter', -apple-system, sans-serif;
}

.fe-gallery-splash.dark,
.fe-album-root.dark .fe-gallery-splash,
html.dark .fe-gallery-splash,
body.dark .fe-gallery-splash {
  background-color: #0E0E0D;
  color: #F5F5F4;
}

.splash-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 24px;
}

.splash-logo-text {
  font-family: 'Montserrat', 'Inter', sans-serif;
  font-weight: 900;
  font-size: 32px;
  letter-spacing: -0.05em;
  color: inherit;
  margin-bottom: 8px;
}

.splash-gold-dot {
  color: #FFC629;
}

.splash-logo-img {
  height: 48px;
  max-width: 220px;
  object-fit: contain;
  margin-bottom: 12px;
}

.splash-title {
  font-family: 'Montserrat', 'Inter', sans-serif;
  font-size: 16px;
  font-weight: 700;
  color: inherit;
  margin: 0 0 4px;
}

.splash-client {
  font-size: 12px;
  color: #686868;
  margin: 0 0 16px;
}

.dark .splash-client,
html.dark .splash-client {
  color: #A3A3A0;
}

.splash-gold-spinner {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  border: 3px solid rgba(255, 198, 41, 0.2);
  border-top-color: #FFC629;
  animation: aifsSpin 0.75s linear infinite;
  margin: 12px 0 14px;
}

.splash-subtext {
  font-size: 12px;
  font-weight: 500;
  letter-spacing: 0.02em;
  color: #686868;
}

.dark .splash-subtext,
html.dark .splash-subtext {
  color: #A3A3A0;
}

.fe-splash-fade-leave-active {
  transition: opacity 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}

.fe-splash-fade-leave-to {
  opacity: 0;
  pointer-events: none;
}

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
