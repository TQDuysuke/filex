<script setup lang="ts">
import { ref } from 'vue';

export interface ClusteredFace {
  face_id: number | string;
  photo_id?: number | string;
  photo_count: number;
  crop_url?: string;
  confidence?: number;
  filename?: string;
  photo_filenames?: string[];
}

const props = withDefaults(
  defineProps<{
    faces: ClusteredFace[];
    selectedFaceId: number | string | null;
    loading?: boolean;
  }>(),
  {
    loading: false,
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
    const amount = direction === 'left' ? -220 : 220;
    scrollRef.value.scrollBy({ left: amount, behavior: 'smooth' });
  }
}
</script>

<template>
  <div v-if="loading" class="fe-people__loading">
    <div v-for="i in 6" :key="i" class="fe-people__skeleton">
      <div class="fe-people__skeleton-avatar"></div>
      <div class="fe-people__skeleton-text"></div>
    </div>
  </div>

  <div v-else-if="faces.length > 0" class="fe-people">
    <div class="fe-people__head">
      <div class="fe-people__title">
        <svg class="fe-people__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
          <circle cx="9" cy="7" r="4"></circle>
          <path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
          <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
        </svg>
        <span>People</span>
      </div>

      <button
        v-if="selectedFaceId !== null"
        type="button"
        class="fe-people__clear-btn"
        @click="chooseFace(null)"
      >
        View All Photos
      </button>
    </div>

    <div class="fe-people__scroll-wrapper">
      <button
        type="button"
        class="fe-people__nav-btn fe-people__nav-btn--left"
        aria-label="Scroll left"
        @click="scroll('left')"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <polyline points="15 18 9 12 15 6"></polyline>
        </svg>
      </button>

      <button
        type="button"
        class="fe-people__nav-btn fe-people__nav-btn--right"
        aria-label="Scroll right"
        @click="scroll('right')"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <polyline points="9 18 15 12 9 6"></polyline>
        </svg>
      </button>

      <div ref="scrollRef" class="fe-people__list">
        <!-- All Photos item -->
        <button
          type="button"
          class="fe-people__item"
          :class="{ 'fe-people__item--active': selectedFaceId === null }"
          @click="chooseFace(null)"
        >
          <div class="fe-people__avatar fe-people__avatar--all">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
              <circle cx="9" cy="7" r="4"></circle>
              <path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
              <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
            </svg>
          </div>
          <span class="fe-people__name">All</span>
        </button>

        <!-- Person Face Items -->
        <button
          v-for="person in faces"
          :key="person.face_id"
          type="button"
          class="fe-people__item"
          :class="{ 'fe-people__item--active': selectedFaceId === person.face_id }"
          @click="chooseFace(selectedFaceId === person.face_id ? null : person.face_id)"
        >
          <div class="fe-people__avatar">
            <img
              v-if="person.crop_url"
              :src="person.crop_url"
              alt="Person"
              loading="lazy"
              class="fe-people__img"
            />
            <div v-else class="fe-people__placeholder">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="7" r="4"></circle>
                <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
              </svg>
            </div>
            <span class="fe-people__count-badge">{{ person.photo_count }}</span>
          </div>
          <span class="fe-people__name">{{ person.photo_count }} photos</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fe-people {
  position: relative;
  width: 100%;
  margin-bottom: 20px;
  background: rgba(255, 255, 255, 0.6);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 20px;
  padding: 14px 18px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.03);
}

:global(.dark) .fe-people,
:global([data-theme='dark']) .fe-people {
  background: rgba(20, 20, 22, 0.6);
  border-color: rgba(255, 255, 255, 0.08);
}

.fe-people__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.fe-people__title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 700;
  color: var(--fe-text, #18181b);
}

:global(.dark) .fe-people__title,
:global([data-theme='dark']) .fe-people__title {
  color: #f4f4f5;
}

.fe-people__icon {
  width: 16px;
  height: 16px;
  color: #3b82f6;
}

.fe-people__clear-btn {
  font-size: 12px;
  font-weight: 600;
  color: #3b82f6;
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 2px 6px;
}

.fe-people__clear-btn:hover {
  text-decoration: underline;
}

.fe-people__scroll-wrapper {
  position: relative;
}

.fe-people__nav-btn {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  z-index: 5;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: #ffffff;
  border: 1px solid #e4e4e7;
  color: #52525b;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.1);
  opacity: 0;
  transition: opacity 0.2s, background 0.2s;
}

.fe-people:hover .fe-people__nav-btn {
  opacity: 1;
}

.fe-people__nav-btn svg {
  width: 14px;
  height: 14px;
}

.fe-people__nav-btn--left {
  left: -8px;
}

.fe-people__nav-btn--right {
  right: -8px;
}

.fe-people__list {
  display: flex;
  align-items: center;
  gap: 16px;
  overflow-x: auto;
  padding: 4px 2px;
  scrollbar-width: none;
}

.fe-people__list::-webkit-scrollbar {
  display: none;
}

.fe-people__item {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  background: transparent;
  border: none;
  cursor: pointer;
  outline: none;
  transition: transform 0.2s ease, opacity 0.2s ease;
  opacity: 0.8;
}

.fe-people__item:hover {
  opacity: 1;
  transform: translateY(-2px);
}

.fe-people__item--active {
  opacity: 1;
  transform: scale(1.05);
}

.fe-people__avatar {
  position: relative;
  width: 58px;
  height: 58px;
  border-radius: 50%;
  overflow: hidden;
  background: #e4e4e7;
  border: 2px solid transparent;
  transition: all 0.2s ease;
}

:global(.dark) .fe-people__avatar,
:global([data-theme='dark']) .fe-people__avatar {
  background: #27272a;
}

.fe-people__item--active .fe-people__avatar {
  border-color: #3b82f6;
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.35);
}

.fe-people__avatar--all {
  display: flex;
  align-items: center;
  justify-content: center;
  background: #18181b;
  color: #ffffff;
}

:global(.dark) .fe-people__avatar--all,
:global([data-theme='dark']) .fe-people__avatar--all {
  background: #ffffff;
  color: #18181b;
}

.fe-people__avatar--all svg {
  width: 22px;
  height: 22px;
}

.fe-people__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.fe-people__placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a1a1aa;
}

.fe-people__count-badge {
  position: absolute;
  bottom: 0;
  right: 0;
  background: #3b82f6;
  color: #ffffff;
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 9999px;
  border: 1.5px solid #ffffff;
  line-height: 1.2;
}

.fe-people__name {
  font-size: 11px;
  font-weight: 600;
  color: var(--fe-text-muted, #71717a);
  max-width: 64px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fe-people__item--active .fe-people__name {
  color: #3b82f6;
  font-weight: 700;
}

/* Skeleton loader */
.fe-people__loading {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 18px;
  margin-bottom: 20px;
}

.fe-people__skeleton {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.fe-people__skeleton-avatar {
  width: 58px;
  height: 58px;
  border-radius: 50%;
  background: #e4e4e7;
  animation: pulse 1.5s infinite;
}

.fe-people__skeleton-text {
  width: 40px;
  height: 10px;
  border-radius: 4px;
  background: #e4e4e7;
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
</style>
