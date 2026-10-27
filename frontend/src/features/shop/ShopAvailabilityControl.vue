<template>
  <div class="shop-availability-grid">
  <section class="shop-availability" :aria-label="officialName" :aria-busy="pending">
    <div class="shop-availability-content">
      <div class="shop-setting-heading">
        <h2>{{ officialName }}</h2>
        <button v-if="!nameEditing.official" type="button" class="shop-icon-button shop-edit-button" :title="t('shop.editName')" :aria-label="t('shop.editName') + ': ' + officialName" :disabled="labelsPending" @click="beginNameEdit('official')"><Icon name="edit" size="sm" /></button>
      </div>
      <p v-if="pending || confirmed !== true" role="status">{{ pending ? t('common.loading') : confirmed === null ? t('shop.availabilityUnknown') : t('shop.availabilityOff') }}</p>
      <form v-if="nameEditing.official" class="shop-inline-editor" @submit.prevent="saveName('official')">
        <label for="official-shop-name">{{ t('shop.menuName') }}</label>
        <input id="official-shop-name" v-model="nameDraft.official" maxlength="24" :placeholder="t('shop.entry')" :disabled="nameSaving.official" />
        <div class="shop-editor-actions"><button type="submit" class="shop-secondary" :disabled="nameSaving.official">{{ t('shop.saveName') }}</button><button type="button" class="shop-plain" :disabled="nameSaving.official" @click="cancelNameEdit('official')">{{ t('shop.cancelEdit') }}</button></div>
        <p>{{ t('shop.nameHint') }}</p>
      </form>
      <p v-if="error" class="shop-error" role="alert">{{ error }} <button v-if="confirmed === null" type="button" @click="load">{{ t('shop.retry') }}</button></p>
      <p v-if="nameError.official || labelsError" class="shop-error" role="alert">{{ nameError.official || labelsError }} <button v-if="labelsError" type="button" @click="loadLabels">{{ t('shop.retry') }}</button></p>
    </div>
    <Toggle :model-value="confirmed === true" :disabled="pending || confirmed === null" :aria-label="t('shop.availability')" @update:model-value="save" />
  </section>
  <section class="shop-availability shop-availability-external" :aria-label="externalName" :aria-busy="externalPending">
    <div class="shop-availability-content">
      <div class="shop-setting-heading">
        <h2>{{ externalName }}</h2>
        <button v-if="!nameEditing.external" type="button" class="shop-icon-button shop-edit-button" :title="t('shop.editName')" :aria-label="t('shop.editName') + ': ' + externalName" :disabled="labelsPending" @click="beginNameEdit('external')"><Icon name="edit" size="sm" /></button>
      </div>
      <p v-if="externalPending || externalConfirmed !== true" role="status">{{ externalPending ? t('common.loading') : externalConfirmed === null ? t('shop.availabilityUnknown') : t('shop.externalAvailabilityOff') }}</p>
      <form v-if="nameEditing.external" class="shop-inline-editor" @submit.prevent="saveName('external')">
        <label for="external-shop-name">{{ t('shop.menuName') }}</label>
        <input id="external-shop-name" v-model="nameDraft.external" maxlength="24" :placeholder="t('shop.externalEntry')" :disabled="nameSaving.external" />
        <div class="shop-editor-actions"><button type="submit" class="shop-secondary" :disabled="nameSaving.external">{{ t('shop.saveName') }}</button><button type="button" class="shop-plain" :disabled="nameSaving.external" @click="cancelNameEdit('external')">{{ t('shop.cancelEdit') }}</button></div>
        <p>{{ t('shop.nameHint') }}</p>
      </form>
      <div class="shop-link-setting">
        <div v-if="!urlEditing" class="shop-readonly-row">
          <span class="shop-readonly-value">{{ confirmedExternalURL || t('shop.noURL') }}</span>
          <button type="button" class="shop-icon-button shop-edit-button" :title="t('shop.editURL')" :aria-label="t('shop.editURL')" :disabled="externalPending || externalConfirmed === null" @click="beginURLEdit"><Icon name="edit" size="sm" /></button>
        </div>
        <form v-else class="shop-inline-editor" novalidate @submit.prevent="saveExternal(externalConfirmed === true, externalURL)">
          <label class="sr-only" for="external-shop-url">{{ t('shop.externalURL') }}</label>
          <input id="external-shop-url" v-model="externalURL" type="url" :disabled="externalPending" maxlength="2000" placeholder="https://" />
          <div class="shop-editor-actions"><button type="submit" class="shop-secondary" :disabled="externalPending">{{ t('shop.externalSave') }}</button><button type="button" class="shop-plain" :disabled="externalPending" @click="cancelURLEdit">{{ t('shop.cancelEdit') }}</button></div>
          <p>{{ t('shop.externalURLHint') }}</p>
        </form>
      </div>
      <p v-if="externalError" class="shop-error" role="alert">{{ externalError }} <button v-if="externalConfirmed === null" type="button" @click="loadExternal">{{ t('shop.retry') }}</button></p>
      <p v-if="nameError.external" class="shop-error" role="alert">{{ nameError.external }}</p>
    </div>
    <Toggle :model-value="externalConfirmed === true" :disabled="externalPending || externalConfirmed === null || urlEditing" :aria-label="t('shop.externalAvailability')" @update:model-value="enabled => saveExternal(enabled, confirmedExternalURL)" />
  </section>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { shopAPI, type ShopChannel } from './api'
