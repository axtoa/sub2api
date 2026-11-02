<template>
  <section id="home-shop" class="home-shop" aria-labelledby="home-shop-title">
    <div class="home-shop-heading">
      <p class="home-shop-eyebrow"><Icon name="gift" size="sm" aria-hidden="true" />BEST AI · SHOP</p>
      <h2 id="home-shop-title">{{ title }}</h2>
      <p class="home-shop-lead">{{ lead }}</p>
    </div>

    <div v-if="officialEnabled" class="home-shop-products" :aria-busy="loading">
      <div v-if="loading" class="home-shop-state" role="status">{{ t('common.loading') }}</div>
      <div v-else-if="failed" class="home-shop-state" role="status">
        <p>{{ t('shop.loadError') }}</p>
        <button type="button" class="home-shop-text-link" @click="load">{{ t('shop.retry') }} <Icon name="refresh" size="sm" /></button>
      </div>
      <div v-else-if="!storeEnabled || !products.length" class="home-shop-state" role="status">
        <Icon name="sparkles" size="lg" />
        <p>{{ t(storeEnabled ? 'shop.empty' : 'shop.storefrontPaused') }}</p>
        <small>{{ t('shop.storefrontComeBack') }}</small>
      </div>
      <a
        v-for="product in featuredProducts"
        :key="product.id"
        :href="storefrontLink(product.id)"
        target="_blank"
        rel="noopener noreferrer"
        class="home-shop-product"
        :aria-label="`${product.name}${product.reward_cents > 0 ? ' · ' + t('shop.storefrontReward', { amount: formatStorefrontReward(product.reward_cents) }) : ''} · ${t('shop.viewDetails')} · ${t('shop.openInNewTab')}`"
        :aria-describedby="product.reward_cents > 0 ? 'home-shop-reward-terms' : undefined"
      >
        <span class="home-shop-product-icon" aria-hidden="true"><Icon name="sparkles" size="md" /></span>
        <span class="home-shop-product-copy">
          <strong>{{ product.name }}</strong>
          <span>{{ product.spec || product.description }}</span>
          <small>{{ t(product.stock > 0 ? 'shop.storefrontInStock' : 'shop.storefrontSoldOut') }}</small>
          <small v-if="product.reward_cents > 0" class="home-shop-reward"><Icon name="gift" size="xs" aria-hidden="true" />{{ t('shop.storefrontReward', { amount: formatStorefrontReward(product.reward_cents) }) }}</small>
        </span>
        <span class="home-shop-product-end">
          <strong>¥{{ (product.price_cents / 100).toFixed(2) }}</strong>
          <span>{{ t('shop.viewDetails') }} <Icon name="arrowRight" size="sm" /></span>
        </span>
      </a>
    </div>

    <div v-else class="home-shop-external-cards">
      <a :href="externalUrl" target="_blank" rel="noopener noreferrer" class="home-shop-product">
        <span class="home-shop-product-icon" aria-hidden="true"><Icon name="sparkles" size="md" /></span>
        <span class="home-shop-product-copy"><strong>{{ t('shop.storefrontExternal') }}</strong><span>{{ t('shop.storefrontExternalHint') }}</span></span>
        <Icon name="externalLink" size="sm" />
      </a>
      <p class="home-shop-price-note">{{ t('shop.storefrontExternalPrices') }}</p>
    </div>

    <p v-if="hasFeaturedReward" id="home-shop-reward-terms" class="home-shop-reward-terms">{{ t('shop.storefrontRewardTerms') }}</p>

    <ol v-if="officialEnabled" class="home-shop-flow" :aria-label="t('shop.storefrontFlowLabel')">
      <li>{{ t('shop.storefrontFlowSelect') }}</li>
      <li>{{ t('shop.storefrontFlowPay') }}</li>
      <li>{{ t('shop.storefrontFlowRedeem') }}</li>
    </ol>

    <div class="home-shop-bottom">
      <div class="home-shop-actions">
        <a :href="shopUrl" target="_blank" rel="noopener noreferrer" class="home-shop-cta" :aria-label="`${t('shop.storefrontBrowse')} · ${t('shop.openInNewTab')}`">
          {{ t('shop.storefrontBrowse') }} <Icon name="externalLink" size="sm" />
        </a>
        <a v-if="officialEnabled" :href="storefrontLink(undefined, true)" target="_blank" rel="noopener noreferrer" class="home-shop-text-link">{{ t('shop.storefrontLookup') }}</a>
        <a v-else :href="externalUrl" target="_blank" rel="noopener noreferrer" class="home-shop-text-link">{{ t('shop.storefrontExternal') }} <Icon name="externalLink" size="sm" /></a>
      </div>
      <p class="home-shop-note">{{ t(officialEnabled ? 'shop.storefrontGuestHint' : 'shop.storefrontExternalHint') }}</p>
      <a v-if="officialEnabled && externalUrl" :href="externalUrl" target="_blank" rel="noopener noreferrer" class="home-shop-alternative">{{ t('shop.storefrontExternal') }} ↗</a>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { fetchStorefrontCatalog, formatStorefrontReward, storefrontLink, type StorefrontProduct } from './storefront'

