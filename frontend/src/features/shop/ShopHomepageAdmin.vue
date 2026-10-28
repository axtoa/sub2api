<template>
  <section class="shop-homepage-admin">
    <div class="shop-section-heading">
      <h2>{{ t('shop.homepageTab') }}</h2>
      <button type="button" class="shop-icon-button" :disabled="loading || saving" :title="t('shop.refresh')" :aria-label="t('shop.refresh')" @click="load"><Icon name="refresh" size="sm" /></button>
    </div>
    <div v-if="loading" class="shop-empty">{{ t('common.loading') }}</div>
    <form v-else class="shop-form shop-homepage-editor" @submit.prevent="save">
      <fieldset :disabled="saving">
        <label>{{ t('shop.homepageTitleField') }}<input v-model="title" required maxlength="60" /></label>
        <label>{{ t('shop.homepageLeadField') }}<textarea v-model="lead" required maxlength="180" rows="2"></textarea></label>
        <div class="shop-section-heading"><h3>{{ t('shop.homepageCards') }}</h3><button type="button" class="shop-secondary" :disabled="cards.length >= 6" @click="addCard"><Icon name="plus" size="sm" />{{ t('shop.homepageAddCard') }}</button></div>
        <div class="shop-homepage-card-list">
          <div v-for="(card, index) in cards" :key="card.key" class="shop-homepage-card-editor">
            <div class="shop-section-heading"><h3>{{ index + 1 }}. {{ card.title || t('shop.homepageCardTitle') }}</h3><div class="shop-toolbar"><button type="button" class="shop-icon-button shop-edit-button" :disabled="index === 0" :title="t('shop.moveUp')" :aria-label="t('shop.moveUp')" @click="moveCard(index, -1)"><Icon name="arrowUp" size="sm" /></button><button type="button" class="shop-icon-button shop-edit-button" :disabled="index === cards.length - 1" :title="t('shop.moveDown')" :aria-label="t('shop.moveDown')" @click="moveCard(index, 1)"><Icon name="arrowDown" size="sm" /></button><button type="button" class="shop-icon-button shop-edit-button" :title="t('shop.homepageRemoveCard')" :aria-label="t('shop.homepageRemoveCard')" @click="removeCard(index)"><Icon name="trash" size="sm" /></button></div></div>
            <div class="shop-form-pair"><label>{{ t('shop.homepageCardTitle') }}<input v-model="card.title" required maxlength="60" /></label><label>{{ t('shop.homepagePriceHint') }}<input v-model="card.price_hint" maxlength="40" :placeholder="t('shop.homepagePricePlaceholder')" /></label></div>
            <div class="shop-form-pair"><label>{{ t('shop.homepageCardSummary') }}<input v-model="card.summary" required maxlength="140" /></label><label>{{ t('shop.homepageBonus') }}<input v-model="card.bonus" type="text" inputmode="decimal" placeholder="0.00" /></label></div>
          </div>
        </div>
        <p class="shop-caption">{{ t('shop.homepageManualBonus') }}</p>
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
import type { ShopHomepageCard, ShopHomepageContent } from '@/types'
import { shopAPI } from './api'
import { useShopAvailability } from './availability'

type DraftCard = Omit<ShopHomepageCard, 'bonus_cents'> & { key: number; bonus: string }
const { t } = useI18n()
const { applyHomepageContent } = useShopAvailability()
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const saved = ref(false)
const title = ref('')
const lead = ref('')
const cards = ref<DraftCard[]>([])
let nextKey = 0

function draftCard(card?: ShopHomepageCard): DraftCard {
  return { key: nextKey++, title: card?.title || '', summary: card?.summary || '', price_hint: card?.price_hint || '', bonus: card?.bonus_cents ? (card.bonus_cents / 100).toFixed(2) : '' }
}

async function load() {
  loading.value = true
  error.value = ''
  saved.value = false
  try {
    const content = await shopAPI.homepageContent()
    title.value = content?.title || t('shop.homepageTitle')
    lead.value = content?.lead || t('shop.homepageDefaultLead')
    cards.value = (content?.cards || []).map(draftCard)
  } catch (e) { error.value = (e as Error).message || t('shop.loadError') }
  finally { loading.value = false }
}

function addCard() { if (cards.value.length < 6) cards.value.push(draftCard()) }
function removeCard(index: number) { cards.value.splice(index, 1) }
function moveCard(index: number, direction: -1 | 1) {
  const target = index + direction
  if (target < 0 || target >= cards.value.length) return
  const [card] = cards.value.splice(index, 1)
  cards.value.splice(target, 0, card)
}

function bonusCents(value: string): number | null {
  const amount = value.trim()
  if (!amount) return 0
  if (!/^(?:0|[1-9]\d{0,4})(?:\.\d{1,2})?$/.test(amount)) return null
  const cents = Math.round(Number(amount) * 100)
  return Number.isSafeInteger(cents) && cents <= 1000000 ? cents : null
}

async function save() {
  if (saving.value) return
  const prepared = cards.value.map(card => ({ title: card.title.trim(), summary: card.summary.trim(), price_hint: card.price_hint.trim(), bonus_cents: bonusCents(card.bonus) }))
  if (!title.value.trim() || !lead.value.trim() || Array.from(title.value.trim()).length > 60 || Array.from(lead.value.trim()).length > 180 ||
      prepared.some(card => !card.title || !card.summary || card.bonus_cents === null)) {
    error.value = t('shop.homepageInvalid')
    return
  }
  const content: ShopHomepageContent = { title: title.value.trim(), lead: lead.value.trim(), cards: prepared as ShopHomepageCard[] }
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
