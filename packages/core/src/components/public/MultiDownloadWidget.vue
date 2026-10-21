<script setup lang="ts">
import { ref, onMounted } from 'vue';
import type { AlbumItem } from '../../types/Public';

const props = defineProps<{
  photos: AlbumItem[];
  isIOS?: boolean;
}>();

const emit = defineEmits<{
  (e: 'complete'): void;
  (e: 'close'): void;
  (e: 'openIosModal'): void;
}>();

const currentIndex = ref(0);
const completedCount = ref(0);
const currentFilename = ref('');
const status = ref<'downloading' | 'completed' | 'cancelled'>('downloading');
const isCancelled = ref(false);

onMounted(() => {
  if (props.isIOS) {
    emit('openIosModal');
    emit('close');
    return;
  }
  void startBatchDownload();
});

async function startBatchDownload() {
  if (!props.photos || props.photos.length === 0) {
    emit('close');
    return;
  }

  status.value = 'downloading';
  completedCount.value = 0;

  for (let i = 0; i < props.photos.length; i++) {
    if (isCancelled.value) {
      status.value = 'cancelled';
      return;
    }

    const p = props.photos[i];
    currentIndex.value = i + 1;
    currentFilename.value = p.filename || `photo_${i + 1}.jpg`;

    const url = p.originalUrl || p.previewUrl || p.thumbnailUrl || '';
    if (url) {
      try {
        const res = await fetch(url);
        if (res.ok) {
          const blob = await res.blob();
          if (isCancelled.value) return;

          const blobUrl = window.URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.style.display = 'none';
          a.href = blobUrl;
          a.download = currentFilename.value;
          document.body.appendChild(a);
          a.click();

          setTimeout(() => {
            if (document.body.contains(a)) document.body.removeChild(a);
            window.URL.revokeObjectURL(blobUrl);
          }, 1500);

          completedCount.value++;
        }
      } catch {
        // Fallback direct link
        const a = document.createElement('a');
        a.href = url;
        a.download = currentFilename.value;
        document.body.appendChild(a);
        a.click();
        setTimeout(() => {
          if (document.body.contains(a)) document.body.removeChild(a);
        }, 1000);
        completedCount.value++;
      }
    }

    // Stagger downloads by 350ms to prevent browser popup blocking
    await new Promise((r) => setTimeout(r, 350));
  }

  status.value = 'completed';
  emit('complete');
  setTimeout(() => {
    emit('close');
  }, 2500);
}

function cancel() {
  isCancelled.value = true;
  status.value = 'cancelled';
  emit('close');
}
</script>

<template>
  <div class="fe-multi-dl">
    <div class="fe-multi-dl__head">
      <div class="fe-multi-dl__status">
        <span
          class="fe-multi-dl__dot"
          :class="{
            'fe-multi-dl__dot--active': status === 'downloading',
            'fe-multi-dl__dot--done': status === 'completed',
          }"
        ></span>
        <span class="fe-multi-dl__title">
          {{
            status === 'completed'
              ? 'Download Complete'
              : `Downloading ${currentIndex} of ${photos.length}`
          }}
        </span>
      </div>

      <button
        type="button"
        class="fe-multi-dl__close-btn"
        aria-label="Close"
        @click="cancel"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <line x1="18" y1="6" x2="6" y2="18"></line>
          <line x1="6" y1="6" x2="18" y2="18"></line>
        </svg>
      </button>
    </div>

    <div class="fe-multi-dl__file-row">
      <span class="fe-multi-dl__filename">{{ currentFilename }}</span>
      <span class="fe-multi-dl__pct">
        {{ Math.round((completedCount / (photos.length || 1)) * 100) }}%
      </span>
    </div>

    <div class="fe-multi-dl__track">
      <div
        class="fe-multi-dl__bar"
        :style="{ width: `${Math.round((completedCount / (photos.length || 1)) * 100)}%` }"
      ></div>
    </div>
  </div>
</template>

<style scoped>
.fe-multi-dl {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 9999;
  width: 320px;
  background: #ffffff;
  border-radius: 16px;
  padding: 16px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  border: 1px solid #e4e4e7;
  animation: fe-slide-up 0.25s ease;
}

:global(.dark) .fe-multi-dl,
:global([data-theme='dark']) .fe-multi-dl {
  background: #18181b;
  border-color: rgba(255, 255, 255, 0.1);
  color: #ffffff;
}

.fe-multi-dl__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.fe-multi-dl__status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 700;
}

.fe-multi-dl__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #a1a1aa;
}

.fe-multi-dl__dot--active {
  background: #3b82f6;
  animation: fe-pulse 1.5s infinite;
}

.fe-multi-dl__dot--done {
  background: #10b981;
}

.fe-multi-dl__close-btn {
  background: transparent;
  border: none;
  color: #71717a;
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.fe-multi-dl__close-btn svg {
  width: 14px;
  height: 14px;
}

.fe-multi-dl__file-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.fe-multi-dl__filename {
  font-size: 11px;
  color: #71717a;
  max-width: 220px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.fe-multi-dl__pct {
  font-size: 11px;
  font-weight: 700;
  font-family: monospace;
  color: #3b82f6;
}

.fe-multi-dl__track {
  width: 100%;
  height: 6px;
  background: #f4f4f5;
  border-radius: 9999px;
  overflow: hidden;
}

:global(.dark) .fe-multi-dl__track,
:global([data-theme='dark']) .fe-multi-dl__track {
  background: #27272a;
}

.fe-multi-dl__bar {
  height: 100%;
  background: #3b82f6;
  border-radius: 9999px;
  transition: width 0.2s ease;
}

@keyframes fe-slide-up {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes fe-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}
</style>
