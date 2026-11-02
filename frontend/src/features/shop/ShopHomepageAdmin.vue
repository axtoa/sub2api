<template>
  <section class="shop-homepage-admin">
    <div class="shop-section-heading">
      <h2>{{ t('shop.homepageTab') }}</h2>
      <button type="button" class="shop-icon-button" :disabled="loading || saving" :title="t('shop.refresh')" :aria-label="t('shop.refresh')" @click="load"><Icon name="refresh" size="sm" /></button>
    </div>
    <div v-if="loading" class="shop-empty">{{ t('common.loading') }}</div>
    <p v-else-if="!loaded" class="shop-error" role="alert">{{ error }}</p>
    <form v-else class="shop-form shop-homepage-editor" @submit.prevent="save">
      <fieldset :disabled="saving">
        <label>{{ t('shop.homepageTitleField') }}<input v-model="title" required maxlength="60" /></label>
        <label>{{ t('shop.homepageLeadField') }}<textarea v-model="lead" required maxlength="180" rows="2"></textarea></label>
        <p class="shop-caption">{{ t('shop.homepageCatalogHint') }}</p>
        <router-link class="shop-secondary" to="/admin/shop/cdk">{{ t('shop.cdkManagement') }}<Icon name="arrowRight" size="sm" /></router-link>
      </fieldset>
      <p v-if="error" class="shop-error" role="alert">{{ error }}</p>
      <p v-if="saved" class="shop-success" role="status">{{ t('shop.homepageSaved') }}</p>
      <button type="submit" class="shop-primary" :disabled="saving">{{ saving ? t('common.submitting') : t('shop.homepageSave') }}</button>
    </form>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { ShopHomepageContent } from '@/types'
import { shopAPI } from './api'
import { useShopAvailability } from './availability'

const { t } = useI18n()
const { applyHomepageContent } = useShopAvailability()
const loading = ref(true)
const loaded = ref(false)
const saving = ref(false)
const error = ref('')
const saved = ref(false)
const title = ref('')
const lead = ref('')

async function load() {
  loading.value = true
  loaded.value = false
  error.value = ''
  saved.value = false
  try {
    const content = await shopAPI.homepageContent()
    title.value = content?.title || t('shop.storefrontTitle')
    lead.value = content?.lead || t('shop.storefrontLead')
    loaded.value = true
  } catch (e) { error.value = (e as Error).message || t('shop.loadError') }
  finally { loading.value = false }
}

async function save() {
  if (saving.value || loading.value || !loaded.value) return
  if (!title.value.trim() || !lead.value.trim() || Array.from(title.value.trim()).length > 60 || Array.from(lead.value.trim()).length > 180) {
    error.value = t('shop.homepageInvalid')
    return
  }
  const content: ShopHomepageContent = { title: title.value.trim(), lead: lead.value.trim(), cards: [] }
  saving.value = true
  error.value = ''
  saved.value = false
  try {
    const result = await shopAPI.saveHomepageContent(content)
    applyHomepageContent(result, true)
    saved.value = true
  } catch (e) { error.value = (e as Error).message || t('shop.availabilitySaveError') }
  finally { saving.value = false }
}

onMounted(load)
</script>
