<script setup lang="ts">
/**
 * PublicShareBody — what a share actually contains.
 *
 * Three bodies, one shell (`PublicShell`): a document to download, a folder,
 * or an app plugin's screen when the link carries one (v3 §1.1 — "an app's
 * public page IS a share"). The chrome around all three — branding, the PIN
 * gate, expiry, the language picker — is the shell's and is not repeated
 * here.
 *
 * NOTE: The surface is drawn by the SAME `SurfaceConversation` the explorer's
 * dialog and an app's page use. A visitor and a signed-in user must see one
 * surface rendered one way, or every plugin has to be tested twice.
 *
 * WARNING: The bytes are NOT fetched through `/api/`. `/s/<token>` has always been
 * the address that serves the file and still is: the SPA is what a
 * JavaScript browser gets when it NAVIGATES there, and `?download=1` /
 * `?zip=1` are the escapes that mean "the thing, not the page"
 * (`handlers/public_shell.go`). A second download route under `/api/` would
 * be a second thing to keep working.
 */
import { computed, ref, watch } from 'vue';
import type { LocaleCode, ThemeMode } from '../../types/ExplorerConfig';
import type { PublicEntry, PublicShareInfo, AlbumData } from '../../types/Public';
import type { PublicLinkStore } from '../../composables/usePublicLink';
import { useLocale } from '../../composables/useLocale';
import { usePublicText } from '../../composables/usePublicText';
import SurfaceConversation from '../plugin/SurfaceConversation.vue';
import SurfaceFooterButtons from '../plugin/SurfaceFooterButtons.vue';
import PublicAlbumGallery from './PublicAlbumGallery.vue';

const props = defineProps<{
  link: PublicLinkStore<PublicShareInfo> & {
    downloadUrl: (o?: { zip?: boolean; path?: string }) => string;
    noJsUrl: () => string;
  };
  locale: LocaleCode | string;
  theme?: ThemeMode;
}>();

const emit = defineEmits<{
  (e: 'navigate', path: string): void;
}>();

const { t, formatSize, formatDate } = useLocale(() => props.locale);
/* The link page's own sentences are the server's (server.public.*), the same
   ones its no-JavaScript page says (composables/usePublicText). */
const { pt } = usePublicText(() => String(props.locale));

const info = computed(() => props.link.info.value);
const kind = computed(() => info.value?.kind ?? 'file');
const node = computed(() => info.value?.node ?? null);
const app = computed(() => info.value?.app ?? null);
const remoteEntries = ref<PublicEntry[]>([]);
const entries = computed<PublicEntry[]>(() => {
  if (info.value?.entries && info.value.entries.length) return info.value.entries;
  return remoteEntries.value;
});
/** #104: the storage could not answer for the item - nothing to download. */
const unavailable = computed(() => info.value?.unavailable === true);

/**
 * A picture is shown, everything else is offered.
 *
 * NOTE: Deliberately narrow. A public page renders bytes a stranger was sent by
 * a stranger; an <img> is a decode the browser does every day, while an
 * inline frame of arbitrary content on our origin is not something a share
 * link should be able to ask for.
 */
const isImage = computed(() => (node.value?.mime ?? '').startsWith('image/'));

/** The path back up, or null at the share's root. */
const parent = computed(() => {
  const p = (info.value?.path ?? '').replace(/\/+$/, '');
  if (!p) return null;
  const i = p.lastIndexOf('/');
  return i <= 0 ? '' : p.slice(0, i);
});

// Photo Album Gallery integration (web/sharing_source)
const albumManifest = ref<AlbumData | null>(null);
const isAlbumView = ref(true);
const isCheckingAlbum = ref(true);

const resolvedToken = computed(() => {
  if (typeof window !== 'undefined') {
    const match = window.location.pathname.match(/\/s\/([a-zA-Z0-9_\-]+)/);
    if (match) return match[1];
  }
  return '';
});

const imageEntries = computed(() => {
  const exts = ['jpg', 'jpeg', 'png', 'webp', 'gif', 'heic', 'avif'];
  return entries.value.filter((e) => {
    if (e.is_dir) return false;
    const fn = e.name || '';
    const ext = (fn.split('.').pop() || '').toLowerCase();
    const isMimeImg = (e.mime || '').startsWith('image/');
    return isMimeImg || exts.includes(ext);
  });
});

