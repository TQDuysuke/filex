<script setup lang="ts">
import { ref } from 'vue';

export interface ClusteredFace {
  face_id: number | string;
  photo_id?: number | string;
  photo_count: number;
  crop_url?: string;
  face_crop_url?: string;
  sample_photo_thumbnail?: string;
  confidence?: number;
  filename?: string;
  photo_filenames?: string[];
}

const props = withDefaults(
  defineProps<{
    faces: ClusteredFace[];
    selectedFaceId: number | string | null;
    loading?: boolean;
    hideHeader?: boolean;
  }>(),
  {
    loading: false,
    hideHeader: false,
  },
);

const emit = defineEmits<{
  (e: 'selectFace', faceId: number | string | null): void;
  (e: 'select', faceId: number | string | null): void;
}>();

const scrollRef = ref<HTMLDivElement | null>(null);

function chooseFace(faceId: number | string | null) {
  emit('selectFace', faceId);
  emit('select', faceId);
}

function scroll(direction: 'left' | 'right') {
  if (scrollRef.value) {
    const amount = direction === 'left' ? -250 : 250;
    scrollRef.value.scrollBy({ left: amount, behavior: 'smooth' });
  }
}
</script>

<template>
  <div v-if="loading" class="aifs-people-loading">
    <div v-for="i in 6" :key="i" class="aifs-people-skeleton">
      <div class="aifs-skeleton-avatar"></div>
      <div class="aifs-skeleton-text"></div>
    </div>
  </div>

  <div v-else-if="faces.length > 0" class="aifs-people-wrap" :class="{ 'no-frame': hideHeader }">
    <!-- Header (only shown if not embedded in Hero) -->
    <div v-if="!hideHeader" class="aifs-people-header">
      <div class="aifs-people-header-left">
        <svg class="aifs-people-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
          <circle cx="9" cy="7" r="4"></circle>
          <path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
          <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
        </svg>
        <span class="aifs-people-title">Recognized People</span>
      </div>

      <button
        v-if="selectedFaceId !== null"
        type="button"
        class="aifs-people-clear-btn"
        @click="chooseFace(null)"
      >
        View all photos
      </button>
    </div>

    <!-- Carousel with floating navigation buttons -->
    <div class="aifs-people-carousel-box group">
      <button
        type="button"
        class="aifs-people-nav-btn aifs-people-nav-btn--left"
        aria-label="Scroll left"
        @click="scroll('left')"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <polyline points="15 18 9 12 15 6"></polyline>
        </svg>
      </button>

      <button
        type="button"
        class="aifs-people-nav-btn aifs-people-nav-btn--right"
        aria-label="Scroll right"
        @click="scroll('right')"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <polyline points="9 18 15 12 9 6"></polyline>
        </svg>
      </button>

      <div ref="scrollRef" class="aifs-people-track">
        <!-- All Photos Pill -->
        <button
          type="button"
          class="aifs-person-item"
          :class="{ active: selectedFaceId === null }"
          @click="chooseFace(null)"
        >
          <div class="aifs-avatar-circle aifs-avatar-all">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
              <circle cx="9" cy="7" r="4"></circle>
              <path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
              <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
            </svg>
          </div>
          <span class="aifs-person-label">All</span>
        </button>

        <!-- Individual Clustered Faces -->
        <button
          v-for="person in faces"
          :key="person.face_id"
          type="button"
          class="aifs-person-item"
          :class="{ active: selectedFaceId === person.face_id }"
          @click="chooseFace(selectedFaceId === person.face_id ? null : person.face_id)"
        >
          <div class="aifs-avatar-circle aifs-avatar-face">
            <img
              v-if="person.crop_url || person.face_crop_url"
              :src="person.crop_url || person.face_crop_url"
              alt="Recognized People"
              loading="lazy"
              class="aifs-avatar-img"
            />
            <div v-else class="aifs-avatar-placeholder">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="7" r="4"></circle>
                <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
              </svg>
            </div>

            <!-- Active checkmark overlay -->
            <div v-if="selectedFaceId === person.face_id" class="aifs-avatar-check-badge">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3.5">
                <polyline points="20 6 9 17 4 12"></polyline>
              </svg>
            </div>
          </div>

          <span v-if="person.photo_count > 0" class="aifs-person-count">
            {{ person.photo_count }} {{ person.photo_count === 1 ? 'photo' : 'photos' }}
          </span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* AIFS Luxury People Selector Styles */
