<template>
  <ShopOrderDetail v-if="route.params.id" :key="String(route.params.id)" :admin="admin" />
  <section v-else>
    <div class="shop-section-heading"><h2>{{ t(admin ? 'shop.allOrders' : 'shop.myOrders') }}</h2><button class="shop-icon-button" type="button" :title="t('shop.refresh')" :aria-label="t('shop.refresh')" :disabled="loading" @click="load"><Icon name="refresh" size="sm" /></button></div>
    <form class="shop-filters" @submit.prevent="search">
      <input v-model="filters.q" type="search" maxlength="200" :placeholder="t('shop.search')" :aria-label="t('shop.search')" />
      <select v-model="filters.payment" :aria-label="t('shop.payment')"><option value="">{{ t('shop.payment') }} · {{ t('shop.filterAll') }}</option><option v-for="s in paymentStates" :key="s" :value="s">{{ t(statusKey(s)) }}</option></select>
      <select v-model="filters.fulfillment" :aria-label="t('shop.fulfillment')"><option value="">{{ t('shop.fulfillment') }} · {{ t('shop.filterAll') }}</option><option v-for="s in fulfillmentStates" :key="s" :value="s">{{ t(statusKey(s)) }}</option></select>
      <select v-if="admin" v-model="filters.product" :aria-label="t('shop.products')"><option value="">{{ t('shop.products') }} · {{ t('shop.filterAll') }}</option><option v-for="p in products" :key="p.id" :value="p.id">{{ p.brand }} {{ p.name }}</option></select>
      <label>{{ t('shop.from') }}<input v-model="filters.from" type="date" :max="filters.to || undefined" /></label><label>{{ t('shop.to') }}<input v-model="filters.to" type="date" :min="filters.from || undefined" /></label>
      <button type="submit" class="shop-secondary" :disabled="loading">{{ t('shop.searchButton') }}</button><button type="button" class="shop-plain" :disabled="loading" @click="clear">{{ t('shop.clear') }}</button>
    </form>
    <p v-if="error" class="shop-error" role="alert">{{ error }}</p>
    <div v-if="loading" class="shop-empty" aria-live="polite">{{ t('common.loading') }}</div>
    <div v-else-if="!orders.length" class="shop-empty">{{ t('shop.emptyOrders') }}</div>
    <div v-else class="shop-table-scroll"><table class="shop-table"><thead><tr><th>{{ t('shop.orderId') }}</th><th>{{ t('shop.catalog') }}</th><th>{{ t('shop.target') }}</th><th>{{ t('shop.amount') }}</th><th>{{ t('shop.payment') }}</th><th>{{ t('shop.fulfillment') }}</th><th>{{ t('shop.createdAt') }}</th></tr></thead><tbody><tr v-for="order in orders" :key="order.id"><td><router-link class="shop-order-link" :to="`${admin ? '/admin/shop' : '/shop'}/orders/${order.id}`">{{ order.id }}</router-link></td><td>{{ order.product.brand }} {{ order.product.name }}<small>{{ order.product.spec }}</small></td><td>{{ order.target_email }}<small v-if="admin">{{ order.applicant_email }}</small></td><td class="shop-nowrap">{{ shopMoney(order.amount_cents) }}</td><td><span class="shop-status" :data-status="order.payment_status">{{ t(statusKey(order.payment_status)) }}</span></td><td><span class="shop-status" :data-status="order.fulfillment_status">{{ t(statusKey(order.fulfillment_status)) }}</span></td><td class="shop-nowrap">{{ shopDate(order.created_at) }}</td></tr></tbody></table></div>
    <div class="shop-pagination"><span>{{ t('shop.page', { total, page, pages }) }}</span><div><button class="shop-icon-button" type="button" :disabled="loading || page <= 1" :title="t('shop.previous')" :aria-label="t('shop.previous')" @click="paginate(-1)"><Icon name="arrowLeft" size="sm" /></button><button class="shop-icon-button" type="button" :disabled="loading || page >= pages" :title="t('shop.next')" :aria-label="t('shop.next')" @click="paginate(1)"><Icon name="arrowRight" size="sm" /></button></div></div>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ShopOrderDetail from './ShopOrderDetail.vue'
import { fulfillmentStates, paymentStates, shopAPI, shopDate, shopMoney, statusKey, type ShopOrder, type ShopProduct } from './api'
const props = defineProps<{ admin?: boolean }>()
const { t } = useI18n()
const route = useRoute()
const orders = ref<ShopOrder[]>([])
const products = ref<ShopProduct[]>([])
const loading = ref(false)
const error = ref('')
const total = ref(0)
const page = ref(1)
const pages = computed(() => Math.max(1, Math.ceil(total.value / 20)))
const filters = reactive({ q: '', payment: '', fulfillment: '', product: '', from: '', to: '' })
let applied = { ...filters }
let generation = 0
async function load() {
  const current = ++generation
  loading.value = true; error.value = ''
  try {
    const result = await shopAPI.orders(!!props.admin, { ...applied, page: page.value })
    if (current !== generation) return
    orders.value = result.items; total.value = result.total
    if (page.value > pages.value) { page.value = pages.value; await load() }
  } catch (e) { if (current === generation) error.value = (e as Error).message || t('shop.loadError') } finally { if (current === generation) loading.value = false }
}
function search() { applied = { ...filters }; page.value = 1; void load() }
function clear() { Object.assign(filters, { q: '', payment: '', fulfillment: '', product: '', from: '', to: '' }); search() }
function paginate(direction: number) { page.value += direction; void load() }
onMounted(() => {
  if (!route.params.id) void load()
  if (props.admin) void shopAPI.products(true).then(result => { products.value = result }).catch(() => { /* Orders remain usable when product filters cannot load. */ })
})
watch(() => route.params.id, (id, previous) => { if (!id && previous) void load() })
</script>