function normalizeSharePhotoUrl(rawUrl: string | undefined, filename: string, isDownload = false): string {
  const currentToken = resolvedToken.value;
  // If rawUrl is an internal share link (e.g., /s/<old_token>/f/<filename> or http.../s/<old_token>/f/<filename>)
  if (rawUrl && rawUrl.includes('/s/')) {
    const match = rawUrl.match(/\/s\/[a-zA-Z0-9_\-]+\/f\/(.+)$/);
    if (match && currentToken) {
      const cleanPath = match[1].replace(/\?.*$/, '');
      const query = isDownload ? '?download=1' : '';
      return `/s/${currentToken}/f/${cleanPath}${query}`;
    }
  }

  // If rawUrl is an external image URL (e.g. https://images.unsplash.com/...)
  if (rawUrl && (rawUrl.startsWith('https://') || rawUrl.startsWith('http://')) && !rawUrl.includes('/s/')) {
    return rawUrl;
  }

  // If filename is provided, build from current token
  if (filename && currentToken) {
    const encoded = filename.split('/').map(encodeURIComponent).join('/');
    const query = isDownload ? '?download=1' : '';
    return `/s/${currentToken}/f/${encoded}${query}`;
  }

  return props.link.downloadUrl({ path: filename });
}

const fallbackAlbum = computed<AlbumData | null>(() => {
  if (albumManifest.value) return albumManifest.value;
  if (!imageEntries.value.length) return null;
  return {
    title: node.value?.name || info.value?.subject || 'Photo Collection',
    clientName: 'Collection',
    coverStyle: 'haduanh',
    items: imageEntries.value.map((e, idx) => ({
      id: `img_${idx}_${e.name}`,
      filename: e.name,
      size: e.size,
      thumbnailUrl: normalizeSharePhotoUrl('', e.name, false),
      previewUrl: normalizeSharePhotoUrl('', e.name, false),
      originalUrl: normalizeSharePhotoUrl('', e.name, true),
    })),
  };
});

const activeAlbum = computed(() => albumManifest.value || fallbackAlbum.value);

async function loadEntries() {
  const token = resolvedToken.value;
  if (kind.value !== 'folder' || !token) return;
  try {
    const res = await fetch(`/api/public/s/${token}/entries`);
    if (res.ok) {
      const data = await res.json();
      if (Array.isArray(data?.entries)) {
        remoteEntries.value = data.entries;
      }
    }
  } catch {
    // ignore
  }
}

async function checkAlbumManifest() {
  if (kind.value !== 'folder') {
    await new Promise<void>((r) => setTimeout(r, 800));
    isCheckingAlbum.value = false;
    return;
  }
  isCheckingAlbum.value = true;
  try {
    await loadEntries();
  const candidates = ['album_manifest.json', 'album.json'];
  for (const name of candidates) {
    try {
      const url = props.link.downloadUrl({ path: name });
      const res = await fetch(url);
      if (res.ok) {
        const text = await res.text();
        const json = JSON.parse(text);
        const alb = json?.data?.album || json?.album || (json?.data?.type === 'ALBUM' ? json.data : null);
        if (alb && alb.items && alb.items.length) {
          const manifestItems = [...alb.items];
          const existingNames = new Set(manifestItems.map((it: any) => it.filename));
          for (const img of (imageEntries.value || [])) {
            if (!existingNames.has(img.name)) {
              manifestItems.push({
                id: `img_new_${manifestItems.length + 1}_${img.name}`,
                filename: img.name,
                size: img.size,
                position: manifestItems.length + 1,
                previewUrl: normalizeSharePhotoUrl('', img.name, false),
                thumbnailUrl: normalizeSharePhotoUrl('', img.name, false),
                originalUrl: normalizeSharePhotoUrl('', img.name, true),
                isFavorite: false,
                favoriteCount: 0,
              });
            }
          }
          albumManifest.value = {
            ...alb,
            coverUrl: alb.coverUrl
              ? normalizeSharePhotoUrl(alb.coverUrl, manifestItems[0]?.filename || '', false)
              : (manifestItems[0] ? normalizeSharePhotoUrl('', manifestItems[0].filename, false) : ''),
            items: manifestItems.map((it: any) => ({
              ...it,
              previewUrl: normalizeSharePhotoUrl(it.previewUrl, it.filename, false),
              thumbnailUrl: normalizeSharePhotoUrl(it.thumbnailUrl || it.previewUrl, it.filename, false),
              originalUrl: normalizeSharePhotoUrl(it.originalUrl || it.previewUrl, it.filename, true),
            })),
          };
          isAlbumView.value = true;
          return;
        }
      }
    } catch {
      // try next candidate
    }
  }
  } finally {
    await new Promise<void>((r) => setTimeout(r, 800));
    isCheckingAlbum.value = false;
  }
}