.aifs-people-wrap {
  position: relative;
  width: 100%;
}

.aifs-people-wrap:not(.no-frame) {
  border-radius: 20px;
  border: 1px solid rgba(255, 198, 41, 0.35);
  background: rgba(255, 255, 255, 0.75);
  padding: 16px 20px;
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}

:global(.dark) .aifs-people-wrap:not(.no-frame),
:global(.fe--theme-dark) .aifs-people-wrap:not(.no-frame) {
  background: rgba(18, 18, 17, 0.75);
  border-color: rgba(255, 198, 41, 0.25);
}

/* Header */
.aifs-people-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.aifs-people-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13.5px;
  font-weight: 700;
  color: #171717;
}

:global(.dark) .aifs-people-header-left,
:global(.fe--theme-dark) .aifs-people-header-left {
  color: #F5F5F4;
}

.aifs-people-icon {
  width: 16px;
  height: 16px;
  color: #FFC629;
}

.aifs-people-clear-btn {
  font-size: 11.5px;
  font-weight: 600;
  color: #b45309;
  background: none;
  border: none;
  cursor: pointer;
  text-decoration: underline;
  transition: opacity 0.15s ease;
}

:global(.dark) .aifs-people-clear-btn,
:global(.fe--theme-dark) .aifs-people-clear-btn {
  color: #FFC629;
}

.aifs-people-clear-btn:hover {
  opacity: 0.8;
}

/* Carousel Box */
.aifs-people-carousel-box {
  position: relative;
  width: 100%;
}

.aifs-people-track {
  display: flex;
  align-items: center;
  gap: 14px;
  overflow-x: auto;
  padding: 8px 4px;
  scrollbar-width: none;
  -ms-overflow-style: none;
  scroll-behavior: smooth;
}

.aifs-people-track::-webkit-scrollbar {
  display: none;
}

/* Scroll Nav Buttons */
.aifs-people-nav-btn {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  z-index: 10;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: #ffffff;
  border: 1px solid #e8e5de;
  color: #52525b;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.12);
  opacity: 0;
  transition: opacity 0.2s ease, transform 0.15s ease;
}

:global(.dark) .aifs-people-nav-btn,
:global(.fe--theme-dark) .aifs-people-nav-btn {
  background: #222220;
  border-color: #292927;
  color: #e4e4e7;
}

.aifs-people-carousel-box:hover .aifs-people-nav-btn {
  opacity: 1;
}

.aifs-people-nav-btn svg {
  width: 15px;
  height: 15px;
}

.aifs-people-nav-btn--left {
  left: -8px;
}

.aifs-people-nav-btn--right {
  right: -8px;
}

.aifs-people-nav-btn:hover {
  transform: translateY(-50%) scale(1.08);
  border-color: #FFC629;
}

/* Person Item */
.aifs-person-item {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  transition: transform 0.2s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.2s ease;
  outline: none;
}

.aifs-person-item:not(.active) {
  opacity: 0.85;
}

.aifs-person-item:hover {
  opacity: 1;
  transform: translateY(-2px);
}

.aifs-person-item.active {
  opacity: 1;
  transform: scale(1.06);
}

/* Avatar Circle */
.aifs-avatar-circle {
  position: relative;
  width: 58px;
  height: 58px;
  border-radius: 50%;
  overflow: hidden;
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}