import { useShopAvailability } from './availability'
import { safeExternalLink } from '@/utils/externalLinks'

const { t } = useI18n()
const { enabled: publicEnabled, apply, applyExternal, applyLabel } = useShopAvailability()
const confirmed = ref<boolean | null>(null)
const pending = ref(true)
const error = ref('')
const externalConfirmed = ref<boolean | null>(null)
const externalPending = ref(true)
const externalURL = ref('')
const confirmedExternalURL = ref('')
const externalError = ref('')
const urlEditing = ref(false)
const labelsPending = ref(true)
const labelsError = ref('')
const names = reactive({ official: '', external: '' })
const nameDraft = reactive({ official: '', external: '' })
const nameEditing = reactive({ official: false, external: false })
const nameSaving = reactive({ official: false, external: false })
const nameError = reactive({ official: '', external: '' })
const officialName = computed(() => names.official || t('shop.entry'))
const externalName = computed(() => names.external || t('shop.externalEntry'))

async function loadLabels() {
  labelsPending.value = true
  labelsError.value = ''
  try {
    const labels = await shopAPI.labels()
    names.official = labels.official_name
    names.external = labels.external_name
    applyLabel('official', labels.official_name)
    applyLabel('external', labels.external_name)
  } catch (e) { labelsError.value = (e as Error).message || t('shop.loadError') }
  finally { labelsPending.value = false }
}

function beginNameEdit(channel: ShopChannel) {
  nameDraft[channel] = names[channel]
  nameError[channel] = ''
  nameEditing[channel] = true
}

function cancelNameEdit(channel: ShopChannel) {
  nameDraft[channel] = names[channel]
  nameError[channel] = ''
  nameEditing[channel] = false
}

async function saveName(channel: ShopChannel) {
  if (nameSaving[channel]) return
  const name = nameDraft[channel].trim()
  if (Array.from(name).length > 24 || /[\p{Cc}\p{Cf}]/u.test(name)) {
    nameError[channel] = t('shop.invalidName')
    return
  }
  nameSaving[channel] = true
  nameError[channel] = ''
  try {
    const saved = await shopAPI.saveLabel(channel, name)
    names[channel] = saved.name
    applyLabel(channel, saved.name, true)
    nameEditing[channel] = false
  } catch (e) { nameError[channel] = (e as Error).message || t('shop.availabilitySaveError') }
  finally { nameSaving[channel] = false }
}

function beginURLEdit() {
  externalURL.value = confirmedExternalURL.value
  externalError.value = ''
  urlEditing.value = true
}

function cancelURLEdit() {
  externalURL.value = confirmedExternalURL.value
  externalError.value = ''
  urlEditing.value = false
}

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

async function saveExternal(enabled: boolean, url: string) {
  if (externalPending.value || externalConfirmed.value === null) return
  const link = url.trim()
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
    urlEditing.value = false
    applyExternal(settings.enabled, settings.url, true)
  } catch (e) { externalError.value = (e as Error).message || t('shop.availabilitySaveError') }
  finally { externalPending.value = false }
}

onMounted(() => { void load(); void loadExternal(); void loadLabels() })
watch(publicEnabled, enabled => {
  if (!pending.value && confirmed.value !== null) confirmed.value = enabled
})
</script>
