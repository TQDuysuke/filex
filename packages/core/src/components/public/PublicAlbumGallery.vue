<script setup lang="ts">
/**
 * PublicAlbumGallery — Luxury Photo Album Showcase for public share links.
 * Exact implementation matching `web/sharing_source` (Vanilla & React templates):
 * - Full viewport edge-to-edge luxury editorial layout
 * - 4 Cover templates: Haduanh Luxury Hero, Editorial Serif, Minimal Rounded, Split Magazine
 * - Sticky navigation bar with title, metadata, and filter pills (Tất cả / Đã thích)
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
const viewMode = ref<'standard' | 'large' | 'compact'>('standard');
const aspectMode = ref<'natural' | 'square'>('natural');

// Clustered Faces State
const clusteredFaces = ref<ClusteredFace[]>([]);
const selectedFaceId = ref<number | string | null>(null);
const loadingFaces = ref(false);
const token = computed<string>(() => {
  if (props.token) return props.token;
  if (typeof window !== 'undefined') {
    const match = window.location.pathname.match(/\/s\/([a-zA-Z0-9_\-]+)/);
    if (match) return match[1];
  }
  return '';
});
const currentUrl = computed(() => (typeof window !== 'undefined' ? window.location.href : ''));

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
  const url = item.originalUrl || item.previewUrl || item.thumbnailUrl;
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
  const url = photo.originalUrl || photo.previewUrl || photo.thumbnailUrl || '';
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
  const url = item.originalUrl || item.previewUrl || window.location.href;
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
  const url = item.originalUrl || item.previewUrl || item.thumbnailUrl;
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
  if (selfieInputRef.value) selfieInputRef.value.value = '';
}

async function runSelfieSearch() {
  if (!aiSelfieFile.value || !token.value) return;
  aiSearching.value = true;
  aiSearchError.value = '';
  try {
    const formData = new FormData();
    formData.append('file', aiSelfieFile.value);
    const res = await fetch(`/api/public/s/${token.value}/face-search`, {
      method: 'POST',
      body: formData,
    });
    if (!res.ok) {
      const errJson = await res.json().catch(() => null);
      throw new Error(errJson?.error || `Search failed: HTTP ${res.status}`);
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
      showToast(`Found ${count} matching photos`);
      scrollToGallery();
    } else {
      aiSearchError.value = 'No matching faces found in this album.';
    }
  } catch (err: any) {
    aiSearchError.value = err?.message || 'Error occurred while searching';
  } finally {
    aiSearching.value = false;
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
  if (props.album.coverUrl) return props.album.coverUrl;
  const first = props.album.items[0];
  return first ? (first.originalUrl || first.previewUrl || first.thumbnailUrl || '') : '';
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
  const url = item.originalUrl || item.previewUrl || item.thumbnailUrl;
  if (!url) return;
  loadingExif.value = true;
  try {
    const meta = await fetchAndParseExif(url);
    if (meta) {
      exifCache.value[id] = meta;
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
  if (e.key === 'Escape') closeLightbox();
  else if (e.key === 'ArrowLeft') prevLightbox();
  else if (e.key === 'ArrowRight') nextLightbox();
}

onMounted(() => {
  isIOS.value = detectIsIOS();
  loadFavorites();
  window.addEventListener('keydown', onKeydown);
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
  if (toastTimer) clearTimeout(toastTimer);
  if (statusPollTimer) clearInterval(statusPollTimer);
});

watch(() => props.album, () => {
  loadFavorites();
});
</script>

<template>
  <div class="fe-album-root">
    <!-- Toast Notification -->
    <transition name="fe-album-fade">
      <div v-if="toastMsg" class="fe-album-toast" role="status">
        {{ toastMsg }}
      </div>
    </transition>

    <!-- ========================================================================= -->
    <!-- 1. HERO COVER SECTION (Tuỳ theo coverStyle: haduanh, editorial, minimal, split) -->
    <!-- ========================================================================= -->

    <!-- TEMPLATE 1: HADUANH LUXURY HERO (Chuẩn phong cách tạp chí cao cấp) -->
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

    <!-- TEMPLATE 2: EDITORIAL SERIF (Căn giữa nghệ thuật) -->
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
    <!-- 2. STICKY NAVBAR (Album title, photo count, filter All / Favorites)        -->
    <!-- ========================================================================= -->
    <nav id="feAlbumNav" class="gallery-nav">
      <div class="gallery-nav-inner">
        <div>
          <h2 class="gallery-nav-title">{{ album.title }}</h2>
          <p class="gallery-nav-meta">
            {{ album.items.length }} photos · {{ totalFavorites }} favorited
          </p>
        </div>

        <!-- Filter Pills & Action Buttons -->
        <div class="gallery-nav-actions">
          <div class="filter-pills">
            <button
              type="button"
              class="filter-btn"
              :class="{ active: filterMode === 'ALL' }"
              @click="filterMode = 'ALL'"
            >
              All ({{ album.items.length }})
            </button>
            <button
              type="button"
              class="filter-btn"
              :class="{ active: filterMode === 'FAVORITES' }"
              @click="filterMode = 'FAVORITES'"
            >
              <svg viewBox="0 0 24 24" width="14" height="14" :fill="filterMode === 'FAVORITES' ? '#ef4444' : 'currentColor'">
                <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
              </svg>
              Favorites ({{ totalFavorites }})
            </button>
            <button
              v-if="aiMatchCount > 0"
              type="button"
              class="filter-btn filter-btn--ai"
              :class="{ active: filterMode === 'AI_MATCHES' }"
              @click="filterMode = 'AI_MATCHES'"
            >
              Matched Photos ({{ aiMatchCount }})
            </button>
          </div>

          <!-- View Controls: Grid Density & Aspect Ratio -->
          <div class="view-density-group">
            <button
              type="button"
              class="density-btn"
              :class="{ active: viewMode === 'standard' }"
              title="Standard Grid"
              @click="viewMode = 'standard'"
            >
              Standard
            </button>
            <button
              type="button"
              class="density-btn"
              :class="{ active: viewMode === 'large' }"
              title="Large Grid"
              @click="viewMode = 'large'"
            >
              Large
            </button>
            <button
              type="button"
              class="density-btn"
              :class="{ active: viewMode === 'compact' }"
              title="Compact Grid"
              @click="viewMode = 'compact'"
            >
              Compact
            </button>
          </div>

          <div class="view-aspect-group">
            <button
              type="button"
              class="density-btn"
              :class="{ active: aspectMode === 'natural' }"
              title="Natural Aspect"
              @click="aspectMode = 'natural'"
            >
              Natural
            </button>
            <button
              type="button"
              class="density-btn"
              :class="{ active: aspectMode === 'square' }"
              title="Square Aspect"
              @click="aspectMode = 'square'"
            >
              Square
            </button>
          </div>

          <button
            type="button"
            class="nav-action-btn nav-action-btn--ai"
            title="Face Search"
            @click="showAiModal = true"
          >
            <span>Face Search</span>
          </button>

          <!-- QR Code Share -->
          <button
            type="button"
            class="nav-action-btn"
            title="QR Code"
            @click="showQrModal = true"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="3" width="7" height="7"></rect>
              <rect x="14" y="3" width="7" height="7"></rect>
              <rect x="14" y="14" width="7" height="7"></rect>
              <rect x="3" y="14" width="7" height="7"></rect>
            </svg>
            <span>QR Code</span>
          </button>

          <!-- iOS Save to Photos Button -->
          <button
            v-if="isIOS"
            type="button"
            class="nav-action-btn nav-action-btn--ios"
            title="Save to Photos (iOS)"
            @click="showIosSaveModal = true"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
            </svg>
            <span>Save to Photos</span>
          </button>

          <!-- Multi-Select Mode Toggle -->
          <button
            type="button"
            class="nav-action-btn"
            :class="{ 'nav-action-btn--active': isSelectMode }"
            title="Select Photos"
            @click="toggleSelectMode"
          >
            <span>{{ isSelectMode ? 'Cancel' : 'Select' }}</span>
          </button>

          <a
            v-if="zipUrl"
            :href="zipUrl"
            class="nav-action-btn"
            title="Download entire album as ZIP"
            download
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
            </svg>
            Download ZIP
          </a>

          <button
            type="button"
            class="nav-action-btn nav-action-btn--secondary"
            title="Switch to file list view"
            @click="emit('switchToFileView')"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="8" y1="6" x2="21" y2="6"/>
              <line x1="8" y1="12" x2="21" y2="12"/>
              <line x1="8" y1="18" x2="21" y2="18"/>
              <line x1="3" y1="6" x2="3.01" y2="6"/>
              <line x1="3" y1="12" x2="3.01" y2="12"/>
              <line x1="3" y1="18" x2="3.01" y2="18"/>
            </svg>
            Switch to files
          </button>
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
            {{ aifsStatus.indexed_photos ?? 0 }} / {{ aifsStatus.total_photos ?? 0 }} photos indexed
            <template v-if="aifsStatus.detected_faces"> · {{ aifsStatus.detected_faces }} faces detected</template>
            <template v-if="aifsStatus.clusters_count"> · {{ aifsStatus.clusters_count }} unique people</template>
          </span>
        </div>
        <span class="indexing-banner-percent">{{ Math.round(aifsStatus.percent_complete ?? 0) }}%</span>
      </div>
      <div class="indexing-banner-bar">
        <div class="indexing-banner-bar-fill" :style="{ width: `${Math.min(100, Math.round(aifsStatus.percent_complete ?? 0))}%` }"></div>
      </div>
    </div>

    <!-- Clustered Faces Carousel -->
    <section v-if="token && (clusteredFaces.length > 0 || loadingFaces)" class="gallery-people-container">
      <PeopleSelector
        :faces="clusteredFaces"
        :selected-face-id="selectedFaceId"
        :token="token"
        :loading="loadingFaces"
        @select="handleSelectFace"
      />
    </section>

    <!-- Active Face Filter Banner -->
    <div v-if="selectedFaceId !== null" class="gallery-active-filter-banner">
      <div class="active-filter-left">
        <span class="active-filter-indicator"></span>
        <span class="active-filter-text">Filtered by person · {{ displayItems.length }} photos</span>
      </div>
      <button type="button" class="btn-clear-face-filter" @click="clearFaceFilter">
        Clear filter
      </button>
    </div>

    <!-- ========================================================================= -->
    <!-- 3. MASONRY GALLERY GRID (Natural aspect ratio, uncropped)                  -->
    <!-- ========================================================================= -->
    <main class="gallery-main">
      <div v-if="displayItems.length === 0" class="gallery-empty-state">
        <svg viewBox="0 0 24 24" width="48" height="48" fill="none" stroke="#cbd5e1" stroke-width="1.5">
          <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
        </svg>
        <p v-if="filterMode === 'AI_MATCHES'">No matching photos found in this album.</p>
        <p v-else>No favorite photos yet in this collection.</p>
        <button type="button" class="btn-reset-filter" @click="filterMode = 'ALL'">
          View all photos
        </button>
      </div>

      <div v-else :class="['masonry-columns', `view-${viewMode}`, `aspect-${aspectMode}`]">
        <div
          v-for="(item, index) in displayItems"
          :key="item.id || index"
          class="photo-card"
          :class="{ 'is-selected': selectedItemIds.has(item.id), 'card-square': aspectMode === 'square' }"
          @click="isSelectMode ? toggleSelectItem(item.id, $event) : openLightbox(index)"
        >
          <!-- Selection indicator in select mode -->
          <div
            v-if="isSelectMode"
            class="photo-select-checkbox"
            :class="{ 'is-checked': selectedItemIds.has(item.id) }"
            @click.stop="toggleSelectItem(item.id, $event)"
          >
            <svg v-if="selectedItemIds.has(item.id)" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="#ffffff" stroke-width="3">
              <polyline points="20 6 9 17 4 12"/>
            </svg>
          </div>

          <!-- AI similarity match badge -->
          <div v-if="aiMatchesMap[item.filename]" class="ai-match-badge" title="Match confidence">
            <span>{{ Math.round((aiMatchesMap[item.filename].similarity || 0.85) * 100) }}% Match</span>
          </div>

          <!-- Natural aspect ratio image -->
          <img
            :src="item.thumbnailUrl || item.previewUrl || item.originalUrl"
            :alt="item.filename"
            loading="lazy"
          />

          <!-- Quick Download / Save to Photos button -->
          <button
            type="button"
            class="btn-download-quick photo-action-shadow"
            :title="isIOS ? 'Save to Photos' : 'Download'"
            @click.stop="downloadPhoto(item, $event)"
          >
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="#ffffff" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
            </svg>
          </button>

          <!-- Heart favorite button -->
          <button
            type="button"
            class="btn-heart photo-heart-shadow heart-interactive"
            :title="favoritesMap[item.id] ? 'Favorited' : 'Favorite'"
            @click.stop="toggleFavorite(item.id, $event)"
          >
            <svg
              viewBox="0 0 24 24"
              width="26"
              height="26"
              :fill="favoritesMap[item.id] ? '#ef4444' : 'rgba(0,0,0,0.3)'"
              :stroke="favoritesMap[item.id] ? '#ef4444' : '#ffffff'"
              stroke-width="2"
            >
              <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
            </svg>
          </button>
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
                  :fill="favoritesMap[currentLightboxItem.id] ? '#ef4444' : 'none'"
                  :stroke="favoritesMap[currentLightboxItem.id] ? '#ef4444' : 'currentColor'"
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

            <img
              :src="currentLightboxItem.originalUrl || currentLightboxItem.previewUrl"
              :alt="currentLightboxItem.filename"
              @load="onLightboxImageLoad($event, currentLightboxItem)"
            />

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
              <span class="quick-exif-main">{{ currentLightboxItem.camera || 'Photo details' }}</span>
              <template v-if="currentLightboxItem.taken_at">
                <span class="quick-exif-sep">•</span>
                <span>{{ new Date(currentLightboxItem.taken_at).toLocaleDateString() }}</span>
              </template>
              <template v-if="getMegapixels(currentLightboxItem.width, currentLightboxItem.height)">
                <span class="quick-exif-sep">•</span>
                <span class="quick-exif-mp">{{ getMegapixels(currentLightboxItem.width, currentLightboxItem.height) }}</span>
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
                <div v-if="currentLightboxItem.camera || currentLightboxItem.lens || currentLightboxItem.taken_at" class="exif-section">
                  <div class="exif-section-title">Camera & Gear</div>
                  <div v-if="currentLightboxItem.camera" class="exif-row">
                    <span class="exif-label">Camera</span>
                    <span class="exif-value">{{ currentLightboxItem.camera }}</span>
                  </div>
                  <div v-if="currentLightboxItem.lens" class="exif-row">
                    <span class="exif-label">Lens</span>
                    <span class="exif-value">{{ currentLightboxItem.lens }}</span>
                  </div>
                  <div v-if="currentLightboxItem.taken_at" class="exif-row">
                    <span class="exif-label">Date Taken</span>
                    <span class="exif-value">{{ new Date(currentLightboxItem.taken_at).toLocaleString('en-US', { dateStyle: 'medium', timeStyle: 'short' }) }}</span>
                  </div>
                </div>

                <!-- Exposure Parameters -->
                <div v-if="currentLightboxItem.aperture || currentLightboxItem.shutter_speed || currentLightboxItem.iso || currentLightboxItem.focal_length" class="exif-section">
                  <div class="exif-section-title">Exposure</div>
                  <div class="exif-params-grid">
                    <div v-if="currentLightboxItem.aperture" class="exif-param-box">
                      <span class="exif-param-name">Aperture</span>
                      <span class="exif-param-val">{{ currentLightboxItem.aperture }}</span>
                    </div>
                    <div v-if="currentLightboxItem.shutter_speed" class="exif-param-box">
                      <span class="exif-param-name">Shutter</span>
                      <span class="exif-param-val">{{ currentLightboxItem.shutter_speed }}</span>
                    </div>
                    <div v-if="currentLightboxItem.iso" class="exif-param-box">
                      <span class="exif-param-name">ISO</span>
                      <span class="exif-param-val">{{ currentLightboxItem.iso }}</span>
                    </div>
                    <div v-if="currentLightboxItem.focal_length" class="exif-param-box">
                      <span class="exif-param-name">Focal Length</span>
                      <span class="exif-param-val">{{ currentLightboxItem.focal_length }}</span>
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
                  <div v-if="currentLightboxItem.width && currentLightboxItem.height" class="exif-row">
                    <span class="exif-label">Dimensions</span>
                    <span class="exif-value">{{ currentLightboxItem.width }} × {{ currentLightboxItem.height }} px {{ getMegapixels(currentLightboxItem.width, currentLightboxItem.height) ? `(${getMegapixels(currentLightboxItem.width, currentLightboxItem.height)})` : '' }}</span>
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

    <!-- 5. AI FACE SEARCH MODAL -->
    <teleport to="body">
      <transition name="fe-album-fade">
        <div v-if="showAiModal" class="ai-modal-overlay" @click.self="showAiModal = false">
          <div class="ai-modal-card">
            <!-- Modal Header -->
            <div class="ai-modal-header">
              <div class="ai-modal-title">
                <span>Face Search</span>
              </div>
              <button type="button" class="ai-modal-close" @click="showAiModal = false" title="Close">
                <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="18" y1="6" x2="6" y2="18"/>
                  <line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
              </button>
            </div>

            <!-- Modal Body -->
            <div class="ai-modal-body">
              <!-- Indexing status banner -->
              <div v-if="aifsStatus" class="ai-status-banner" style="margin-bottom: 16px; padding: 10px 14px; border-radius: 8px; font-size: 12.5px; display: flex; align-items: center; justify-content: space-between; background: #f8fafc; border: 1px solid #e2e8f0;">
                <div style="display: flex; align-items: center; gap: 8px;">
                  <span style="display: inline-block; width: 8px; height: 8px; border-radius: 50%;" :style="{ background: aifsStatus.status === 'ready' ? '#10b981' : '#f59e0b' }"></span>
                  <span style="color: #475569; font-weight: 500;">
                    AI Status: {{ aifsStatus.status === 'ready' ? 'Ready' : 'Indexing' }}
                  </span>
                </div>
                <span style="color: #64748b; font-size: 12px;">
                  {{ aifsStatus.indexed_photos }} / {{ aifsStatus.total_photos }} photos ({{ aifsStatus.detected_faces ?? aifsStatus.total_faces ?? 0 }} faces)
                </span>
              </div>

              <!-- Upload Selfie Area -->
              <div
                v-if="!aiSelfiePreview"
                class="ai-dropzone"
                @click="triggerSelfieInput"
                @dragover.prevent
                @drop.prevent="onSelfieDrop"
              >
                <input
                  ref="selfieInputRef"
                  type="file"
                  accept="image/*"
                  style="display: none;"
                  @change="onSelfieChange"
                />
                <div style="font-size: 14.5px; font-weight: 600; color: #1e293b;">Upload Selfie</div>
              </div>

              <!-- Selfie Preview -->
              <div v-else style="text-align: center;">
                <div class="ai-preview-box">
                  <img :src="aiSelfiePreview" alt="Selfie" class="ai-preview-img" />
                </div>
                <button
                  type="button"
                  style="background: none; border: none; color: #64748b; font-size: 12.5px; cursor: pointer; text-decoration: underline; margin-bottom: 16px;"
                  @click="clearSelfie"
                >
                  Change photo
                </button>
              </div>

              <!-- Error display -->
              <div v-if="aiSearchError" style="margin-top: 14px; padding: 10px 14px; border-radius: 8px; background: #fef2f2; border: 1px solid #fecaca; color: #b91c1c; font-size: 13px;">
                {{ aiSearchError }}
              </div>

              <!-- Action button -->
              <div style="margin-top: 20px;">
                <button
                  type="button"
                  class="ai-btn-primary"
                  :disabled="!aiSelfieFile || aiSearching"
                  @click="runSelfieSearch"
                >
                  <span v-if="aiSearching" class="ai-spinner"></span>
                  <span>{{ aiSearching ? 'Searching...' : 'Search' }}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </transition>
    </teleport>

    <!-- 6. FLOATING BATCH SELECTION BAR -->
    <teleport to="body">
      <transition name="fe-album-fade">
        <div v-if="isSelectMode && displayItems.length > 0" class="gallery-selection-bar">
          <span class="selection-count">{{ selectedItemIds.size }} selected</span>
          <div class="selection-actions">
            <button type="button" class="btn-selection-action" @click="selectAllItems">
              {{ selectedItemIds.size === displayItems.length ? 'Deselect All' : 'Select All' }}
            </button>
            <button
              type="button"
              class="btn-selection-action btn-selection-action--primary"
              :disabled="selectedItemIds.size === 0"
              @click="handleBatchAction"
            >
              {{ isIOS ? 'Save to Photos' : 'Download Selected' }}
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
   Matching web/sharing_source: Phong cách Luxury Editorial Gallery
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
  background-color: #fafafa;
  color: #171717;
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  -webkit-font-smoothing: antialiased;
  scroll-behavior: smooth;
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
  background-color: rgba(250, 250, 250, 0.95);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid #e5e5e5;
  transition: all 0.2s ease;
}

.gallery-nav-inner {
  width: 85%;
  max-width: 1600px;
  margin: 0 auto;
  padding: 14px 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.gallery-nav-title {
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-size: 1.25rem;
  font-weight: 600;
  color: #171717;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin: 0;
}

.gallery-nav-meta {
  font-size: 12px;
  color: #737373;
  margin-top: 2px;
}

.gallery-nav-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* Filter Pill */
.filter-pills {
  display: flex;
  background-color: #f1f5f9;
  padding: 4px;
  border-radius: 9999px;
  border: 1px solid #e5e5e5;
  font-size: 12px;
}

