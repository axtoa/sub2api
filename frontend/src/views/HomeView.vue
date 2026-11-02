<template>
  <div v-if="hasHomeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-[44px] shrink-0 items-center justify-center rounded-md text-xs font-semibold text-gray-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500 dark:text-gray-200"
          >
            <span class="inline-flex min-h-[30px] items-center rounded-md bg-gray-100 px-3 transition-colors hover:bg-gray-200 dark:bg-dark-800 dark:hover:bg-dark-700">{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}</span>
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <div v-else class="best-ai-home">
    <header class="home-header">
      <a class="home-brand" href="/home" aria-label="Best AI 首页">
        <img class="home-brand-mark" :src="siteLogo || '/logo.svg'" alt="Best AI" />
        <span>Best AI</span>
      </a>
      <nav class="home-actions" :aria-label="t('home.navigation')">
        <LocaleSwitcher />
        <a v-if="docUrl" class="icon-action" :href="docUrl" target="_blank" rel="noopener noreferrer" :title="t('home.viewDocs')">
          <Icon name="book" size="md" />
        </a>
        <router-link
          v-if="showModelPlazaEntry"
          class="model-link"
          to="/model-plaza"
        >
          {{ t('nav.modelPlaza') }}
        </router-link>
        <router-link class="account-link" :to="isAuthenticated ? dashboardPath : '/login'">
          <span>{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}</span>
        </router-link>
      </nav>
    </header>

    <main class="home-main">
      <nav v-if="showShopPromotion" class="home-mobile-nav" :aria-label="t('home.serviceNavigation')">
        <a href="#home-api">{{ t('home.apiService') }}</a>
        <a href="#home-shop">{{ t('shop.storefrontTitle') }} <span aria-hidden="true">↘</span></a>
      </nav>
      <div class="home-hero" :class="{ 'home-hero-single': !showShopPromotion }">
        <section id="home-api" class="hero-copy" aria-labelledby="home-title">
          <div class="hero-intro">
            <p class="hero-eyebrow"><Icon name="server" size="sm" aria-hidden="true" />BEST AI · API SERVICES</p>
            <h1 id="home-title">{{ t('home.platformTitle') }}</h1>
            <p class="hero-quality">{{ t('home.heroEyebrow') }}</p>
            <p class="hero-poem">
              <span>{{ t('home.heroTitleLine1') }}</span>
              <span>{{ t('home.heroTitleLine2') }}</span>
            </p>
            <p class="hero-description">{{ t('home.heroLead') }}</p>
            <div class="hero-actions">
              <router-link class="hero-cta" :to="isAuthenticated ? dashboardPath : '/login'">
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.startConnecting') }}
                <Icon name="arrowRight" size="md" :stroke-width="2" />
              </router-link>
              <a v-if="docUrl" class="hero-docs" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.viewDocs') }} <Icon name="externalLink" size="sm" /></a>
            </div>
          </div>
          <div class="home-capabilities" :aria-label="t('home.providers.title')">
            <article class="home-capability">
              <h2>{{ t('home.modelEntryTitle') }}</h2>
              <p>{{ t('home.modelEntryLead') }}</p>
              <div class="home-model-flow">
                <span>{{ t('home.yourTools') }}</span><Icon name="arrowRight" size="xs" aria-hidden="true" />
                <span>Best AI</span><Icon name="arrowRight" size="xs" aria-hidden="true" />
                <span>ChatGPT / Claude</span>
              </div>
            </article>
            <article class="home-capability home-billing">
              <h2>{{ t('home.billingTitle') }}</h2>
              <p>{{ t('home.openaiModels') }}</p>
              <p>{{ t('home.anthropicModels') }}</p>
              <p>{{ t('home.simpleBilling') }}</p>
              <p>{{ t('home.invoiceAvailable') }}</p>
            </article>
          </div>
          <ul class="home-use-cases" :aria-label="t('home.useCasesLabel')">
            <li v-for="useCase in useCases" :key="useCase">{{ t(`home.useCases.${useCase}`) }}</li>
          </ul>
        </section>
        <HomeShopSection v-if="showShopPromotion" :official-enabled="officialShopEnabled" :external-url="externalShopURL" />
      </div>
    </main>

    <footer class="home-footer">
      <span>&copy; {{ currentYear }} Best AI. {{ t('home.footer.allRightsReserved') }}</span>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import HomeShopSection from '@/features/shop/HomeShopSection.vue'
