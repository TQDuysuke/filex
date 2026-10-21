<script setup lang="ts">
import { ref, computed } from 'vue';

const props = defineProps<{
  url: string;
  title: string;
}>();

const emit = defineEmits<{
  (e: 'close'): void;
}>();

const copied = ref(false);

const qrImageUrl = computed(() => {
  return `https://api.qrserver.com/v1/create-qr-code/?size=260x260&margin=10&data=${encodeURIComponent(props.url)}`;
});

function copyLink() {
  void navigator.clipboard.writeText(props.url);
  copied.value = true;
  setTimeout(() => {
    copied.value = false;
  }, 2000);
}

function downloadQR() {
  const a = document.createElement('a');
  a.href = qrImageUrl.value;
  a.download = `QR_${props.title.replace(/\s+/g, '_')}.png`;
  a.target = '_blank';
  document.body.appendChild(a);
  a.click();
  setTimeout(() => {
    if (document.body.contains(a)) document.body.removeChild(a);
  }, 1000);
}
</script>

<template>
  <div class="fe-qr-modal__backdrop" @click="emit('close')">
    <div class="fe-qr-modal__card" @click.stop>
      <button type="button" class="fe-qr-modal__close" aria-label="Close" @click="emit('close')">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <line x1="18" y1="6" x2="6" y2="18"></line>
          <line x1="6" y1="6" x2="18" y2="18"></line>
        </svg>
      </button>

      <div class="fe-qr-modal__header">
        <div class="fe-qr-modal__icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="3" width="7" height="7"></rect>
            <rect x="14" y="3" width="7" height="7"></rect>
            <rect x="14" y="14" width="7" height="7"></rect>
            <rect x="3" y="14" width="7" height="7"></rect>
          </svg>
        </div>
        <h3 class="fe-qr-modal__title">{{ title }}</h3>
        <p class="fe-qr-modal__sub">Scan QR Code</p>
      </div>

      <div class="fe-qr-modal__qr-frame">
        <img :src="qrImageUrl" alt="QR Code" class="fe-qr-modal__qr-img" />
      </div>

      <div class="fe-qr-modal__url-box">
        <span class="fe-qr-modal__url-text">{{ url }}</span>
      </div>

      <div class="fe-qr-modal__actions">
        <button type="button" class="fe-btn fe-btn--primary fe-qr-modal__btn" @click="copyLink">
          {{ copied ? 'Link Copied' : 'Copy Link' }}
        </button>
        <button type="button" class="fe-btn fe-qr-modal__btn" @click="downloadQR">
          Save QR Code
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fe-qr-modal__backdrop {
  position: fixed;
  inset: 0;
  z-index: 99999;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  animation: fe-fade 0.2s ease;
}

.fe-qr-modal__card {
  position: relative;
  width: 100%;
  max-width: 380px;
  background: #ffffff;
  border-radius: 24px;
  padding: 28px 24px;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.3);
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
}

:global(.dark) .fe-qr-modal__card,
:global([data-theme='dark']) .fe-qr-modal__card {
  background: #18181b;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.fe-qr-modal__close {
  position: absolute;
  top: 16px;
  right: 16px;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: none;
  background: transparent;
  color: #71717a;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}

.fe-qr-modal__close:hover {
  background: #f4f4f5;
  color: #18181b;
}

:global(.dark) .fe-qr-modal__close:hover,
:global([data-theme='dark']) .fe-qr-modal__close:hover {
  background: #27272a;
  color: #ffffff;
}

.fe-qr-modal__close svg {
  width: 18px;
  height: 18px;
}

.fe-qr-modal__icon {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: rgba(59, 130, 246, 0.12);
  color: #3b82f6;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 12px;
}

.fe-qr-modal__icon svg {
  width: 22px;
  height: 22px;
}

.fe-qr-modal__title {
  font-size: 18px;
  font-weight: 700;
  color: var(--fe-text, #18181b);
  margin: 0 0 4px;
}

:global(.dark) .fe-qr-modal__title,
:global([data-theme='dark']) .fe-qr-modal__title {
  color: #ffffff;
}

.fe-qr-modal__sub {
  font-size: 13px;
  color: var(--fe-text-muted, #71717a);
  margin: 0 0 16px;
}

.fe-qr-modal__qr-frame {
  background: #ffffff;
  padding: 12px;
  border-radius: 16px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  border: 1px solid #e4e4e7;
  margin-bottom: 16px;
}

.fe-qr-modal__qr-img {
  width: 200px;
  height: 200px;
  display: block;
  border-radius: 8px;
}

.fe-qr-modal__url-box {
  width: 100%;
  padding: 8px 12px;
  background: #f4f4f5;
  border-radius: 10px;
  margin-bottom: 16px;
  overflow: hidden;
}

:global(.dark) .fe-qr-modal__url-box,
:global([data-theme='dark']) .fe-qr-modal__url-box {
  background: #27272a;
}

.fe-qr-modal__url-text {
  font-size: 11px;
  font-family: monospace;
  color: #71717a;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: block;
}

.fe-qr-modal__actions {
  width: 100%;
  display: flex;
  gap: 10px;
}

.fe-qr-modal__btn {
  flex: 1;
  padding: 10px 14px;
  font-size: 13px;
  font-weight: 600;
  border-radius: 12px;
}

@keyframes fe-fade {
  from { opacity: 0; }
  to { opacity: 1; }
}
</style>