watch(
  () => [kind.value, info.value?.path],
  () => {
    if (kind.value === 'folder') {
      void checkAlbumManifest();
    }
  },
  { immediate: true },
);
</script>

<template>
  <!-- ── an app plugin's screen ─────────────────────────────────────── -->
  <template v-if="kind === 'app'">
    <!-- WARNING: A document in an app's screen (a signing link's "see and
         approve" step) is fitted to the WINDOW, as the in-app page fits it —
         never drawn at the card's full width inside a fixed-height pane.
         Measured on a signing link, 2026-09-21: the page was 676px wide in a
         615px-tall pane, the signer saw the top 60% of it and none of their
         own boxes, which sat at the bottom — the owner's "arayüz berbat". -->
    <SurfaceConversation
      :conv="link.conv"
      :locale="locale as LocaleCode"
      :theme="theme"
      :file-url="link.resolveFile"
      :toast="link.toast.value"
      layout="page"
      page-height="min(80vh, 1000px)"
    />
    <div v-if="link.conv.footer.value.length" class="fe-ppage__actions">
      <SurfaceFooterButtons :conv="link.conv" :locale="locale as LocaleCode" testid-prefix="public-page" />
    </div>
    <!-- NOTE: The files an app exposes are drawn the way a SHARED FILE is —
         its name and size, and a Download button — because to the person
         holding the link that is what they are: a signer's copy of the
         document is a file somebody shared with them. It used to be a
         "Documents" heading over a bare link, a second way of saying the
         same thing on what should be the same page. -->
    <section v-if="app?.files?.length" class="fe-ppage__appfiles" data-testid="public-page-files">
      <div v-for="f in app?.files ?? []" :key="f.ref" class="fe-ppage__file-one fe-ppage__appfile">
        <p class="fe-surface__text">
          <strong>{{ f.name }}</strong>
          <span v-if="f.size" class="fe-ppage__filesize"> · {{ formatSize(f.size) }}</span>
        </p>
        <a
          class="fe-btn"
          :href="link.fileDownloadUrl(f.ref)"
          download
          :title="t('plugin.page.open_file', { name: f.name })"
          :data-testid="`public-page-file-${f.ref}`"
        >
          {{ t('ctx.download') }}
        </a>
      </div>
    </section>
  </template>

  <!-- ── #104: the storage could not answer for it ─────────────────────── -->
  <template v-else-if="unavailable">
    <div class="fe-ppage__file-one" data-testid="public-share-unavailable">
      <p v-if="!node?.name" class="fe-surface__text">
        <strong>{{ info?.subject }}</strong>
      </p>
      <p class="fe-surface__text fe-surface__text--muted" role="status">{{ pt('err_unavailable_body') }}</p>
    </div>
  </template>

  <!-- ── a folder ───────────────────────────────────────────────────── -->
  <template v-else-if="kind === 'folder'">
    <!-- 1. Luxury Loading Screen while checking and preloading album -->
    <div v-if="isCheckingAlbum && isAlbumView" class="fe-share-splash-loading">
      <div class="fe-share-splash-spinner"></div>
      <div class="fe-share-splash-text">Loading album...</div>
    </div>

    <!-- 2. Album View when active -->
    <PublicAlbumGallery
      v-else-if="activeAlbum && isAlbumView"
      :album="activeAlbum"
      :token="resolvedToken"
      :zip-url="link.downloadUrl({ zip: true })"
      :locale="locale"
      @switch-to-file-view="isAlbumView = false"
    />

    <!-- 3. Normal File Listing View -->
    <div v-else class="fe-ppage__file-one" data-testid="public-share-folder">
      <div v-if="activeAlbum" style="display: flex; justify-content: flex-end; margin-bottom: 14px;">
        <button type="button" class="fe-btn fe-btn--sm fe-btn--primary" @click="isAlbumView = true">
          View as Photo Album
        </button>
      </div>
      <!-- Same rule as a document: the heading is the folder's name already. -->
      <p v-if="!node?.name" class="fe-surface__text">
        <strong>{{ info?.subject }}</strong>
      </p>
      <nav v-if="info?.path" class="fe-ppage__crumbs" data-testid="public-share-crumbs">
        <button type="button" class="fe-btn fe-btn--sm" @click="emit('navigate', parent ?? '')">
          {{ pt('folder_up') }}
        </button>
        <span class="fe-ppage__crumb">{{ info?.path }}</span>
      </nav>

      <!-- NOTE: The server does not send a listing for a folder share yet: it
           answers the folder's NAME and leaves the walking to the no-JS page.
           The listing is drawn the moment one arrives; until then the two
           things a visitor actually came for are here, and the third is one
           link away. Writing a SECOND browse implementation against a
           different endpoint is what v3 exists to stop. -->
      <ul v-if="entries.length" class="fe-ppage__files" data-testid="public-share-entries">
        <li v-for="e in entries" :key="e.path" class="fe-ppage__file" :data-dir="e.is_dir ? '1' : undefined">
          <button
            v-if="e.is_dir"
            type="button"
            class="fe-ppage__entry"
            :data-testid="`public-entry-${e.name}`"
            @click="emit('navigate', e.path)"
          >
            <bdi>{{ e.name }}</bdi>
          </button>
          <a
            v-else
            :href="e.url || link.downloadUrl({ path: e.path })"
            class="fe-ppage__entry"
            :data-testid="`public-entry-${e.name}`"
            download
          ><bdi>{{ e.name }}</bdi></a>
          <span v-if="!e.is_dir && e.size !== undefined" class="fe-ppage__filesize">{{ formatSize(e.size) }}</span>
          <span v-if="e.modified" class="fe-ppage__filesize">{{ formatDate(e.modified) }}</span>
        </li>
      </ul>

      <div class="fe-ppage__actions">
        <a class="fe-btn fe-btn--primary" :href="link.downloadUrl({ zip: true })" data-testid="public-share-zip">
          {{ pt('folder_zip') }}
        </a>
        <a v-if="!entries.length" class="fe-btn" :href="link.noJsUrl()" data-testid="public-share-browse">
          {{ pt('browse') }}
        </a>
      </div>
    </div>
  </template>

  <!-- ── one document ───────────────────────────────────────────────── -->
  <template v-else>
    <div class="fe-ppage__file-one" data-testid="public-share-file">
      <img v-if="isImage" class="fe-ppage__shot" :src="link.downloadUrl()" :alt="node?.name ?? ''" />
      <!-- NOTE: The name is NOT repeated here. The shell's heading is already the
           file's own name (`PublicLinkPage.title`), and printing it again
           under itself — "brand-guidelines.pdf" over "brand-guidelines.pdf ·
           791 B" — is the stutter the levelled-down page shipped with. Only
           the size, and the name as a fallback for a link that has none. -->
      <p class="fe-surface__text fe-surface__text--muted" data-testid="public-share-fileline">
        <strong v-if="!node?.name">{{ info?.subject }}</strong>
        <span v-if="node?.size !== undefined" class="fe-ppage__filesize">{{ formatSize(node?.size) }}</span>
      </p>
      <a class="fe-btn fe-btn--primary" :href="link.downloadUrl()" data-testid="public-share-download">
        {{ t('ctx.download') }}
      </a>
    </div>
  </template>