.filter-btn {
  padding: 6px 14px;
  border-radius: 9999px;
  border: none;
  background: transparent;
  color: #737373;
  font-weight: 600;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: all 0.2s ease;
}

.filter-btn.active {
  background-color: #171717;
  color: #ffffff;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
}

.nav-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 9999px;
  background-color: #171717;
  color: #ffffff;
  font-size: 12px;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
  border: none;
  transition: opacity 0.2s ease;
}
.nav-action-btn:hover {
  opacity: 0.85;
}
.nav-action-btn--secondary {
  background-color: #f1f5f9;
  color: #475569;
  border: 1px solid #e2e8f0;
}
.nav-action-btn--secondary:hover {
  background-color: #e2e8f0;
}

/* 3. MASONRY GRID (CSS MULTI-COLUMN) */
.gallery-main {
  width: 85%;
  max-width: 1600px;
  margin: 0 auto;
  padding: 40px 0 80px;
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
@media (max-width: 768px) {
  .masonry-columns {
    column-count: 2;
  }
  .gallery-nav-inner {
    width: 92%;
    flex-direction: column;
    align-items: flex-start;
  }
  .gallery-main {
    width: 92%;
  }
}
@media (max-width: 480px) {
  .masonry-columns {
    column-count: 1;
  }
}

.photo-card {
  break-inside: avoid;
  position: relative;
  margin-bottom: 16px;
  background-color: #e2e8f0;
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
  box-shadow: 0 1px 3px rgba(0,0,0,0.05);
  transition: transform 0.3s ease, box-shadow 0.3s ease;
}

.photo-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(0,0,0,0.08);
}

