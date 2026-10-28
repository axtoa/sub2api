<template>
  <section class="home-shop best-shop" aria-labelledby="home-shop-title">
    <div class="shop-section-heading">
      <div><h2 id="home-shop-title">{{ title }}</h2><p>{{ lead }}</p></div>
      <router-link to="/shop/options" class="shop-text-link home-shop-cta">{{ t(auth.isAuthenticated ? 'shop.homepageView' : 'shop.homepageExplore') }}<Icon name="arrowRight" size="sm" /></router-link>
    </div>
    <div v-if="cards.length" class="home-shop-grid">
      <article v-for="(card, index) in cards" :key="index" class="home-shop-card">
        <div class="home-shop-card-top"><h3>{{ card.title }}</h3><span v-if="card.price_hint" class="home-shop-price">{{ card.price_hint }}</span></div>
        <p>{{ card.summary }}</p>
        <strong v-if="card.bonus_cents > 0" class="home-shop-bonus">{{ t('shop.homepageBonusLabel', { amount: formatBonus(card.bonus_cents) }) }}</strong>
      </article>
    </div>
    <p class="home-shop-note">{{ t('shop.homepagePriceNote') }}<span v-if="hasBonus"> {{ t('shop.homepageManualBonus') }}</span></p>
  </section>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import type { ShopHomepageCard } from '@/types'
import './shop.css'

const { t } = useI18n()
const app = useAppStore()
const auth = useAuthStore()
const content = computed(() => app.cachedPublicSettings?.shop_homepage_content)
const cards = computed<ShopHomepageCard[]>(() => content.value?.cards ?? [{
  title: t('shop.homepageFallbackCard'), summary: t('shop.homepageFallbackSummary'), price_hint: '', bonus_cents: 0,
}])
const hasBonus = computed(() => cards.value.some(card => card.bonus_cents > 0))
const title = computed(() => content.value?.title || t('shop.homepageTitle'))
const lead = computed(() => content.value?.lead || t(hasBonus.value ? 'shop.homepageBonusLead' : 'shop.homepageDefaultLead'))
function formatBonus(cents: number): string { return `$${(cents / 100).toFixed(2)}` }
</script>
