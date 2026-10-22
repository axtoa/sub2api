<template>
  <ShopLayout>
    <ShopOrders v-if="route.path.startsWith('/shop/orders')" />
    <p v-else-if="!enabled" class="shop-empty" role="status">{{ t('shop.paused') }}</p>
    <template v-else>
      <p v-if="error" class="shop-error" role="alert">{{ error }} <button type="button" @click="load">{{ t('shop.retry') }}</button></p>
      <div v-if="loading" class="shop-empty">{{ t('common.loading') }}</div>
      <div v-else-if="!products.length" class="shop-empty">{{ t('shop.empty') }}</div>
      <div v-else class="shop-checkout-grid">
        <section>
          <div class="shop-section-heading"><h2>{{ t('shop.catalog') }}</h2><div class="shop-segments" role="group" :aria-label="t('shop.brand')"><button v-for="brand in brands" :key="brand" type="button" :disabled="busy" :aria-pressed="selectedBrand === brand" @click="chooseBrand(brand)">{{ brand }}</button></div></div>
          <div class="shop-product-grid">
            <button v-for="product in visible" :key="product.id" type="button" class="shop-product-choice" :disabled="busy" :aria-pressed="selected?.id === product.id" @click="choose(product.id)"><ProductCard :product="product" :selected="selected?.id === product.id" /></button>
          </div>
          <p class="shop-caption">{{ t('shop.referenceHint') }}</p>
          <a v-if="selected && checkoutLink(selected.reference_url)" :href="checkoutLink(selected.reference_url)" target="_blank" rel="noopener noreferrer" class="shop-text-link">{{ t('shop.source') }}<Icon name="externalLink" size="sm" /><span>{{ t('shop.checked') }} {{ selected.reference_date }}</span></a>
        </section>
        <section v-if="selected" class="shop-checkout-form">
          <h2>{{ t('shop.accountTitle') }}</h2>
          <form class="shop-form" @submit.prevent="submit">
            <div class="shop-applicant"><span>{{ t('shop.applicant') }}</span><strong>{{ auth.user?.email || t('shop.login') }}</strong></div>
            <label>{{ t('shop.target') }}<input v-model="targetEmail" type="email" maxlength="254" :disabled="busy" :placeholder="auth.user?.email || ''" @input="resetReview" /><small>{{ t('shop.targetHint') }}</small></label>
            <label>{{ t('shop.note') }}<textarea v-model="note" rows="2" maxlength="1000" :disabled="busy" :placeholder="t('shop.noteHint')" @input="resetReview"></textarea></label>
            <div class="shop-product-description"><strong>{{ selected.brand }} {{ selected.name }} · {{ selected.spec }}</strong><p>{{ selected.description }}</p><p>{{ selected.requirements }}</p><small>{{ t('shop.delivery', { hours: selected.delivery_hours }) }}</small></div>
            <section class="shop-notice"><h3>{{ t('shop.purchaseNotice') }}</h3><ol><li v-for="(line, index) in noticeLines" :key="index">{{ line }}</li></ol></section>
            <label class="shop-checkbox"><input v-model="accepted" type="checkbox" required :disabled="busy" /><span>{{ t('shop.accepted') }}</span></label>
            <p v-if="submitError" class="shop-error" role="alert">{{ submitError }} <button v-if="stale" type="button" @click="refreshProduct">{{ t('shop.refresh') }}</button></p>
            <button v-if="auth.isAuthenticated" type="submit" class="shop-primary" :disabled="busy || stale || !accepted || !purchasable">{{ busy ? t('common.submitting') : purchasable ? `${t('shop.create')} · ${shopMoney(selected.price_cents!)}` : t('shop.upcoming') }}</button>
            <router-link v-else class="shop-primary" :to="{ path: '/login', query: { redirect: route.fullPath } }">{{ t('shop.login') }}</router-link>
            <p class="shop-caption">{{ t('shop.flow') }}</p>
          </form>
        </section>
      </div>
    </template>
  </ShopLayout>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import Icon from '@/components/icons/Icon.vue'
import ShopLayout from './ShopLayout.vue'
import ShopOrders from './ShopOrders.vue'
import ProductCard from './ProductCard.vue'
import { checkoutLink, shopAPI, shopMoney, type ShopProduct } from './api'
import { isShopDisabled, useShopAvailability } from './availability'
import { createShopEntryId } from './entryFlow'
const { t } = useI18n()
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const { enabled, apply } = useShopAvailability()
const products = ref<ShopProduct[]>([])
const selectedId = ref('')
const selectedBrand = ref('ChatGPT')
const targetEmail = ref(auth.user?.email || '')
const note = ref('')
const accepted = ref(false)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const submitError = ref('')
const stale = ref(false)
let requestKey = ''
let loadVersion = 0
const brands = computed(() => [...new Set(products.value.map(p => p.brand))])
const visible = computed(() => products.value.filter(p => p.brand === selectedBrand.value))
const selected = computed(() => visible.value.find(p => p.id === selectedId.value) || visible.value[0])
const noticeLines = computed(() => (selected.value?.purchase_notice || '').split(/\n+/).filter(Boolean))
const purchasable = computed(() => selected.value?.price_cents != null && selected.value.price_cents > 0 && !!checkoutLink(selected.value.payment_url))
function resetReview() { accepted.value = false; requestKey = ''; if (!stale.value) submitError.value = '' }
function choose(id: string) { selectedId.value = id; resetReview() }
function chooseBrand(brand: string) { selectedBrand.value = brand; choose('') }
async function load() {
  if (!enabled.value) return false
  const version = ++loadVersion
  resetReview()
  loading.value = true; error.value = ''
  try {
    const items = await shopAPI.products()
    if (version !== loadVersion || !enabled.value) return false
    products.value = items
    const preferred = products.value.find(p => p.id === route.query.product)
    selectedId.value = preferred?.id || ''
    selectedBrand.value = preferred?.brand || brands.value[0] || 'ChatGPT'
    return true
  } catch (e) {
    if (version === loadVersion) {
      products.value = []
      resetReview()
      if (isShopDisabled(e)) apply(false)
      else error.value = (e as Error).message || t('shop.loadError')
    }
    return false
  } finally { if (version === loadVersion) loading.value = false }
}
async function submit() {
  if (!enabled.value || !auth.isAuthenticated || loading.value || busy.value || stale.value || !selected.value || !purchasable.value || !accepted.value) return
  busy.value = true; submitError.value = ''
  try {
    requestKey ||= createShopEntryId() || ''
    if (!requestKey) throw new Error(t('shop.requestIdError'))
    const order = await shopAPI.create({ product_id: selected.value.id, product_version: selected.value.version, target_email: targetEmail.value.trim(), note: note.value.trim(), accepted: true, request_key: requestKey })
    await router.push(`/shop/orders/${order.id}`)
  } catch (e) {
    if (isShopDisabled(e)) apply(false)
    else { submitError.value = (e as Error).message || t('shop.stale'); stale.value = (e as { status?: number }).status === 409 }
  } finally { busy.value = false }
}
async function refreshProduct() { if (await load()) { stale.value = false; resetReview() } }
watch([() => !route.path.startsWith('/shop/orders'), enabled], ([catalog, open]) => {
  if (!open) {
    loadVersion++
    products.value = []
    loading.value = false
    stale.value = false
    resetReview()
  } else if (catalog) void load()
}, { immediate: true })
</script>
