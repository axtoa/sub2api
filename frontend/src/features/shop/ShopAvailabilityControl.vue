<template>
  <section class="shop-availability" :aria-label="t('shop.availability')" :aria-busy="pending">
    <div>
      <h2>{{ t('shop.availability') }}</h2>
      <p role="status">{{ pending ? t('common.loading') : confirmed === null ? t('shop.availabilityUnknown') : t(confirmed ? 'shop.availabilityOn' : 'shop.availabilityOff') }}</p>
      <p v-if="error" class="shop-error" role="alert">{{ error }} <button v-if="confirmed === null" type="button" @click="load">{{ t('shop.retry') }}</button></p>
    </div>
    <Toggle :model-value="confirmed === true" :disabled="pending || confirmed === null" :aria-label="t('shop.availability')" @update:model-value="save" />
  </section>
  <section class="shop-availability" :aria-label="t('shop.externalAvailability')" :aria-busy="externalPending">
    <div class="min-w-0 flex-1">
      <h2>{{ t('shop.externalAvailability') }}</h2>
      <p role="status">{{ externalPending ? t('common.loading') : externalConfirmed === null ? t('shop.availabilityUnknown') : t(externalConfirmed ? 'shop.externalAvailabilityOn' : 'shop.externalAvailabilityOff') }}</p>
      <label class="mt-3 block text-sm" for="external-shop-url">{{ t('shop.externalURL') }}</label>
      <div class="mt-1 flex flex-wrap items-center gap-2">
        <input id="external-shop-url" v-model="externalURL" type="url" class="min-w-0 flex-1" :disabled="externalPending || externalConfirmed === null" maxlength="2000" placeholder="https://" />
        <button type="button" class="shop-secondary" :disabled="externalPending || externalConfirmed === null || externalURL === confirmedExternalURL" @click="saveExternal(externalConfirmed === true)">{{ t('shop.externalSave') }}</button>
      </div>
      <p>{{ t('shop.externalURLHint') }}</p>
      <p v-if="externalError" class="shop-error" role="alert">{{ externalError }} <button v-if="externalConfirmed === null" type="button" @click="loadExternal">{{ t('shop.retry') }}</button></p>
    </div>
    <Toggle :model-value="externalConfirmed === true" :disabled="externalPending || externalConfirmed === null" :aria-label="t('shop.externalAvailability')" @update:model-value="saveExternal" />
  </section>
</template>
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { shopAPI } from './api'
import { useShopAvailability } from './availability'
import { safeExternalLink } from '@/utils/externalLinks'

const { t } = useI18n()
const { enabled: publicEnabled, apply, applyExternal } = useShopAvailability()
const confirmed = ref<boolean | null>(null)
const pending = ref(true)
const error = ref('')
const externalConfirmed = ref<boolean | null>(null)
const externalPending = ref(true)
const externalURL = ref('')
const confirmedExternalURL = ref('')
const externalError = ref('')

async function load() {
  pending.value = true
  error.value = ''
  try {
    const settings = await shopAPI.settings()
    confirmed.value = settings.enabled
    apply(settings.enabled)
  } catch (e) { error.value = (e as Error).message || t('shop.loadError') }
  finally { pending.value = false }
}

async function save(enabled: boolean) {
  if (pending.value || confirmed.value === null) return
  pending.value = true
  error.value = ''
  try {
    const settings = await shopAPI.saveSettings(enabled)
    confirmed.value = settings.enabled
    apply(settings.enabled, true)
  } catch (e) { error.value = (e as Error).message || t('shop.availabilitySaveError') }
  finally { pending.value = false }
}

async function loadExternal() {
  externalPending.value = true
  externalError.value = ''
  try {
    const settings = await shopAPI.externalSettings()
    externalConfirmed.value = settings.enabled
    externalURL.value = settings.url
    confirmedExternalURL.value = settings.url
    applyExternal(settings.enabled, settings.url)
  } catch (e) { externalError.value = (e as Error).message || t('shop.loadError') }
  finally { externalPending.value = false }
}

async function saveExternal(enabled: boolean) {
  if (externalPending.value || externalConfirmed.value === null) return
  const link = externalURL.value.trim()
  if ((link && !safeExternalLink(link, '')) || (enabled && !link)) {
    externalError.value = t('shop.externalInvalidURL')
    return
  }
  externalPending.value = true
  externalError.value = ''
  try {
    const settings = await shopAPI.saveExternalSettings({ enabled, url: link })
    externalConfirmed.value = settings.enabled
    externalURL.value = settings.url
    confirmedExternalURL.value = settings.url
    applyExternal(settings.enabled, settings.url, true)
  } catch (e) { externalError.value = (e as Error).message || t('shop.availabilitySaveError') }
  finally { externalPending.value = false }
}

onMounted(() => { void load(); void loadExternal() })
watch(publicEnabled, enabled => {
  if (!pending.value && confirmed.value !== null) confirmed.value = enabled
})
</script>
