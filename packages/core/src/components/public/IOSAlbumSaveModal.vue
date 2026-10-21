<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue';
import type { AlbumItem } from '../../types/Public';

const props = defineProps<{
  photos: AlbumItem[];
  token?: string;
}>();

const emit = defineEmits<{
  (e: 'close'): void;
}>();

const savedPhotoIds = ref<Set<string>>(new Set());
const sharingPhotoId = ref<string | null>(null);
const readyFiles = ref<Record<string, File>>({});
const isDownloadingAll = ref(false);

function formatFileSize(bytes?: number | string | null): string | null {
  if (!bytes) return null;
  const num = typeof bytes === 'string' ? parseInt(bytes, 10) : bytes;
  if (isNaN(num) || num <= 0) return null;
  const mb = num / (1024 * 1024);
  return mb >= 1 ? `${mb.toFixed(1)} MB` : `${Math.round(num / 1024)} KB`;
}

let isCancelled = false;

onMounted(() => {
  // Lock body scroll
  const scrollY = window.scrollY;
  const origOverflow = document.body.style.overflow;
  document.body.style.overflow = 'hidden';

  // Background preload photos into memory for zero-latency gesture
  async function preloadAll() {
    for (const p of props.photos) {
      if (isCancelled) break;
      if (readyFiles.value[p.id]) continue;
      const url = p.originalUrl || p.previewUrl || p.thumbnailUrl;
      if (!url) continue;
      try {
        const res = await fetch(url);
        if (!res.ok) continue;
        const blob = await res.blob();
        if (isCancelled) break;
        const ext = blob.type.includes('png') ? '.png' : '.jpg';
        const cleanFilename = p.filename && p.filename.includes('.')
          ? p.filename
          : `${p.filename || 'photo'}${ext}`;
        const mimeType = blob.type && blob.type.startsWith('image/')
          ? blob.type
          : ext === '.png'
          ? 'image/png'
          : 'image/jpeg';
        const file = new File([blob], cleanFilename, { type: mimeType });
        readyFiles.value[p.id] = file;
      } catch {
        // silent preload
      }
    }
  }

  void preloadAll();

  onBeforeUnmount(() => {
    isCancelled = true;
    document.body.style.overflow = origOverflow;
    window.scrollTo(0, scrollY);
  });
});

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

async function handleShareSinglePhoto(photo: AlbumItem) {
  if (sharingPhotoId.value || isDownloadingAll.value) return;

  const fileToShare = readyFiles.value[photo.id];

  if (fileToShare) {
    if (typeof navigator !== 'undefined' && navigator.share) {
      try {
        await navigator.share({
          files: [fileToShare],
        });
        savedPhotoIds.value = new Set(savedPhotoIds.value).add(photo.id);
        return;
      } catch (err: any) {
        if (err.name === 'AbortError') return;
      }
    }
    triggerDirectDownload(photo);
    return;
  }

  sharingPhotoId.value = photo.id;
  try {
    const url = photo.originalUrl || photo.previewUrl || photo.thumbnailUrl || '';
    const res = await fetch(url);
    if (!res.ok) throw new Error('Download failed');
    const blob = await res.blob();

    const ext = blob.type.includes('png') ? '.png' : '.jpg';
    const cleanFilename = photo.filename && photo.filename.includes('.')
      ? photo.filename
      : `${photo.filename || 'photo'}${ext}`;
    const mimeType = blob.type && blob.type.startsWith('image/')
      ? blob.type
      : ext === '.png'
      ? 'image/png'
      : 'image/jpeg';

    const file = new File([blob], cleanFilename, { type: mimeType });
    readyFiles.value[photo.id] = file;

    if (typeof navigator !== 'undefined' && navigator.share) {
      try {
        await navigator.share({
          files: [file],
        });
        savedPhotoIds.value = new Set(savedPhotoIds.value).add(photo.id);
        return;
      } catch (shareErr: any) {
        if (shareErr.name === 'AbortError') return;
      }
    }

    triggerDirectDownload(photo);
  } catch (err: any) {
    if (err.name === 'AbortError') return;
  } finally {
    sharingPhotoId.value = null;
  }
}
</script>

