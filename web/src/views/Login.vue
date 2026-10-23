<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { stashDesktopHandoff } from '@/lib/desktopHandoff';
import { useRoute, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import {
  Lock,
  Mail,
  Github,
  Sparkles,
  ArrowRight,
  KeyRound,
  Eye,
  EyeOff,
  Check,
  Box,
  Building2,
} from 'lucide-vue-next';

import { useAuthStore } from '@/stores/auth';
import { useCapabilitiesStore } from '@/stores/capabilities';
import { AuthApi } from '@/api/auth';
import type { LoginMethods, LoginSSO } from '@/api/types';
import { useLoginRefusal } from '@/composables/useLoginRefusal';
import { useTenancy } from '@/composables/useTenancy';
import { ssoRefusalKey, ssoRefusalKeyOf } from '@/lib/ssoRefusal';
import { BrandingApi, type BrandingConfig } from '@/api/branding'; /* wiring:e1 */
import { accentButtonStyle } from '@/lib/accentButton';
import { liveTheme } from '@/lib/theme';
import { handoffTarget, navigateTo, readHandoffFragment } from '@/lib/realmHandoff';
import { appBase, productVersionLine } from '@brftech/filex-core';

import LogoMark from '@/components/LogoMark.vue';
import Button from '@/components/ui/Button.vue';
import Input from '@/components/ui/Input.vue';
import Checkbox from '@/components/ui/Checkbox.vue';

// gorunum:v1 — the sign-in card is painted from the product's own palette
// (`--fe-*`), and those variables are DECLARED by the core stylesheet. The
// admin bundle does not load it globally: measured on /admin/login before this
// change, `getComputedStyle(document.documentElement).getPropertyValue('--fe-bg')`
// came back as the empty string. Same import Home.vue already carries, for the
// same reason.
import '@brftech/filex-core/style.css';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const caps = useCapabilitiesStore();

const email = ref('');
const password = ref('');
// The tenant realm (docs/MULTI-TENANCY.md, Realms). The server says whether
// there is one at all (a multi-tenant install, composables/useTenancy - with
// the mode off there is no field and no realm is sent) and, on a tenant's own
// address, which: the field then arrives filled in and read-only.
const realm = ref('');
const tenancy = useTenancy();
const realmEnabled = computed(() => tenancy.realm.value !== null);
const lockedRealm = computed(() => tenancy.realm.value?.locked_realm ?? null);
const realmLocked = computed(() => realmEnabled.value && !!lockedRealm.value);
watch(
  lockedRealm,
  (v) => {
    if (v) realm.value = v;
  },
  { immediate: true },
);
// A sign-in is being carried to (or arriving from) a tenant's own address.
const handingOver = ref(false);
const totp = ref('');
const remember = ref(true);
const showTotp = ref(false);
const showPassword = ref(false);
const showSignIn = ref(false);
const localError = ref<string | null>(null);
// A refused attempt says more than "no": the tries left before a lock, or how
// long the lock still holds (a countdown that ends by itself).
const { apply: applyRefusal, text: refusalText, locked: signInLocked, time: lockTime } = useLoginRefusal();
const shownError = computed(() => refusalText.value ?? localError.value);

/** After a failed attempt: what the server said, and the lock's countdown if there is one. */
function failed() {
  applyRefusal(auth.refusal);
  localError.value = auth.refusal?.locked ? null : (auth.error ?? t('login.errGeneric'));
}

/*
 * The SSO buttons (docs/TENANT-ADMIN.md): the address's own list arrives with
 * the page (`auth_sso`: the tenant the address names, else the platform's
 * own). On the platform's page, typing a realm asks the server for THAT
 * realm's (GET /api/auth/methods): a realm with no SSO, and a realm nobody
 * has, answer the same way, with no buttons. An older server says nothing
 * but `auth_drivers`: one button, as before.
 */
const realmMethods = ref<LoginMethods | null>(null);
let methodsTimer: ReturnType<typeof setTimeout> | undefined;
let methodsSeq = 0;
watch(realm, (v) => {
  if (!realmEnabled.value || realmLocked.value) return;
  if (methodsTimer) clearTimeout(methodsTimer);
  const typed = v.trim();
  if (!typed) {
    methodsSeq++;
    realmMethods.value = null;
    return;
  }
  methodsTimer = setTimeout(() => void askMethods(typed), 300);
});
async function askMethods(typed: string) {
  const seq = ++methodsSeq;
  try {
    const m = await AuthApi.methods(typed);
    if (seq === methodsSeq) realmMethods.value = m;
  } catch {
    if (seq === methodsSeq) realmMethods.value = null;
  }
}
onBeforeUnmount(() => methodsTimer && clearTimeout(methodsTimer));
const ssoChoices = computed<LoginSSO[]>(() => {
  if (realmMethods.value) return realmMethods.value.sso;
  if (Array.isArray(caps.data.auth_sso)) return caps.data.auth_sso;
  return caps.data.auth_drivers.includes('oidc') ? [{ id: '', label: '' }] : [];
});
const oidcEnabled = computed(() => ssoChoices.value.length > 0);
const localEnabled = computed(
  () => caps.data.auth_drivers.length === 0 || caps.data.auth_drivers.includes('local'),
);
const demoMode = computed(() => caps.data.demo_mode === true);
const demoUser = computed(() => caps.data.demo_user || 'demo@demo.com');
// ⚠ Both halves of the credentials come from the server (FILEX_DEMO_USER /
// FILEX_DEMO_PASS). The password used to be hardcoded here, so an operator
// who set FILEX_DEMO_PASS broke the CTA and the printed hint at once — the
// button submitted "demo" against a user whose password was no longer that.
// The fallback keeps installs that never set the variable working.
const demoPass = computed(() => caps.data.demo_pass || 'demo');

// SSO-first mode (FILEX_OIDC_AUTO_REDIRECT): unauthenticated visitors go
// straight to the IdP; the password form hides behind a "sign in with
// password" link (?local=1) for break-glass/admin logins.
const wantLocal = computed(() => route.query.local !== undefined);
// A failed SSO round trip (`?error=oidc`, or `?maintenance=1` when the account
// may not open a session now): the sentence that says what happened, picked by
// the reason code the server sent (lib/ssoRefusal). Else the reason the
// server gave for refusing the person the header proxy named (/api/auth/me,
// stores/auth `signInReason`). Null on any other visit.
const ssoFailure = computed(() => ssoRefusalKey(route.query) ?? ssoRefusalKeyOf(auth.signInReason));
// Straight after "Sign out" (lib/signOut, or the IdP sending the browser back
// once it has ended its own session). The page says so and does NOT start SSO
// by itself: wherever the IdP's session outlived the sign-out (an IdP without
// RP-initiated logout, FILEX_OIDC_LOGOUT=local) that would sign the same
// account straight back in — which is what "Sign out" used to do.
const signedOut = computed(() => route.query.signed_out !== undefined);
const autoRedirect = computed(
  () => caps.data.oidc_auto_redirect === true && oidcEnabled.value && !demoMode.value,
);
// Recovery sign-in (FILEX_AUTH_RECOVERY_LOGIN, on by default): password sign-in
// is off on this server, but the administrator created at installation may
// still use it — for the day the identity provider is down. The form is not
// offered to everybody; it waits behind the same ?local=1 link, and says who it
// is for.
const recoveryLogin = computed(
  () => caps.data.auth_recovery_login === true && !localEnabled.value && !demoMode.value,
);
const showLocalForm = computed(
  () =>
    (localEnabled.value && (!autoRedirect.value || wantLocal.value)) ||
    (recoveryLogin.value && wantLocal.value),
);
// The link to the password form: offered on an SSO-first page, and on a page
// where only recovery can use it — one link, labelled for whichever it is.
const showPasswordLink = computed(
  () => !wantLocal.value && ((localEnabled.value && autoRedirect.value) || recoveryLogin.value),
);
const redirecting = ref(false);

/* wiring:e1 — settings-driven branding on the login screen: custom logo
   replaces the LogoMark, the display name becomes the wordmark beside it, and
   the accent colors the primary CTA (inline style — the token palette is the
   product's, the accent is the operator's). Fetch is public and best-effort:
   default look on failure. */
const branding = ref<BrandingConfig | null>(null);
/* issue #29 — the accent-filled button is designed per theme: its label is
   picked from the accent, and it draws an edge whenever the fill does not
   stand out from the card of the theme it is shown in (lib/accentButton).
   The theme is lib/theme's liveTheme, the mode the window is painted in: a
   theme switch in another tab or the OS repaints the button without a
   reload. */
const isDark = computed(() => liveTheme.value === 'dark');
const accentStyle = computed(() => accentButtonStyle(branding.value?.accent, isDark.value));
/* issue #28 — the operator's own label for the SSO button; the translated
   default otherwise. */
const ssoLabel = computed(() => branding.value?.sso_label?.trim() || t('login.oidc'));
/** A button's words: the provider's own label, else the page's. */
const ssoText = (c: LoginSSO) => c.label?.trim() || ssoLabel.value;
const wordmark = computed(() => branding.value?.name?.trim() || 'filex');
async function fetchBranding() {
  try {
    branding.value = await BrandingApi.get();
  } catch {
    branding.value = null;
  }
}

onMounted(async () => {
  void fetchBranding(); /* wiring:e1 */
  // Desktop hand-off: remember it NOW. The OIDC round-trip wipes the query
  // string, so reading these later would only ever work for password logins —
  // i.e. exactly the case the desktop flow does not need.
  stashDesktopHandoff(route.query.desktop_state, route.query.desktop_challenge);
  // A sign-in handed over from the platform's address (lib/realmHandoff): the
  // ticket leaves the address bar first, then becomes the session here.
  const ticket = typeof window !== 'undefined' ? readHandoffFragment(window.location.hash) : null;
  if (ticket) {
    window.history.replaceState(window.history.state, '', window.location.pathname + window.location.search);
    handingOver.value = true;
    const ok = await auth.redeemHandoff(ticket);
    handingOver.value = false;
    if (ok) {
      router.push((route.query.redirect as string) || '/');
      return;
    }
    localError.value = auth.error ?? t('login.errHandoff');
  }
  if (!caps.loaded) await caps.fetch();
  // Loop guards: never auto-redirect when the visitor explicitly asked for
  // the password form (?local=1), when the IdP round-trip just failed
  // (?error=... — redirecting again would loop), when the tenant is
  // locked out (?maintenance=1), or right after signing out (?signed_out —
  // see `signedOut`), or when the server has just said why it refuses the
  // person the header proxy named (`signInReason`). The OIDC callback itself
  // is a backend route, so the SPA never mounts on it.
  if (
    autoRedirect.value &&
    !wantLocal.value &&
    !signedOut.value &&
    route.query.error === undefined &&
    route.query.maintenance === undefined &&
    !auth.signInReason
  ) {
    redirecting.value = true;
    startOidc();
  }
});

async function submit() {
  if (signInLocked.value) return;
  localError.value = null;
  applyRefusal(null);
  const ok = await auth.login({
    email: email.value.trim(),
    password: password.value,
    remember: remember.value,
    totp: totp.value || undefined,
    realm: realmEnabled.value ? (lockedRealm.value ?? realm.value.trim()) || undefined : undefined,
  });
  if (ok) {
    const redirect = (route.query.redirect as string) || '/';
    router.push(redirect);
  } else if (auth.handoff) {
    goToTenant();
  } else {
    failed();
  }
}

/** The sign-in succeeded for a tenant with an address of its own: finish it
 *  there (lib/realmHandoff). */
function goToTenant() {
  const target = auth.handoff
    ? handoffTarget(auth.handoff, window.location, appBase())
    : null;
  if (!target) {
    localError.value = t('login.errHandoff');
    return;
  }
  handingOver.value = true;
  navigateTo(target);
}

async function openDemo() {
  // Auto-submit the documented demo creds, then hand off to the
  // standalone /explore page (FileExplorer Web Component) — NOT
  // the admin dashboard. Demo visitors should see the actual file
  // browser, not the operator panel.
  localError.value = null;
  applyRefusal(null);
  const ok = await auth.login({
    email: demoUser.value,
    password: demoPass.value,
    remember: true,
    realm: realmEnabled.value ? lockedRealm.value || undefined : undefined,
  });
  if (ok) {
    router.push({ name: 'explore' });
  } else {
    failed();
  }
}

/** Starts one SSO (the first when none is named) for the realm typed, if any. */
function startOidc(c?: LoginSSO) {
  const choice = c ?? ssoChoices.value[0];
  const typed = realmEnabled.value ? (lockedRealm.value ?? realm.value.trim()) : '';
  window.location.href = AuthApi.oidcStartUrl('oidc', (route.query.redirect as string) || '/admin/', {
    instance: choice?.id || undefined,
    realm: typed || undefined,
  });
}
</script>

<template>
  <!-- The zinc ground belongs to the demo landing only: the sign-in page
       paints its own from the fe-bg-elev token. -->
  <div class="min-h-screen" :class="demoMode ? 'bg-[var(--fe-bg-elev)]' : ''">
    <!-- ⚠ A language switcher and a theme toggle used to float in the top-right
         corner here. Both were removed on the owner's instruction
         (2026-09-12): language and theme are profile settings, like the
         palette and the density, and a signed-out visitor has no profile to
         open — so the page decides for itself and gets it right.

         Measured rather than assumed, in both directions:
           • language — `getStoredLocale()` honours a previously stored
             `filex.locale` and otherwise follows `navigator.language`, so a
             Turkish browser lands on a Turkish form;
           • theme — `getStoredTheme()` returns 'auto' and `applyStoredTheme()`
             resolves that through `prefers-color-scheme`, so an OS in dark
             mode gets a dark sign-in page.
         ⚠⚠ The theme line USED to end "a stored choice still outranks the OS,
         both ways round", and that was the bug rather than the feature
         (owner, 2026-09-21): the stored choice belongs to whoever signed in at
         this browser last, so a shared machine showed the next person the
         previous one's mode — and, through `filex.palette`, their palette,
         which also suppressed the operator's own house theme on the one page
         every customer sees first. With no session the PERSON is not read at
         all now (`lib/prefs` → SESSION_LS_KEY): the browser decides light or
         dark, and the INSTANCE decides the palette.
         The only place either one is chosen is the user-settings panel. -->
    <!-- ─────────── Demo mode landing (minimalist) ─────────── -->
    <div v-if="demoMode" class="mx-auto max-w-md px-4 py-12 sm:py-20">
      <div class="card p-6 sm:p-8 text-center shadow-lg border border-zinc-200/80 dark:border-zinc-800/80 bg-white dark:bg-zinc-900 rounded-2xl">
        <LogoMark class="mx-auto h-12 w-12" />
        <div class="mt-3 inline-flex items-center gap-1.5 rounded-full bg-brand-50 dark:bg-brand-500/10 px-2.5 py-0.5 text-xs font-medium text-brand-700 dark:text-brand-300">
          <Sparkles class="h-3 w-3" />
          {{ t('demo.badge') }}
        </div>
        <h1 class="mt-3 text-2xl font-bold tracking-tight text-zinc-900 dark:text-zinc-100">
          {{ t('demo.title') }}
        </h1>
        <p class="mt-1.5 text-xs text-zinc-500 dark:text-zinc-400">
          {{ t('demo.subtitle') }}
        </p>

        <div class="mt-6 flex flex-col gap-2.5">
          <Button size="md" variant="primary" @click="openDemo" :loading="auth.loading" class="w-full justify-center">
            <Sparkles class="h-4 w-4" />
            {{ t('demo.openCta') }}
            <ArrowRight class="h-4 w-4" />
          </Button>
          <button
            type="button"
            class="text-xs text-zinc-500 hover:text-zinc-800 dark:text-zinc-400 dark:hover:text-zinc-200 transition-colors py-1"
            @click="showSignIn = !showSignIn"
          >
            {{ showSignIn ? t('demo.hideSignIn') : t('demo.showSignIn') }}
          </button>
        </div>

        <p class="mt-3 text-[11px] text-zinc-400 dark:text-zinc-500">
          {{ t('demo.creds', { email: demoUser, password: demoPass }) }}
        </p>

        <!-- Optional sign-in form (hidden behind toggle) -->
        <div v-if="showSignIn" class="mt-6 pt-5 border-t border-zinc-100 dark:border-zinc-800 text-left">
          <div class="mb-3 text-center">
            <h2 class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">{{ t('login.title') }}</h2>
            <p class="text-[11px] text-zinc-400">{{ t('demo.adminHint') }}</p>
          </div>
          <form v-if="localEnabled" class="space-y-3" @submit.prevent="submit">
            <Input
              v-if="realmEnabled"
              v-model="realm"
              type="text"
              autocomplete="organization"
              :readonly="realmLocked"
              :label="t('login.realm')"
              :hint="realmLocked ? t('login.realmLocked', { realm: lockedRealm }) : t('login.realmHint')"
              name="realm"
            />
            <Input v-model="email" type="text" autocomplete="username" required :label="t('login.identifier')" name="email" />
            <Input v-model="password" type="password" autocomplete="current-password" required :label="t('common.password')" name="password" />
            <Checkbox v-model="remember" :label="t('login.remember')" />
            <p v-if="shownError" class="text-xs text-rose-600 dark:text-rose-400 bg-rose-50 dark:bg-rose-500/10 rounded-md px-3 py-2" role="alert">
              {{ shownError }}
            </p>
            <Button type="submit" :loading="auth.loading" :disabled="signInLocked" block class="mt-2">
              <Lock class="h-4 w-4" />
              {{ signInLocked ? t('login.retryIn', { time: lockTime }) : t('login.submit') }}
            </Button>
          </form>
          <Button v-if="oidcEnabled" variant="outline" block class="mt-2" @click="startOidc()">
            <Github class="h-4 w-4" />
            {{ ssoLabel }}
          </Button>
        </div>
      </div>

      <p class="mt-6 text-center text-xs text-zinc-400 dark:text-zinc-600 inline-flex items-center justify-center gap-1 w-full">
        <Mail class="h-3 w-3" /> {{ productVersionLine(caps.data.release) }}
      </p>
    </div>

    <!-- ─────────── Standard sign-in (non-demo) ─────────── -->
    <!-- gorunum:v1 — the card, its type scale, the filled fields and the
         decorative corners follow the reference shell, painted entirely from
         the fe-* tokens: no Tailwind palette name and no raw hex on this page.

         ⚠ The page keeps a deep bottom padding rather than sitting dead
         centre. The PWA / desktop banner is fixed to the bottom centre of the
         viewport and this card's submit button is the only thing in that same
         place: measured at 390x844 with a desktop user agent,
         `document.elementFromPoint` over the button returned the banner's hint
         text, so the click never reached the button. The card is centred in
         what is left, so the padding lifts it clear rather than adding visible
         whitespace. Guarded by web/cypress/e2e/91-install-banner-login.cy.ts. -->
    <div v-else class="lg">
      <!-- Decorative corners. A fixed, clipped layer: on a short viewport the
           card is taller than the screen and the page has to scroll, and
           `overflow:hidden` on the scrolling element would have cut the card
           off instead of the shapes. -->
      <div class="lg-deco" aria-hidden="true">
        <span class="lg-deco__a" />
        <span class="lg-deco__b" />
      </div>

      <div class="lg-col">
        <div class="lg-card" data-install-clear>
          <div class="lg-brand">
            <!-- wiring:e1 — branded logo (a custom logo replaces the mark) -->
            <img v-if="branding?.logo_url" :src="branding.logo_url" alt="" class="lg-brand__logo" />
            <LogoMark v-else class="lg-brand__mark" />
            <span class="lg-brand__word">{{ wordmark }}</span>
          </div>

          <h1 class="lg-title">{{ t('login.title') }}</h1>
          <p class="lg-subtitle">{{ t('login.subtitle') }}</p>

          <p v-if="redirecting" class="lg-redirecting">
            <span class="lg-spinner" aria-hidden="true" />
            {{ t('login.redirecting') }}
          </p>

          <template v-else>
            <!-- A sign-in on its way to (or arriving from) a tenant's own
                 address. The form below is hidden, not removed: a ticket that
                 turns out spent brings it back with the reason. -->
            <p v-if="handingOver" class="lg-redirecting" data-testid="login-redirecting">
              <span class="lg-spinner" aria-hidden="true" />
              {{ t('login.handoffGoing') }}
            </p>
            <div v-show="!handingOver">
              <p v-if="ssoFailure" role="alert" class="lg-alert" data-testid="login-sso-error">
                {{ t(ssoFailure) }}
              </p>
              <p
                v-else-if="signedOut"
                role="status"
                class="lg-note"
                data-testid="login-signed-out"
              >
                {{ t('login.signedOut') }}
              </p>

              <!-- The tenant realm (multi-tenant installs only), above every way
                   in: it decides which SSO buttons there are. On a tenant's own
                   address it is that tenant's and read-only: the address
                   already says which. Empty on the platform's page means the
                   platform's own accounts. It belongs to the password form
                   (the form attribute), so Enter here signs in too. -->
              <div v-if="realmEnabled" class="lg-realm">
                <label class="lg-label" for="realm">{{ t('login.realm') }}</label>
                <label
                  class="lg-field lg-field--text"
                  :class="{ 'lg-field--locked': realmLocked }"
                  for="realm"
                >
                  <Building2 class="lg-field__icon" aria-hidden="true" />
                  <input
                    id="realm"
                    v-model="realm"
                    name="realm"
                    type="text"
                    form="login-form"
                    autocomplete="organization"
                    autocapitalize="none"
                    spellcheck="false"
                    :readonly="realmLocked"
                    :aria-readonly="realmLocked || undefined"
                    aria-describedby="realm-hint"
                    class="lg-field__input"
                    data-testid="login-realm"
                  />
                </label>
                <p id="realm-hint" class="lg-hint">
                  {{ realmLocked ? t('login.realmLocked', { realm: lockedRealm }) : t('login.realmHint') }}
                </p>
              </div>

              <!-- SSO is the primary path whenever there is one: one button per
                   identity provider of the realm. -->
              <button
                v-for="(c, i) in ssoChoices"
                :key="c.id || 'sso'"
                type="button"
                class="lg-btn lg-btn--sso"
                :class="i === 0 ? 'lg-btn--primary' : 'lg-btn--outline'"
                :style="i === 0 ? accentStyle : undefined"
                :data-testid="`login-sso-${c.id || 'default'}`"
                @click="startOidc(c)"
              >
                <KeyRound class="lg-i18" aria-hidden="true" />
                {{ ssoText(c) }}
              </button>

              <div v-if="showLocalForm && oidcEnabled" class="lg-or">
                <span class="lg-or__line" />
                <span class="lg-or__word">{{ t('login.or') }}</span>
                <span class="lg-or__line" />
              </div>

              <p v-if="recoveryLogin && showLocalForm" role="note" class="lg-subtitle" data-testid="login-recovery-note">
                {{ t('login.recoveryNote') }}
              </p>

              <form v-if="showLocalForm" id="login-form" class="lg-form" @submit.prevent="submit">
                <label class="lg-label" for="email">{{ t('login.identifier') }}</label>
                <label class="lg-field lg-field--text" for="email">
                  <Mail class="lg-field__icon" aria-hidden="true" />
                  <input
                    id="email"
                    v-model="email"
                    name="email"
                    type="text"
                    autocomplete="username"
                    required
                    class="lg-field__input"
                  />
                </label>

                <label class="lg-label lg-label--next" for="password">{{ t('common.password') }}</label>
                <div class="lg-field">
                  <label class="lg-field__grow" for="password">
                    <Lock class="lg-field__icon" aria-hidden="true" />
                    <input
                      id="password"
                      v-model="password"
                      name="password"
                      :type="showPassword ? 'text' : 'password'"
                      autocomplete="current-password"
                      required
                      class="lg-field__input lg-field__input--pw"
                    />
                  </label>
                  <!-- ⚠ The accessible name deliberately avoids the word
                       "password": e2e/helpers/auth.ts fills the field with
                       `getByLabel(/password|parola/i)`, and a second control
                       answering to that name turns every sign-in in the suite
                       into a strict-mode violation. -->
                  <button
                    type="button"
                    class="lg-eye"
                    :aria-label="showPassword ? t('login.hideChars') : t('login.showChars')"
                    :aria-pressed="showPassword"
                    @click="showPassword = !showPassword"
                  >
                    <EyeOff v-if="showPassword" class="lg-i20" aria-hidden="true" />
                    <Eye v-else class="lg-i20" aria-hidden="true" />
                  </button>
                </div>

                <div class="lg-2fa">
                  <button type="button" class="lg-link" @click="showTotp = !showTotp">
                    {{ showTotp ? t('login.hide2fa') : t('login.use2fa') }}
                  </button>
                </div>

                <template v-if="showTotp">
                  <label class="lg-label" for="totp">{{ t('login.totpLabel') }}</label>
                  <label class="lg-field lg-field--text" for="totp">
                    <KeyRound class="lg-field__icon" aria-hidden="true" />
                    <input
                      id="totp"
                      v-model="totp"
                      name="totp"
                      type="text"
                      inputmode="numeric"
                      autocomplete="one-time-code"
                      placeholder="123456"
                      class="lg-field__input"
                      aria-describedby="totp-hint"
                    />
                  </label>
                  <p id="totp-hint" class="lg-hint">{{ t('login.totpHint') }}</p>
                </template>

                <label class="lg-check">
                  <button
                    type="button"
                    role="checkbox"
                    :aria-checked="remember"
                    class="lg-check__box"
                    :class="{ 'is-on': remember }"
                    @click="remember = !remember"
                  >
                    <Check v-if="remember" class="lg-check__tick" aria-hidden="true" />
                  </button>
                  <span>{{ t('login.remember') }}</span>
                </label>

                <p v-if="shownError" role="alert" class="lg-alert lg-alert--form">
                  {{ shownError }}
                </p>

                <button
                  type="submit"
                  class="lg-btn"
                  :class="oidcEnabled ? 'lg-btn--outline' : 'lg-btn--primary'"
                  :style="oidcEnabled ? undefined : accentStyle"
                  :disabled="auth.loading || signInLocked"
                  :aria-busy="auth.loading || undefined"
                >
                  <span v-if="auth.loading" class="lg-spinner" aria-hidden="true" />
                  <Lock v-else class="lg-i18" aria-hidden="true" />
                  {{ signInLocked ? t('login.retryIn', { time: lockTime }) : t('login.submit') }}
                </button>
              </form>

              <div v-if="showPasswordLink" class="lg-localhint">
                <router-link
                  :to="{ query: { ...route.query, local: '1' } }"
                  class="lg-localhint__link"
                  data-testid="login-password-link"
                >
                  {{ recoveryLogin ? t('login.recovery') : t('login.local') }}
                </router-link>
              </div>

              <p v-if="!localEnabled && !oidcEnabled && !recoveryLogin" class="lg-alert lg-alert--form">
                <i18n-t keypath="login.noProviders" tag="span"><template #env><code>AUTH_DRIVERS</code></template></i18n-t>
              </p>
            </div>
          </template>
        </div>

        <!-- `data-install-keep`: the full install card must not stand on this
             line either (InstallPrompt.vue → checkFit; measured under the card
             at 1440×900, 2026-09-21). -->
        <p class="lg-version" data-install-keep>
          <Box class="lg-i16" aria-hidden="true" /> {{ productVersionLine(caps.data.release) }}
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* ── page ───────────────────────────────────────────────────────────── */
.lg {
  position: relative;
  display: flex;
  min-height: 100vh;
  width: 100%;
  align-items: center;
  justify-content: center;
  padding: 32px 16px calc(var(--filex-install-banner-h, 0px) + 24px);
  font-family: var(--fe-font);
  color: var(--fe-text);
  background: var(--fe-login-ground, var(--fe-bg));
}
@media (max-height: 960px) {
  .lg {
    padding-top: 16px;
  }
}

.lg-deco {
  position: fixed;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  background: radial-gradient(circle at 50% 25%, rgba(var(--fe-primary-rgb, 59, 130, 246), 0.04) 0%, transparent 60%);
}
.lg-deco__a,
.lg-deco__b {
  display: none;
}

.lg-col {
  position: relative;
  width: 100%;
  max-width: 400px;
}

/* ── card ───────────────────────────────────────────────────────────── */
.lg-card {
  --lg-pad-y: 36px;
  --lg-mark: 42px;
  --lg-word: 24px;
  --lg-title: 22px;
  --lg-gap: 20px;
  --lg-field-h: 44px;

  padding: var(--lg-pad-y) 28px;
  border-radius: 16px;
  background: var(--fe-login-card, var(--fe-bg-elev));
  border: 1px solid var(--fe-border);
  box-shadow: 0 4px 20px -2px rgba(0, 0, 0, 0.05), 0 2px 6px -1px rgba(0, 0, 0, 0.02);
  transition: all 0.2s ease;
}
@media (min-width: 640px) {
  .lg-card {
    padding: var(--lg-pad-y) 32px;
  }
}

.lg-brand {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
}
.lg-brand__mark,
.lg-brand__logo {
  height: var(--lg-mark);
  width: var(--lg-mark);
  flex-shrink: 0;
}
.lg-brand__logo {
  width: auto;
  max-width: 160px;
  object-fit: contain;
}
.lg-brand__word {
  min-width: 0;
  font-size: var(--lg-word);
  font-weight: 600;
  line-height: 1.2;
  letter-spacing: -0.02em;
  color: var(--fe-text);
  overflow-wrap: anywhere;
}

.lg-title {
  margin: 16px 0 0;
  text-align: center;
  font-size: var(--lg-title);
  font-weight: 600;
  line-height: 1.25;
  letter-spacing: -0.015em;
  color: var(--fe-text);
}
.lg-subtitle {
  margin: 6px 0 0;
  text-align: center;
  font-size: 13px;
  color: var(--fe-text-muted);
}

.lg-redirecting {
  margin: var(--lg-gap) 0 0;
  text-align: center;
  font-size: 13px;
  color: var(--fe-text-muted);
}
.lg-spinner {
  display: inline-block;
  height: 14px;
  width: 14px;
  margin-inline-end: 6px;
  vertical-align: -2px;
  border: 2px solid currentColor;
  border-inline-end-color: transparent;
  border-radius: 999px;
  opacity: 0.7;
  animation: lg-spin 0.7s linear infinite;
}
@keyframes lg-spin {
  to {
    transform: rotate(360deg);
  }
}

/* ── form ───────────────────────────────────────────────────────────── */
.lg-form {
  margin-top: 22px;
}
.lg-realm {
  margin-top: 16px;
}

.lg-label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--fe-text-muted);
}
.lg-label--next {
  margin-top: 16px;
}

