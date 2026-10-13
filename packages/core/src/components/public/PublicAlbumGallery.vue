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

export interface AlbumItem {
  id: string;
  filename: string;
  size?: number;
  width?: number | null;
  height?: number | null;
  thumbnailUrl?: string;
  previewUrl?: string;
  originalUrl?: string;
  isFavorite?: boolean;
}

export interface AlbumData {
  id?: string;
  title: string;
  clientName?: string;
  description?: string;
  location?: string;
  eventDate?: string;
  coverStyle?: 'haduanh' | 'editorial' | 'minimal' | 'split' | string;
  coverUrl?: string;
  itemCount?: number;
  items: AlbumItem[];
}

const props = defineProps<{
  album: AlbumData;
  token?: string;
  zipUrl?: string;
  locale?: string;
}>();

const emit = defineEmits<{
  (e: 'switchToFileView'): void;
}>();

// State
const filterMode = ref<'ALL' | 'FAVORITES'>('ALL');
const activeLightboxIndex = ref<number | null>(null);
const favoritesMap = ref<Record<string, boolean>>({});
const toastMsg = ref('');
let toastTimer: ReturnType<typeof setTimeout> | undefined;

// Storage key for favorites
const storageKey = computed(() => `filex_album_fav_${props.token || props.album.id || 'default'}`);

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
  showToast(!current ? 'Đã thích ảnh' : 'Đã bỏ thích');
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

const currentLightboxItem = computed(() => {
  if (activeLightboxIndex.value === null) return null;
  return displayItems.value[activeLightboxIndex.value] || null;
});

function onKeydown(e: KeyboardEvent) {
  if (activeLightboxIndex.value === null) return;
  if (e.key === 'Escape') closeLightbox();
  else if (e.key === 'ArrowLeft') prevLightbox();
  else if (e.key === 'ArrowRight') nextLightbox();
}

onMounted(() => {
  loadFavorites();
  window.addEventListener('keydown', onKeydown);
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown);
  if (toastTimer) clearTimeout(toastTimer);
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
          <span v-if="album.eventDate">📅 {{ album.eventDate }}</span>
          <span v-if="album.location">📍 {{ album.location }}</span>
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
        <span class="hero-split-badge">✨ Client: {{ album.clientName || 'VIP Guest' }}</span>
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
          </div>

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

    <!-- ========================================================================= -->
    <!-- 3. MASONRY GALLERY GRID (Natural aspect ratio, uncropped)                  -->
    <!-- ========================================================================= -->
    <main class="gallery-main">
      <div v-if="displayItems.length === 0" class="gallery-empty-state">
        <svg viewBox="0 0 24 24" width="48" height="48" fill="none" stroke="#cbd5e1" stroke-width="1.5">
          <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
        </svg>
        <p>No favorite photos yet in this collection.</p>
        <button type="button" class="btn-reset-filter" @click="filterMode = 'ALL'">
          View all photos
        </button>
      </div>

      <div v-else class="masonry-columns">
        <div
          v-for="(item, index) in displayItems"
          :key="item.id || index"
          class="photo-card"
          @click="openLightbox(index)"
        >
          <!-- Natural aspect ratio image -->
          <img
            :src="item.thumbnailUrl || item.previewUrl || item.originalUrl"
            :alt="item.filename"
            loading="lazy"
          />

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
        <div v-if="activeLightboxIndex !== null && currentLightboxItem" class="lightbox-modal">
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

              <a
                :href="currentLightboxItem.originalUrl || currentLightboxItem.previewUrl"
                class="lightbox-btn"
                download
                title="Download original photo"
                target="_blank"
              >
                <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
                </svg>
                <span style="font-size: 13px;">Download</span>
              </a>

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
            />

            <button type="button" class="lightbox-nav-btn right" title="Next photo (Right arrow)" @click.stop="nextLightbox">
              <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="9 18 15 12 9 6"/>
              </svg>
            </button>
          </div>

          <!-- Bottom bar -->
          <div class="lightbox-bottombar">
            <span>{{ currentLightboxItem.filename }}</span>
          </div>
        </div>
      </transition>
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
</style>