@media (min-width: 640px) {
  .aifs-avatar-circle {
    width: 64px;
    height: 64px;
  }
}

/* All Avatar Circle */
.aifs-avatar-all {
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f4f1ea;
  color: #686868;
  border: 2px solid transparent;
}

:global(.dark) .aifs-avatar-all,
:global(.fe--theme-dark) .aifs-avatar-all {
  background: #222220;
  color: #a3a3a0;
}

.aifs-person-item.active .aifs-avatar-all {
  background: #171717;
  color: #ffffff;
  box-shadow: 0 0 0 2px #FFFDF8, 0 0 0 4.5px #FFC629;
}

:global(.dark) .aifs-person-item.active .aifs-avatar-all,
:global(.fe--theme-dark) .aifs-person-item.active .aifs-avatar-all {
  background: #ffffff;
  color: #171717;
  box-shadow: 0 0 0 2px #0E0E0D, 0 0 0 4.5px #FFC629;
}

.aifs-avatar-all svg {
  width: 24px;
  height: 24px;
}

/* Face Avatar Circle */
.aifs-avatar-face {
  background: #e4e4e7;
  border: 2px solid transparent;
}

:global(.dark) .aifs-avatar-face,
:global(.fe--theme-dark) .aifs-avatar-face {
  background: #27272a;
}

.aifs-avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.aifs-avatar-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a1a1aa;
}

.aifs-person-item.active .aifs-avatar-face {
  border-color: #FFC629;
  box-shadow: 0 0 0 2px #FFFDF8, 0 0 0 4.5px #FFC629, 0 6px 16px rgba(255, 198, 41, 0.4);
}

:global(.dark) .aifs-person-item.active .aifs-avatar-face,
:global(.fe--theme-dark) .aifs-person-item.active .aifs-avatar-face {
  box-shadow: 0 0 0 2px #0E0E0D, 0 0 0 4.5px #FFC629, 0 6px 16px rgba(255, 198, 41, 0.35);
}

/* Checkmark Badge on Selected Face */
.aifs-avatar-check-badge {
  position: absolute;
  inset: 0;
  background: rgba(255, 198, 41, 0.25);
  display: flex;
  align-items: center;
  justify-content: center;
}

.aifs-avatar-check-badge svg {
  width: 16px;
  height: 16px;
  padding: 2px;
  border-radius: 50%;
  background: #FFC629;
  color: #171717;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
}

/* Labels */
.aifs-person-label {
  font-size: 11.5px;
  font-weight: 600;
  color: #171717;
  white-space: nowrap;
}

:global(.dark) .aifs-person-label,
:global(.fe--theme-dark) .aifs-person-label {
  color: #F5F5F4;
}

.aifs-person-count {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 10.5px;
  font-weight: 500;
  color: #71717a;
  white-space: nowrap;
}

:global(.dark) .aifs-person-count,
:global(.fe--theme-dark) .aifs-person-count {
  color: #a1a1aa;
}

/* Skeletons */
.aifs-people-loading {
  display: flex;
  align-items: center;
  gap: 12px;
  overflow-x: auto;
  padding: 10px 0;
}

.aifs-people-skeleton {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.aifs-skeleton-avatar {
  width: 58px;
  height: 58px;
  border-radius: 50%;
  background: #e4e4e7;
  animation: aifs-pulse 1.5s infinite;
}

:global(.dark) .aifs-skeleton-avatar,
:global(.fe--theme-dark) .aifs-skeleton-avatar {
  background: #27272a;
}

.aifs-skeleton-text {
  width: 32px;
  height: 10px;
  border-radius: 4px;
  background: #e4e4e7;
  animation: aifs-pulse 1.5s infinite;
}

:global(.dark) .aifs-skeleton-text,
:global(.fe--theme-dark) .aifs-skeleton-text {
  background: #27272a;
}

@keyframes aifs-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.45; }
}
</style>