const props = defineProps<{ officialEnabled: boolean; externalUrl: string }>()
const { t } = useI18n()
const app = useAppStore()
const content = computed(() => app.cachedPublicSettings?.shop_homepage_content)
const title = computed(() => content.value?.title || t('shop.storefrontTitle'))
const lead = computed(() => content.value?.lead || t('shop.storefrontLead'))
const shopUrl = computed(() => props.officialEnabled ? storefrontLink() : props.externalUrl)
const products = ref<StorefrontProduct[]>([])
const storeEnabled = ref(false)
const loading = ref(false)
const failed = ref(false)
const featuredProducts = computed(() => !loading.value && !failed.value && storeEnabled.value ? products.value.slice(0, 2) : [])
const hasFeaturedReward = computed(() => props.officialEnabled && featuredProducts.value.some(product => product.reward_cents > 0))
let controller: AbortController | undefined

async function load() {
  controller?.abort()
  if (!props.officialEnabled) return
  const current = new AbortController()
  controller = current
  loading.value = true
  failed.value = false
  try {
    const catalog = await fetchStorefrontCatalog(current.signal)
    if (current.signal.aborted) return
    products.value = catalog.products
    storeEnabled.value = catalog.enabled
  } catch {
    if (!current.signal.aborted) failed.value = true
  } finally {
    if (!current.signal.aborted) loading.value = false
  }
}

watch(() => props.officialEnabled, () => { void load() }, { immediate: true })
onUnmounted(() => controller?.abort())
</script>

