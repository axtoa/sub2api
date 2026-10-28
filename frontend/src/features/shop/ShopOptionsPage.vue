<template>
  <AppLayout>
    <div class="best-shop shop-options">
      <div class="shop-main">
        <div class="shop-heading">
          <h1>{{ t('shop.optionsTitle') }}</h1>
          <p>{{ t('shop.optionsIntro') }}</p>
        </div>
        <div v-if="loading" class="shop-empty">{{ t('common.loading') }}</div>
        <div v-else-if="error" class="shop-empty"><span>{{ t('shop.loadError') }}</span><button type="button" class="shop-text-link" @click="load">{{ t('shop.retry') }}</button></div>
        <div v-else-if="officialEnabled || externalURL" class="shop-options-list">
          <router-link v-if="officialEnabled" to="/shop" class="shop-option">
            <span><strong>{{ officialName }}</strong><small>{{ t('shop.officialOptionHint') }}</small></span>
            <Icon name="arrowRight" size="sm" />
          </router-link>
          <a v-if="externalURL" :href="externalURL" class="shop-option">
            <span><strong>{{ externalName }}</strong><small>{{ t('shop.externalOptionHint') }}</small></span>
            <Icon name="arrowRight" size="sm" />
          </a>
        </div>
        <div v-else class="shop-empty">{{ t('shop.noChannels') }}</div>
        <p v-if="!loading && !error && hasBonus && (officialEnabled || externalURL)" class="shop-options-note">{{ t('shop.homepageManualBonus') }}</p>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useShopAvailability } from './availability'
import { externalShopLink } from '@/utils/externalLinks'
import './shop.css'

const { t } = useI18n()
const app = useAppStore()
const { enabled: officialEnabled } = useShopAvailability()
const loading = ref(true)
const error = ref(false)
const externalURL = computed(() => externalShopLink(app.cachedPublicSettings))
const hasBonus = computed(() => app.cachedPublicSettings?.shop_homepage_enabled === true &&
  !!app.cachedPublicSettings?.shop_homepage_content?.cards.some(card => card.bonus_cents > 0))
const officialName = computed(() => app.cachedPublicSettings?.official_shop_name?.trim() || t('shop.entry'))
const externalName = computed(() => app.cachedPublicSettings?.external_shop_name?.trim() || t('shop.externalEntry'))

async function load() {
  loading.value = true
  error.value = false
  try { error.value = !(await app.fetchPublicSettings(true)) }
  catch { error.value = true }
  finally { loading.value = false }
}

onMounted(load)
</script>