.photo-card img {
  width: 100%;
  height: auto;
  display: block;
  object-fit: cover;
  transition: transform 0.5s ease;
}

.photo-card:hover img {
  transform: scale(1.02);
}

/* Heart Button */
.btn-heart {
  position: absolute;
  top: 12px;
  right: 12px;
  background: transparent;
  border: none;
  cursor: pointer;
  z-index: 10;
  color: #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
}

.photo-heart-shadow {
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.8)) drop-shadow(0 0 2px rgba(0, 0, 0, 0.6));
}

.heart-interactive {
  transition: transform 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}
.heart-interactive:hover {
  transform: scale(1.2);
}
.heart-interactive:active {
  transform: scale(0.9);
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

/* AI Button & Modal styles */
.nav-action-btn--ai {
  background: linear-gradient(135deg, #7c3aed 0%, #4f46e5 100%) !important;
  color: #ffffff !important;
  border: none !important;
  box-shadow: 0 2px 8px rgba(124, 58, 237, 0.35);
  font-weight: 500;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: all 0.2s ease;
}
.nav-action-btn--ai:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 14px rgba(124, 58, 237, 0.45);
}

.filter-btn--ai {
  border-color: #8b5cf6 !important;
  color: #7c3aed !important;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.filter-btn--ai.active {
  background: #7c3aed !important;
  color: #ffffff !important;
}

.ai-match-badge {
  position: absolute;
  top: 10px;
  left: 10px;
  background: rgba(124, 58, 237, 0.88);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  padding: 4px 8px;
  border-radius: 9999px;
  backdrop-filter: blur(4px);
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
  border-color: #8b5cf6;
  background: #f5f3ff;
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
  background: linear-gradient(135deg, #7c3aed 0%, #4f46e5 100%);
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
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.45);
  border: 2px solid #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 5;
  cursor: pointer;
  transition: all 0.15s ease;
}

.photo-select-checkbox.is-checked {
  background: #3b82f6;
  border-color: #3b82f6;
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
  background: rgba(24, 24, 27, 0.95);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 9999px;
  padding: 10px 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
  color: #ffffff;
}

.selection-count {
  font-size: 13px;
  font-weight: 600;
  font-family: monospace;
}

.selection-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-selection-action {
  padding: 6px 14px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 9999px;
  border: 1px solid #3f3f46;
  background: #27272a;
  color: #ffffff;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-selection-action:hover {
  background: #3f3f46;
}

.btn-selection-action--primary {
  background: #3b82f6;
  border-color: #3b82f6;
}

.btn-selection-action--primary:hover {
  background: #2563eb;
}

.btn-selection-action--primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
  .masonry-columns.view-large {
    column-count: 1;
  }
  .masonry-columns.view-compact {
    column-count: 3;
  }
}
@media (max-width: 480px) {
  .masonry-columns.view-compact {
    column-count: 2;
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
  border-top-color: #38bdf8;
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
  color: #38bdf8;
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
  color: #38bdf8;
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
  border-color: #38bdf8;
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
</style>