</template>

<style>
/* Full bleed immersive experience for AIFS Public Album Gallery */
.fe-ppage:has(.fe-aifs-gallery-root) {
  padding: 0 !important;
  max-width: 100vw !important;
  width: 100% !important;
  background: transparent !important;
  min-height: 100vh !important;
  margin: 0 !important;
}
.fe-ppage:has(.fe-aifs-gallery-root) .fe-ppage__brand,
.fe-ppage:has(.fe-aifs-gallery-root) .fe-ppage__badge,
.fe-ppage:has(.fe-aifs-gallery-root) .fe-ppage__title,
.fe-ppage:has(.fe-aifs-gallery-root) .fe-ppage__subtitle,
.fe-ppage:has(.fe-aifs-gallery-root) .fe-ppage__footer {
  display: none !important;
}
.fe-ppage:has(.fe-aifs-gallery-root) .fe-ppage__card {
  max-width: 100% !important;
  width: 100% !important;
  padding: 0 !important;
  margin: 0 !important;
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
  border-radius: 0 !important;
}
</style>



<style>
.fe-share-splash-loading {
  position: fixed;
  inset: 0;
  z-index: 99999;
  background-color: var(--fe-bg, #FFFDF8);
  color: var(--fe-text, #171717);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
}
.fe-share-splash-spinner {
  width: 38px;
  height: 38px;
  border-radius: 50%;
  border: 3px solid rgba(255, 198, 41, 0.2);
  border-top-color: #FFC629;
  animation: aifsSpin 0.75s linear infinite;
}
.fe-share-splash-text {
  font-size: 13px;
  font-weight: 500;
  color: var(--fe-text-muted, #686868);
  font-family: var(--fe-font, 'Inter', sans-serif);
}
@keyframes aifsSpin {
  to { transform: rotate(360deg); }
}
</style>