import { useShopAvailability } from '@/features/shop/availability'
import { externalShopLink } from '@/utils/externalLinks'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const { enabled: officialShopEnabled, homepageEnabled } = useShopAvailability()
const externalShopURL = computed(() => externalShopLink(appStore.cachedPublicSettings))
const showShopPromotion = computed(() => homepageEnabled.value && (officialShopEnabled.value || !!externalShopURL.value))

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Best AI')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(() => appStore.cachedPublicSettings?.model_plaza_require_auth === true)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const currentYear = computed(() => new Date().getFullYear())
const useCases = ['coding', 'writing', 'research', 'office', 'comics', 'shortDrama', 'image', 'video'] as const

onMounted(() => {
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
})
</script>

<style scoped>
.best-ai-home {
  --home-ink: #173b32;
  --home-muted: #697764;
  --home-green: #234b3a;
  --home-line: #d7e0cd;
  --home-panel: #e8efde;
  --home-card: #fafcf5;
  --home-button-ink: #f7faef;
  display: flex;
  min-height: 100svh;
  flex-direction: column;
  background: #f7f8f2;
  color: var(--home-ink);
}
.home-header, .home-footer, .home-main { width: min(1320px, calc(100% - 64px)); margin: 0 auto; }
.home-header, .home-footer { display: flex; align-items: center; justify-content: space-between; }
.home-header { min-height: 82px; gap: 20px; border-bottom: 1px solid var(--home-line); }
.home-brand, .home-actions, .account-link { display: inline-flex; align-items: center; }
.home-brand { flex-shrink: 0; gap: 9px; color: var(--home-ink); font-size: 23px; font-weight: 700; text-decoration: none; }
.home-brand-mark { display: block; width: 32px; height: 32px; flex-shrink: 0; border-radius: 8px; object-fit: contain; }
.home-actions { flex-wrap: wrap; justify-content: flex-end; gap: 18px; }
.icon-action, .model-link { color: var(--home-muted); text-decoration: none; }
.account-link { min-height: 44px; justify-content: center; border-radius: 6px; color: var(--home-ink); font-size: 12px; font-weight: 600; text-decoration: none; }
.account-link > span { display: inline-flex; min-height: 30px; align-items: center; padding: 0 12px; border-radius: 6px; background: var(--home-panel); transition: background 160ms ease; }
.account-link:hover > span { background: var(--home-line); }
.account-link:focus-visible { outline: 2px solid var(--home-green); outline-offset: 3px; }
.home-main { display: flex; flex: 1; flex-direction: column; justify-content: center; padding: 28px 0; }
.home-hero { display: grid; min-height: 604px; grid-template-columns: minmax(0, 55fr) minmax(0, 45fr); align-items: stretch; gap: 20px; }
.home-hero-single { grid-template-columns: minmax(0, 1fr); }
.hero-copy { display: flex; min-width: 0; flex-direction: column; padding: clamp(24px, 2.7vw, 36px); border: 1px solid var(--home-line); border-radius: 20px; background: var(--home-panel); scroll-margin-top: 20px; }
.hero-intro { min-width: 0; }
.hero-eyebrow { display: flex; align-items: center; gap: 7px; margin: 0; color: var(--home-muted); font-size: 11px; font-weight: 600; letter-spacing: 1.2px; }
.hero-copy h1 { margin: 8px 0 0; font-size: clamp(38px, 4vw, 50px); font-weight: 750; line-height: 1.25; letter-spacing: -1.2px; overflow-wrap: anywhere; }
.hero-quality { margin: 12px 0 0; color: var(--home-muted); font-size: 13px; font-weight: 400; line-height: 1.8; }
.hero-poem { margin: 26px 0 0; font-size: clamp(20px, 2vw, 24px); font-weight: 550; line-height: 1.65; }
.hero-poem > span { display: block; text-wrap: balance; }
.hero-description { max-width: 440px; margin: 10px 0 0; color: var(--home-muted); font-size: 13px; line-height: 1.9; }
.hero-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 20px; margin-top: 22px; }
.hero-cta { display: inline-flex; min-height: 45px; align-items: center; justify-content: center; gap: 14px; padding: 0 20px; border-radius: 9px; background: var(--home-green); color: var(--home-button-ink); font-size: 13px; font-weight: 600; text-decoration: none; transition: opacity 160ms ease, transform 160ms ease; }
.hero-cta:hover { transform: translateY(-1px); opacity: .9; }
.hero-docs { display: inline-flex; min-height: 44px; align-items: center; gap: 6px; color: var(--home-green); font-size: 12px; font-weight: 600; text-decoration: none; }
.home-capabilities { display: grid; grid-template-columns: minmax(0, 1.55fr) minmax(0, 1fr); gap: 10px; margin-top: 28px; }
.home-capability { min-width: 0; padding: 16px; border: 1px solid var(--home-line); border-radius: 13px; background: var(--home-card); overflow-wrap: anywhere; }
.home-capability h2 { margin: 0; font-size: 13px; font-weight: 600; line-height: 1.6; }
.home-capability p { margin: 7px 0 0; color: var(--home-muted); font-size: 12px; line-height: 1.6; }
.home-model-flow { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin-top: 12px; font-size: 11px; }
.home-model-flow span { padding: 3px 7px; border: 1px solid var(--home-line); border-radius: 5px; }
.home-model-flow svg { flex-shrink: 0; }
.home-use-cases { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px 12px; margin: auto 0 0; padding: 20px 0 0; color: var(--home-muted); font-size: 12px; line-height: 1.6; list-style: none; }
.home-use-cases li { min-width: 0; overflow-wrap: anywhere; }
.home-hero-single .hero-copy { width: 100%; max-width: 760px; min-height: 604px; margin: 0 auto; }
.home-footer { min-height: 58px; justify-content: center; padding: 16px 0; border-top: 1px solid var(--home-line); color: var(--home-muted); font-size: 11px; text-align: center; }
.home-mobile-nav { display: none; }
.best-ai-home :is(a, button):focus-visible { outline: 2px solid var(--home-green); outline-offset: 4px; }
@media (max-width: 820px) {
  .home-header, .home-main, .home-footer { width: min(100% - 36px, 620px); }
  .home-header { min-height: 70px; }
  .home-brand { font-size: 20px; }
  .home-actions { gap: 10px; }
  .model-link { display: none; }
  .home-main { padding: 18px 0 24px; }
  .home-hero { min-height: auto; grid-template-columns: minmax(0, 1fr); gap: 18px; }
  .hero-copy { min-height: 604px; }
  .hero-copy h1 { font-size: 38px; }
  .hero-poem { font-size: 21px; }
  .home-mobile-nav { display: flex; gap: 10px; margin-bottom: 16px; }
  .home-mobile-nav a { display: inline-flex; min-height: 40px; flex: 1; align-items: center; justify-content: center; gap: 8px; border: 1px solid var(--home-line); border-radius: 10px; color: var(--home-green); font-size: 12px; text-decoration: none; }
  .home-mobile-nav a:first-child { border-color: var(--home-green); background: var(--home-green); color: var(--home-button-ink); }
  .home-mobile-nav a:last-child { border-color: #dfbea1; background: #f7e2ce; color: #70422b; }
}
@media (max-width: 420px) {
  .home-capabilities { grid-template-columns: minmax(0, 1fr); }
  .home-use-cases { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 380px) {
  .home-header { flex-wrap: wrap; padding: 14px 0; }
  .hero-copy h1 { font-size: 34px; }
}
@media (prefers-reduced-motion: reduce) { .hero-cta { transition: none; } .hero-cta:hover { transform: none; } }
</style>

<style>
/* Unscoped selectors preserve the dark ancestor in production CSS. */
.dark .best-ai-home {
  --home-ink: #edf4e5;
  --home-muted: #b2c0a6;
  --home-green: #d9edb5;
  --home-line: #3e5038;
  --home-panel: #203b2c;
  --home-card: #2b4835;
  --home-button-ink: #183a2b;
  background: #141d19;
}
.dark .best-ai-home .home-mobile-nav a:last-child { border-color: #76533a; background: #493426; color: #f2ddc8; }
</style>