.lg-field {
  margin-top: 6px;
  display: flex;
  align-items: center;
  height: var(--lg-field-h);
  padding: 0 14px;
  border: 1px solid var(--fe-border);
  border-radius: 10px;
  background: var(--fe-bg);
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}
.lg-field--text {
  cursor: text;
}
.lg-field--locked {
  cursor: default;
  background: var(--fe-bg-elev);
}
.lg-field--locked .lg-field__input {
  cursor: default;
  color: var(--fe-text-muted);
}
.lg-field:focus-within {
  border-color: var(--fe-primary);
  box-shadow: 0 0 0 3px rgba(var(--fe-primary-rgb, 59, 130, 246), 0.12);
}
.lg-field__grow {
  display: flex;
  align-items: center;
  height: 100%;
  min-width: 0;
  flex: 1;
  cursor: text;
}
.lg-field__icon {
  height: 17px;
  width: 17px;
  flex-shrink: 0;
  color: var(--fe-text-muted);
  opacity: 0.8;
}
.lg-field__input {
  margin-inline-start: 10px;
  min-width: 0;
  flex: 1;
  border: 0;
  background: transparent;
  font-family: inherit;
  font-size: 14px;
  color: var(--fe-text);
}
.lg-field__input--pw {
  margin-inline-end: 8px;
}
.lg-field__input:focus {
  outline: none;
}
.lg-field__input::placeholder {
  color: var(--fe-text-muted);
}

