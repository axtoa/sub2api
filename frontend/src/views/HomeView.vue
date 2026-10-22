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
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
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
        <span class="home-brand-mark" aria-hidden="true">B</span>
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
        <button
          class="icon-action theme-action"
          type="button"
          :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          @click="toggleTheme"
        >
          <Icon v-if="isDark" name="sun" size="md" />
          <Icon v-else name="moon" size="md" />
        </button>
        <router-link class="account-link" :to="isAuthenticated ? dashboardPath : '/login'">
          {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
        </router-link>
      </nav>
    </header>

    <main class="home-main">
      <section class="home-hero" aria-labelledby="home-title">
        <div class="hero-copy">
          <p class="hero-eyebrow">{{ t('home.heroEyebrow') }}</p>
          <h1 id="home-title">
            <span>{{ t('home.heroTitleLine1') }}</span>
            <span class="hero-highlight">{{ t('home.heroTitleLine2') }}</span>
          </h1>
          <p class="hero-description">{{ t('home.heroLead') }}</p>
          <router-link
            class="hero-cta"
            :to="isAuthenticated ? dashboardPath : '/login'"
          >
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
            <Icon name="arrowRight" size="md" :stroke-width="2" />
          </router-link>
        </div>

        <div class="hero-visual" aria-hidden="true">
          <div class="art-grid"></div>
          <div class="art-orbit"></div>
          <div class="art-label">YOUR NEXT IDEA, CONNECTED</div>
          <article class="visual-card chat-card">
            <div class="card-top">
              <span class="chat-mark">✳</span>
              <span><b>ChatGPT</b><small>A space for your next idea.</small></span>
              <i>PLUS</i>
            </div>
            <div class="chat-bubble">把这个灵感，变成一个好故事。</div>
            <div class="chat-answer"><span>✳</span><i></i><i></i><i></i></div>
            <div class="chat-foot">想法开始生长。 <span>↗</span></div>
          </article>
          <div class="visual-card code-card">
            <div class="code-top"><span>hello_ai.py</span><span>•••</span></div>
            <pre><span># YOUR AI WORKFLOW</span>
client = <b>OpenAI</b>(
  base_url=<b>"Best AI"</b>,
  api_key=<b>"sk-••••••"</b>
)
<i>&gt; Hello, possibility.</i></pre>
          </div>
          <span class="float-label">ChatGPT <i></i> Anthropic</span>
          <span class="art-bottom">CONNECT. BUILD. CREATE.</span>
        </div>
      </section>

      <section class="home-proof" :aria-label="t('home.providers.title')">
        <span>ChatGPT</span>
        <i aria-hidden="true"></i>
        <span>Anthropic</span>
        <i aria-hidden="true"></i>
        <span>{{ t('home.simpleBilling') }}</span>
      </section>

      <section class="home-secondary">
        <!-- Temporarily hidden until the subscription purchase flow is ready. -->
        <!-- <p>{{ t('home.subscriptionTeaser') }}</p> -->
        <div v-if="docUrl" class="home-next-step">
          <span>{{ t('home.nextStep') }}</span>
          <a :href="docUrl" target="_blank" rel="noopener noreferrer">
            {{ t('home.viewDocs') }} <span aria-hidden="true">↗</span>
          </a>
        </div>
      </section>
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
import { useTheme } from '@/composables/useTheme'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const { isDark, toggleTheme } = useTheme()

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

onMounted(() => {
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
})
</script>

<style scoped>
.best-ai-home {
  --home-ink: #173b32;
  --home-muted: #737c71;
  --home-green: #173b32;
  --home-line: #dfe4d7;
  --home-lime: #def58b;
  display: flex;
  min-height: 100vh;
  flex-direction: column;
  overflow: hidden;
  background: #f7f8f2;
  color: var(--home-ink);
}

.home-header,
.home-footer {
  display: flex;
  width: min(1180px, calc(100% - 64px));
  margin: 0 auto;
  align-items: center;
  justify-content: space-between;
}

.home-header {
  min-height: 82px;
  border-bottom: 1px solid var(--home-line);
}

.home-brand,
.home-actions,
.account-link,
.model-link {
  display: inline-flex;
  align-items: center;
}

.home-brand {
  gap: 9px;
  color: var(--home-ink);
  font-size: 23px;
  font-weight: 700;
  text-decoration: none;
}

.home-brand-mark {
  display: grid;
  width: 32px;
  height: 32px;
  place-items: center;
  border-radius: 8px;
  background: #173b32;
  color: #def58b;
  font-size: 20px;
  font-weight: 700;
}

.home-actions {
  gap: 18px;
}

.icon-action,
.model-link {
  color: var(--home-muted);
  text-decoration: none;
}

.theme-action {
  display: inline-flex;
  width: 36px;
  height: 36px;
  align-items: center;
  justify-content: center;
  border: 0;
  background: transparent;
  cursor: pointer;
}

.account-link {
  min-height: 40px;
  padding: 0 16px;
  border: 0;
  border-radius: 6px;
  background: var(--home-ink);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  text-decoration: none;
}

.home-main {
  width: min(1180px, calc(100% - 64px));
  margin: 0 auto;
  flex: 1;
}

.home-hero {
  display: grid;
  min-height: 500px;
  grid-template-columns: 1fr 0.92fr;
  align-items: center;
  gap: 42px;
  padding: 58px 0 54px;
}

.hero-copy {
  max-width: 560px;
}

.hero-eyebrow {
  margin: 0 0 20px;
  color: var(--home-green);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 1.4px;
  text-transform: uppercase;
}

.hero-copy h1 {
  max-width: 580px;
  margin: 22px 0 20px;
  font-size: 58px;
  font-weight: 650;
  line-height: 1.3;
  overflow-wrap: anywhere;
}

.hero-copy h1 > span { display: block; }
.hero-highlight { position: relative; z-index: 0; width: fit-content; }
.hero-highlight::after {
  position: absolute;
  z-index: -1;
  right: -5px;
  bottom: 5px;
  left: -3px;
  height: 12px;
  transform: rotate(-1deg);
  background: var(--home-lime);
  content: '';
}

.hero-description {
  max-width: 470px;
  margin: 22px 0 30px;
  color: var(--home-muted);
  font-size: 14px;
  line-height: 1.95;
}

.hero-cta {
  display: inline-flex;
  min-height: 54px;
  align-items: center;
  gap: 14px;
  padding: 0 22px;
  border-radius: 8px;
  background: var(--home-ink);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  transition: background 160ms ease, transform 160ms ease;
}

.hero-cta:hover {
  transform: translateY(-1px);
  background: #2c5547;
}

.hero-visual {
  position: relative;
  height: 404px;
  overflow: hidden;
  border-radius: 20px;
  background: #e9eddf;
}

.art-grid {
  position: absolute;
  inset: 0;
  background-image: linear-gradient(#bcc9b42b 1px, transparent 1px), linear-gradient(90deg, #bcc9b42b 1px, transparent 1px);
  background-size: 36px 36px;
  mask-image: linear-gradient(transparent, #000);
}

.art-orbit {
  position: absolute;
  top: 48px;
  left: 50%;
  width: 310px;
  height: 310px;
  transform: translateX(-50%);
  border: 1px solid #cdd8be;
  border-radius: 50%;
}

.art-orbit::after {
  position: absolute;
  inset: 28px;
  border: 1px solid #d3ddc7;
  border-radius: 50%;
  content: '';
}

.art-label,
.art-bottom {
  position: absolute;
  color: #6c7d61;
  font-size: 9px;
  letter-spacing: 1.7px;
}

.art-label { top: 22px; left: 25px; }
.art-bottom { right: 22px; bottom: 16px; }

.visual-card {
  position: absolute;
  border: 1px solid #e2e7da;
  border-radius: 14px;
  background: #fff;
  box-shadow: 0 16px 25px rgb(33 57 26 / 8%);
}

.chat-card {
  top: 72px;
  left: 7%;
  z-index: 1;
  width: 256px;
  padding: 19px;
  transform: rotate(-4deg);
}

.card-top { display: flex; align-items: center; gap: 10px; }
.chat-mark { display: grid; width: 31px; height: 31px; place-items: center; border-radius: 9px; background: #e9f2e6; color: #39765b; }
.card-top b { display: block; color: #273b30; font-size: 13px; }
.card-top small { display: block; margin-top: 3px; color: #899188; font-size: 9px; }
.card-top > i { margin-left: auto; padding: 4px 6px; border-radius: 4px; background: #e6f3dc; color: #688657; font-size: 8px; font-style: normal; }
.chat-bubble { margin: 17px 0 12px; padding: 10px 11px; border-radius: 8px 8px 2px 8px; background: #f3f5ee; color: #57665a; font-size: 10px; }
.chat-answer { display: flex; align-items: center; gap: 7px; color: #74a06c; font-size: 11px; }
.chat-answer i { width: 32px; height: 4px; border-radius: 3px; background: #dfe6db; }
.chat-answer i:last-child { width: 22px; }
.chat-foot { display: flex; justify-content: space-between; margin-top: 13px; padding-top: 10px; border-top: 1px solid #edf0e9; color: #9aa197; font-size: 9px; }

.code-card {
  right: 5%;
  bottom: 52px;
  z-index: 2;
  width: 265px;
  overflow: hidden;
  transform: rotate(3deg);
  background: #173b32;
  border-color: #254a3e;
  color: #f0f3e8;
}

.code-top { display: flex; justify-content: space-between; padding: 11px 14px; border-bottom: 1px solid rgb(255 255 255 / 10%); color: #b5c8b7; font: 9px ui-monospace, SFMono-Regular, Menlo, monospace; }
.code-card pre { margin: 0; padding: 12px 14px 15px; font: 9px/1.8 ui-monospace, SFMono-Regular, Menlo, monospace; white-space: pre-wrap; }
.code-card pre span { color: #91a796; }
.code-card pre b { color: #def58b; font-weight: 500; }
.code-card pre i { color: #f0d1aa; font-style: normal; }
.float-label { position: absolute; top: 216px; right: 8%; display: flex; align-items: center; gap: 7px; color: #62735e; font-size: 10px; }
.float-label i { width: 4px; height: 4px; border-radius: 50%; background: #89a76a; }

.home-proof {
  display: flex;
  min-height: 74px;
  align-items: center;
  justify-content: center;
  gap: 22px;
  border-top: 1px solid var(--home-line);
  border-bottom: 1px solid var(--home-line);
  color: #45564b;
  font-size: 13px;
  font-weight: 600;
}

.home-proof i {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #86a191;
}

.home-secondary {
  display: flex;
  min-height: 74px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--home-muted);
  font-size: 12px;
  text-align: center;
}

.home-secondary p { margin: 0; }
.home-next-step { display: flex; gap: 12px; align-items: center; }
.home-next-step {
  display: flex;
  color: var(--home-muted);
  font-size: 13px;
}

.home-next-step a { color: var(--home-green); font-weight: 600; text-decoration: none; }
.home-footer { min-height: 58px; border-top: 1px solid var(--home-line); color: #7c877e; font-size: 12px; }
.home-footer { justify-content: center; text-align: center; }

@media (max-width: 760px) {
  .home-header,
  .home-main,
  .home-footer { width: min(100% - 36px, 560px); }
  .home-header { min-height: 70px; }
  .home-actions { gap: 10px; }
  .model-link { display: none; }
  .account-link { min-height: 36px; padding: 0 11px; font-size: 12px; }
  .home-hero { min-height: auto; grid-template-columns: 1fr; gap: 34px; padding: 52px 0 36px; }
  .hero-copy h1 { max-width: 520px; font-size: 44px; }
  .hero-description { margin: 18px 0 24px; font-size: 14px; }
  .hero-visual { height: 340px; }
  .chat-card { top: 64px; left: 4%; width: min(256px, 76%); }
  .code-card { right: 3%; bottom: 37px; width: min(265px, 78%); }
  .float-label { top: 202px; right: 6%; }
  .home-proof { min-height: 64px; gap: 12px; font-size: 12px; }
  .home-secondary { min-height: 70px; flex-direction: column; gap: 7px; font-size: 11px; }
  .home-next-step { font-size: 11px; }
}

@media (min-width: 761px) and (max-width: 1024px) {
  .hero-copy h1 { font-size: 54px; }
}

@media (prefers-reduced-motion: reduce) {
  .hero-cta { transition: none; }
}
</style>

<style>
/* Keep these overrides unscoped: Vue drops :global(.dark) descendant selectors in production CSS. */
.dark .best-ai-home {
  --home-ink: #edf3ed;
  --home-muted: #a2b0a5;
  --home-green: #87c5a7;
  --home-line: #2e3b34;
  background: #141d19;
}

.dark .best-ai-home .account-link {
  background: #e7efdf;
  color: #173b32;
}

.dark .best-ai-home .hero-cta {
  background: #e7efdf;
  color: #173b32;
}

.dark .best-ai-home .hero-cta:hover {
  background: #d9e5cf;
}

.dark .best-ai-home .home-proof { color: #c0cec2; }

.dark .best-ai-home .hero-highlight::after {
  bottom: 3px;
  height: 6px;
  background: #9aa982;
  opacity: 0.42;
}
</style>
