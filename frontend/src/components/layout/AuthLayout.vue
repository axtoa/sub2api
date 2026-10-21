<template>
  <div
    class="relative flex min-h-screen items-center justify-center overflow-hidden p-4"
    :class="{ 'auth-shell-best-ai': bestAiTheme }"
  >
    <!-- Background -->
    <div
      class="absolute inset-0 bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950"
      :class="{ 'auth-background-best-ai': bestAiTheme }"
    ></div>

    <!-- Decorative Elements -->
    <div v-if="!bestAiTheme" class="pointer-events-none absolute inset-0 overflow-hidden">
      <!-- Gradient Orbs -->
      <div
        class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/20 blur-3xl"
      ></div>
      <div
        class="absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-primary-500/15 blur-3xl"
      ></div>
      <div
        class="absolute left-1/2 top-1/2 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-300/10 blur-3xl"
      ></div>

      <!-- Grid Pattern -->
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(20,184,166,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(20,184,166,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"
      ></div>
    </div>

    <!-- Content Container -->
    <div class="relative z-10 w-full max-w-md">
      <!-- Logo/Brand -->
      <div class="mb-8 text-center">
        <!-- Custom Logo or Default Logo -->
        <template v-if="settingsLoaded || bestAiTheme">
          <div
            v-if="!bestAiTheme"
            class="mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl shadow-lg shadow-primary-500/30"
          >
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <div v-else class="auth-brand-mark mb-4" aria-hidden="true">B</div>
          <h1 class="text-gradient mb-2 text-3xl font-bold" :class="{ 'auth-title-best-ai': bestAiTheme }">
            {{ siteName }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400" :class="{ 'auth-subtitle-best-ai': bestAiTheme }">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <!-- Card Container -->
      <div class="card-glass rounded-2xl p-8 shadow-glass">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const props = withDefaults(defineProps<{
  brandName?: string
  brandSubtitle?: string
  bestAiTheme?: boolean
}>(), {
  bestAiTheme: false,
})

const appStore = useAppStore()

const siteName = computed(() => props.brandName || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => props.brandSubtitle || appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.text-gradient {
  @apply bg-gradient-to-r from-primary-600 to-primary-500 bg-clip-text text-transparent;
}

.auth-shell-best-ai { color: #173b32; }
.auth-background-best-ai { background: #f7f8f2; }
.dark .auth-shell-best-ai { color: #edf3ed; }
.dark .auth-background-best-ai { background: #141d19; }
.auth-brand-mark {
  display: inline-grid;
  width: 48px;
  height: 48px;
  place-items: center;
  border-radius: 10px;
  background: #173b32;
  color: #def58b;
  font-size: 28px;
  font-weight: 700;
}
.auth-title-best-ai { margin-bottom: 5px; background: none; color: #173b32; -webkit-text-fill-color: currentColor; }
.auth-subtitle-best-ai { color: #737c71; }
.auth-shell-best-ai :deep(.card-glass) {
  border: 1px solid #dfe4d7;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 16px 40px rgb(23 59 50 / 7%);
  backdrop-filter: none;
}
.dark .auth-shell-best-ai :deep(.card-glass) {
  border-color: #35463b;
  background: #1c2922;
  box-shadow: 0 16px 40px rgb(0 0 0 / 24%);
}
.auth-shell-best-ai :deep(.btn-primary) { background: #173b32; color: #fff; }
.auth-shell-best-ai :deep(.btn-primary:hover:not(:disabled)) { background: #2c5547; }
.auth-shell-best-ai :deep(.input:focus) { --tw-ring-color: #86a76b; border-color: #73985e; }
.auth-shell-best-ai :deep(.input) {
  border-color: #d6ded2;
  border-radius: 6px;
  background: #fff;
  color: #173b32;
}
.dark .auth-shell-best-ai :deep(.input) {
  border-color: #415449;
  background: #141d19;
  color: #edf3ed;
}
.dark .auth-shell-best-ai :deep(.input:focus) { border-color: #83ad70; --tw-ring-color: #83ad70; }
.dark .auth-shell-best-ai :deep(.input-label) { color: #c4d2c5; }
.dark .auth-shell-best-ai :deep(.btn-primary) { background: #83ad70; color: #152018; }
.dark .auth-shell-best-ai :deep(.btn-primary:hover:not(:disabled)) { background: #a8ca96; }
.auth-shell-best-ai :deep(input::placeholder) { color: #929c91; }
.auth-shell-best-ai :deep(.input-label) { color: #44564a; }
.auth-shell-best-ai :deep(.text-primary-600) { color: #39765b; }
.auth-shell-best-ai :deep(.text-primary-600:hover) { color: #24583f; }
.auth-shell-best-ai :deep(.text-gray-900) { color: #173b32; }
.auth-shell-best-ai :deep(.text-gray-500) { color: #737c71; }
.dark .auth-title-best-ai { color: #edf3ed; }
.dark .auth-subtitle-best-ai { color: #a2b0a5; }
.dark .auth-shell-best-ai :deep(.text-gray-900) { color: #edf3ed; }
.dark .auth-shell-best-ai :deep(.text-gray-500) { color: #a2b0a5; }
</style>