.lg-eye {
  display: flex;
  flex-shrink: 0;
  padding: 4px;
  border: 0;
  background: none;
  color: var(--fe-text-muted);
  cursor: pointer;
  border-radius: 6px;
  transition: color 0.15s ease;
}
.lg-eye:hover {
  color: var(--fe-text);
}

.lg-2fa {
  margin-top: 10px;
  text-align: end;
}
.lg-link {
  padding: 0;
  border: 0;
  background: none;
  font-family: inherit;
  font-size: 12px;
  color: var(--fe-primary);
  cursor: pointer;
  transition: opacity 0.15s;
}
.lg-link:hover {
  text-decoration: underline;
}

.lg-hint {
  margin: 5px 0 0;
  font-size: 11px;
  color: var(--fe-text-muted);
}

.lg-check {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 16px;
  font-size: 13px;
  color: var(--fe-text);
  cursor: pointer;
  user-select: none;
}
.lg-check__box {
  position: relative;
  display: inline-flex;
  height: 17px;
  width: 17px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--fe-border-strong);
  border-radius: 4px;
  background: var(--fe-bg);
  cursor: pointer;
  transition: all 0.15s ease;
}
.lg-check__box.is-on {
  border-color: var(--fe-primary);
  background: var(--fe-primary);
  color: var(--fe-text-on-primary);
}
.lg-check__tick {
  height: 12px;
  width: 12px;
  stroke-width: 3;
}