<template>
  <div class="ios-save-overlay" @click.self="emit('close')">
    <div class="ios-save-card" @click.stop>
      <!-- Header -->
      <div class="ios-save-header">
        <div>
          <h3 class="ios-save-title">Save to Photos (iOS)</h3>
          <p class="ios-save-subtitle">
            {{ savedPhotoIds.size }} / {{ photos.length }} saved
          </p>
        </div>
        <button
          type="button"
          class="ios-save-close"
          title="Close"
          @click="emit('close')"
        >
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      <!-- Photo List -->
      <div class="ios-save-list">
        <div
          v-for="photo in photos"
          :key="photo.id"
          class="ios-save-item"
        >
          <div class="ios-save-item-info">
            <div class="ios-save-thumb-wrap">
              <img
                :src="photo.thumbnailUrl || photo.previewUrl || photo.originalUrl"
                :alt="photo.filename"
                class="ios-save-thumb"
                loading="lazy"
              />
              <div v-if="savedPhotoIds.has(photo.id)" class="ios-save-badge-saved">
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="#10b981" stroke-width="3">
                  <polyline points="20 6 9 17 4 12" />
                </svg>
              </div>
            </div>
            <div class="ios-save-meta">
              <span class="ios-save-filename" :title="photo.filename">{{ photo.filename }}</span>
              <span v-if="formatFileSize(photo.size)" class="ios-save-filesize">
                {{ formatFileSize(photo.size) }}
              </span>
            </div>
          </div>

          <button
            type="button"
            class="ios-save-btn"
            :class="{ 'is-saved': savedPhotoIds.has(photo.id) }"
            :disabled="sharingPhotoId === photo.id"
            @click="handleShareSinglePhoto(photo)"
          >
            <span v-if="sharingPhotoId === photo.id">Loading...</span>
            <span v-else-if="savedPhotoIds.has(photo.id)">Saved</span>
            <span v-else>Save</span>
          </button>
        </div>
      </div>

      <!-- Footer -->
      <div class="ios-save-footer">
        <button
          type="button"
          class="ios-save-footer-btn"
          @click="emit('close')"
        >
          Done
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ios-save-overlay {
  position: fixed;
  inset: 0;
  z-index: 99999;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding: 0;
}

@media (min-width: 640px) {
  .ios-save-overlay {
    align-items: center;
    padding: 16px;
  }
}

.ios-save-card {
  width: 100%;
  max-width: 540px;
  max-height: 85vh;
  background: #18181b;
  border-top: 1px solid #27272a;
  border-radius: 20px 20px 0 0;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5);
  overflow: hidden;
}

@media (min-width: 640px) {
  .ios-save-card {
    border: 1px solid #27272a;
    border-radius: 20px;
  }
}

.ios-save-header {
  padding: 16px 20px;
  border-bottom: 1px solid #27272a;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
  background: #18181b;
}

.ios-save-title {
  font-size: 16px;
  font-weight: 700;
  color: #ffffff;
  margin: 0;
}

.ios-save-subtitle {
  font-size: 12px;
  color: #a1a1aa;
  margin: 2px 0 0;
  font-family: monospace;
}

.ios-save-close {
  background: transparent;
  border: none;
  color: #a1a1aa;
  padding: 8px;
  cursor: pointer;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s;
}

.ios-save-close:hover {
  color: #ffffff;
  background: #27272a;
}

.ios-save-list {
  flex: 1;
  overflow-y: auto;
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.ios-save-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 12px;
  background: #27272a;
}

.ios-save-item-info {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  flex: 1;
}

.ios-save-thumb-wrap {
  position: relative;
  width: 44px;
  height: 44px;
  border-radius: 8px;
  overflow: hidden;
  background: #3f3f46;
  flex-shrink: 0;
}

.ios-save-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.ios-save-badge-saved {
  position: absolute;
  inset: 0;
  background: rgba(6, 78, 59, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
}

.ios-save-meta {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.ios-save-filename {
  font-size: 13px;
  font-weight: 600;
  color: #ffffff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ios-save-filesize {
  font-size: 11px;
  color: #a1a1aa;
  font-family: monospace;
}

.ios-save-btn {
  padding: 6px 14px;
  font-size: 12px;
  font-weight: 700;
  border-radius: 8px;
  border: 1px solid #FFC629;
  background: #FFC629;
  color: #171717;
  cursor: pointer;
  flex-shrink: 0;
  transition: all 0.15s;
}

.ios-save-btn:hover {
  background: #F2B91B;
}

.ios-save-btn.is-saved {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.4);
  color: #10b981;
}

.ios-save-footer {
  padding: 12px 20px calc(12px + env(safe-area-inset-bottom, 0px));
  border-top: 1px solid #27272a;
  background: #18181b;
  flex-shrink: 0;
}

.ios-save-footer-btn {
  width: 100%;
  padding: 10px;
  border-radius: 10px;
  border: none;
  background: #27272a;
  color: #ffffff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s;
}

.ios-save-footer-btn:hover {
  background: #3f3f46;
}
</style>