<style scoped>
.home-shop {
  --store-bg: #f7e2ce;
  --store-ink: #70422b;
  --store-muted: #875e47;
  --store-accent: #aa5e37;
  --store-line: #dfbea1;
  --store-card: #fff8ef;
  --store-button-ink: #fff8ef;
  --store-reward-bg: #e5eed9;
  --store-reward-ink: #315238;
  display: flex;
  min-width: 0;
  flex-direction: column;
  padding: clamp(24px, 2.7vw, 36px);
  border: 1px solid var(--store-line);
  border-radius: 20px;
  background: var(--store-bg);
  color: var(--store-ink);
  scroll-margin-top: 20px;
}
.home-shop-eyebrow { display: flex; align-items: center; gap: 7px; margin: 0 0 10px; color: var(--store-muted); font-size: 11px; font-weight: 600; letter-spacing: 1.2px; }
.home-shop h2 { margin: 0; font-size: clamp(28px, 2.8vw, 36px); font-weight: 650; line-height: 1.4; overflow-wrap: anywhere; }
.home-shop-lead { margin: 12px 0 24px; color: var(--store-muted); font-size: 14px; line-height: 1.8; white-space: pre-line; overflow-wrap: anywhere; }
.home-shop-products, .home-shop-external-cards { display: grid; gap: 12px; margin-bottom: 20px; }
.home-shop-product { display: flex; min-width: 0; align-items: center; gap: 12px; padding: 20px 16px; border: 1px solid var(--store-line); border-radius: 14px; background: var(--store-card); color: inherit; text-decoration: none; transition: transform 180ms ease, box-shadow 180ms ease, border-color 180ms ease; }
.home-shop-product:hover { transform: translateY(-2px); border-color: var(--store-accent); box-shadow: 0 6px 16px rgb(112 66 43 / 6%); }
.home-shop-product-icon { display: grid; width: 30px; height: 30px; flex: 0 0 30px; place-items: center; color: var(--store-accent); }
.home-shop-product-copy { display: grid; min-width: 0; flex: 1; gap: 4px; }
.home-shop-product-copy strong { font-size: 15px; line-height: 1.4; overflow-wrap: anywhere; }
.home-shop-product-copy > span { display: -webkit-box; overflow: hidden; -webkit-line-clamp: 2; -webkit-box-orient: vertical; color: var(--store-muted); font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; }
.home-shop-product-copy small { color: var(--store-muted); font-size: 11px; }
.home-shop-product-copy .home-shop-reward { display: inline-flex; justify-self: start; align-items: center; gap: 5px; max-width: 100%; margin-top: 3px; padding: 4px 7px; border-radius: 6px; background: var(--store-reward-bg); color: var(--store-reward-ink); font-size: 11px; line-height: 1.6; overflow-wrap: anywhere; }
.home-shop-reward svg { flex-shrink: 0; }
.home-shop-reward-terms { margin: -6px 0 18px; color: var(--store-muted); font-size: 11px; line-height: 1.7; }
.home-shop-product-end { display: grid; max-width: 40%; flex-shrink: 0; justify-items: end; gap: 8px; text-align: right; overflow-wrap: anywhere; }
.home-shop-product-end strong { font-size: 16px; font-weight: 600; }
.home-shop-product-end > span { display: inline-flex; align-items: center; gap: 4px; color: var(--store-accent); font-size: 11px; }
.home-shop-bottom { margin-top: auto; padding-top: 26px; }
.home-shop-flow { display: flex; flex-wrap: wrap; gap: 6px 15px; margin: 0; padding: 0; color: var(--store-muted); font-size: 11px; line-height: 1.7; list-style: none; }
.home-shop-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 20px; }
.home-shop-cta { display: inline-flex; min-height: 45px; align-items: center; justify-content: center; gap: 14px; padding: 0 20px; border: 0; border-radius: 9px; background: var(--store-accent); color: var(--store-button-ink); font-size: 13px; font-weight: 600; text-decoration: none; transition: opacity 180ms ease; }
.home-shop-cta:hover { opacity: .9; }
.home-shop-text-link, .home-shop-alternative { display: inline-flex; align-items: center; gap: 5px; color: var(--store-accent); font-size: 12px; text-decoration: none; }
.home-shop-text-link { min-height: 44px; font-weight: 600; }
button.home-shop-text-link { padding: 0; border: 0; background: none; cursor: pointer; }
.home-shop-note, .home-shop-price-note { margin: 14px 0 0; color: var(--store-muted); font-size: 11px; line-height: 1.7; }
.home-shop-alternative { margin-top: 10px; }
.home-shop-state { display: flex; min-height: 215px; flex-direction: column; align-items: center; justify-content: center; gap: 12px; padding: 24px; border: 1px dashed var(--store-line); border-radius: 14px; color: var(--store-muted); font-size: 13px; text-align: center; }
.home-shop-state p { margin: 0; }
.home-shop-state small { font-size: 12px; }
.home-shop :is(a, button):focus-visible { outline: 2px solid var(--store-accent); outline-offset: 4px; }
@media (max-width: 400px) {
  .home-shop-product { gap: 10px; padding: 14px 12px; }
  .home-shop-product-icon { display: none; }
}
@media (prefers-reduced-motion: reduce) { .home-shop-product, .home-shop-cta { transition: none; } .home-shop-product:hover { transform: none; } }
</style>

<style>
.dark .home-shop {
  --store-bg: #493426;
  --store-ink: #f2ddc8;
  --store-muted: #d0af90;
  --store-accent: #de9c66;
  --store-line: #76533a;
  --store-card: #55402e;
  --store-button-ink: #2e2017;
  --store-reward-bg: #30472f;
  --store-reward-ink: #d8edc2;
}
</style>