.lg-alert {
  margin: 16px 0 0;
  padding: 10px 12px;
  border: 1px solid var(--fe-danger);
  border-radius: 8px;
  background: var(--fe-bg-elev);
  font-size: 13px;
  color: var(--fe-danger);
}
.lg-alert--form {
  margin: 18px 0 0;
}
.lg-note {
  margin: 16px 0 0;
  padding: 10px 12px;
  border: 1px solid var(--fe-border);
  border-radius: 8px;
  background: var(--fe-bg-elev);
  font-size: 13px;
  color: var(--fe-text-muted);
}
.lg-alert code {
  font-family: var(--fe-font-mono);
}

/* ── buttons ────────────────────────────────────────────────────────── */
.lg-btn {
  display: inline-flex;
  width: 100%;
  height: 42px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 20px;
  padding: 0 16px;
  border: 1px solid transparent;
  border-radius: 10px;
  font-family: inherit;
  font-size: 14px;
  font-weight: 500;
  line-height: 1;
  cursor: pointer;
  user-select: none;
  transition: all 0.15s ease;
}
.lg-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.lg-btn--sso {
  margin-top: 16px;
}
.lg-btn--primary {
  background: var(--fe-primary);
  border-color: var(--fe-primary);
  color: var(--fe-text-on-primary);
}
.lg-btn--primary:hover:not(:disabled) {
  background: var(--fe-primary-hover);
  border-color: var(--fe-primary-hover);
}
.lg-btn--outline {
  background: var(--fe-bg);
  border-color: var(--fe-border);
  color: var(--fe-text);
}
.lg-btn--outline:hover:not(:disabled) {
  background: var(--fe-bg-hover);
  border-color: var(--fe-border-strong);
}

