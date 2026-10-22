<template>
  <section class="shop-availability" :aria-label="t('shop.availability')" :aria-busy="pending">
    <div>
      <h2>{{ t('shop.availability') }}</h2>
      <p role="status">{{ pending ? t('common.loading') : confirmed === null ? t('shop.availabilityUnknown') : t(confirmed ? 'shop.availabilityOn' : 'shop.availabilityOff') }}</p>
      <p v-if="error" class="shop-error" role="alert">{{ error }} <button v-if="confirmed === null" type="button" @click="load">{{ t('shop.retry') }}</button></p>
    </div>
    <Toggle :model-value="confirmed === true" :disabled="pending || confirmed === null" :aria-label="t('shop.availability')" @update:model-value="save" />
  </section>
</template>
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { shopAPI } from './api'
import { useShopAvailability } from './availability'

const { t } = useI18n()
const { enabled: publicEnabled, apply } = useShopAvailability()
const confirmed = ref<boolean | null>(null)
const pending = ref(true)
const error = ref('')

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
onMounted(load)
watch(publicEnabled, enabled => {
  if (!pending.value && confirmed.value !== null) confirmed.value = enabled
})
</script>