.lg-or {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 18px 0;
}
.lg-or__line {
  flex: 1;
  border-top: 1px solid var(--fe-border);
}
.lg-or__word {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--fe-text-muted);
}

.lg-localhint {
  margin-top: 18px;
  text-align: center;
}
.lg-localhint__link {
  font-size: 12px;
  color: var(--fe-text-muted);
  text-decoration: none;
}
.lg-localhint__link:hover {
  color: var(--fe-text);
  text-decoration: underline;
}

/* ── version line ───────────────────────────────────────────────────── */
.lg-version {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  margin: 20px 0 0;
  font-size: 12px;
  color: var(--fe-text-muted);
  opacity: 0.75;
}

.lg-i16 {
  height: 15px;
  width: 15px;
}
.lg-i18 {
  height: 17px;
  width: 17px;
}
.lg-i20 {
  height: 18px;
  width: 18px;
}

.lg :focus-visible {
  outline: 2px solid var(--fe-primary);
  outline-offset: 2px;
}
</style>

<style>
@media (max-height: 880px) {
  html.filex-install-banner .lg-card {
    --lg-pad-y: 24px;
    --lg-mark: 36px;
    --lg-word: 22px;
    --lg-title: 20px;
    --lg-gap: 16px;
    --lg-field-h: 40px;
  }
}
</style>
