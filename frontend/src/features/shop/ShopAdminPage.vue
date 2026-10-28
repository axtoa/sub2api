<template>
  <ShopLayout admin>
    <ShopAvailabilityControl />
    <ShopHomepageAdmin v-if="route.path.endsWith('/homepage')" />
    <ShopOrders v-else-if="!route.path.endsWith('/products')" admin />
    <section v-else>
      <div class="shop-section-heading"><h2>{{ t('shop.products') }}</h2><div class="shop-toolbar"><button type="button" class="shop-icon-button" :disabled="loading || busy" :title="t('shop.refresh')" :aria-label="t('shop.refresh')" @click="load"><Icon name="refresh" size="sm" /></button><button type="button" class="shop-secondary" :disabled="busy" @click="edit()"><Icon name="plus" size="sm" />{{ t('shop.addProduct') }}</button></div></div>
      <p class="shop-caption">{{ t('shop.productHint') }}</p><p v-if="error" class="shop-error" role="alert">{{ error }}</p>
      <div v-if="loading" class="shop-empty">{{ t('common.loading') }}</div>
      <div v-else class="shop-admin-grid">
        <section class="shop-product-grid"><button v-for="p in products" :key="p.id" type="button" class="shop-product-choice" :disabled="busy" :aria-label="`${t('shop.editProduct')} ${p.brand} ${p.name}`" :aria-pressed="form?.id === p.id" @click="edit(p)"><ProductCard :product="p" :selected="form?.id === p.id"><span class="shop-status" :data-status="p.active ? 'PAID' : 'CLOSED'">{{ t(p.active ? 'shop.shown' : 'shop.hidden') }}</span></ProductCard></button></section>
        <form v-if="form" class="shop-form shop-product-editor" @submit.prevent="save">
          <h2>{{ t(form.id ? 'shop.editProduct' : 'shop.addProduct') }}</h2>
          <fieldset :disabled="busy"><div class="shop-form-pair"><label>{{ t('shop.brand') }}<select v-model="form.brand"><option>ChatGPT</option><option>Anthropic</option></select></label><label>{{ t('shop.productName') }}<input v-model="form.name" required maxlength="80" /></label></div>
          <div class="shop-form-pair"><label>{{ t('shop.spec') }}<input v-model="form.spec" required maxlength="80" /></label><label>{{ t('shop.price') }}<input v-model="price" type="text" inputmode="decimal" placeholder="123.00" /></label></div>
          <p class="shop-caption">{{ t('shop.priceHint') }}</p>
          <label>{{ t('shop.paymentURL') }}<input v-model="form.payment_url" type="url" maxlength="2000" placeholder="https://" /></label>
          <label>{{ t('shop.description') }}<textarea v-model="form.description" maxlength="1000" rows="2"></textarea></label>
          <label>{{ t('shop.requirements') }}<textarea v-model="form.requirements" maxlength="1000" rows="2"></textarea></label>
          <label>{{ t('shop.purchaseNotice') }}<textarea v-model="form.purchase_notice" required maxlength="6000" rows="5"></textarea></label>
          <div class="shop-form-pair"><label>{{ t('shop.deliveryHours') }}<input v-model.number="form.delivery_hours" type="number" min="1" max="720" step="1" required /></label><label>{{ t('shop.sort') }}<input v-model.number="form.sort" type="number" min="0" max="100000" step="1" required /></label></div>
          <div class="shop-form-pair"><label>{{ t('shop.referenceAmount') }}<input v-model="referencePrice" type="number" min="0.01" max="1000000" step="0.01" /></label><label>{{ t('shop.referenceCurrency') }}<select v-model="form.reference_currency"><option v-for="currency in currencies" :key="currency">{{ currency }}</option></select></label></div>
          <label>{{ t('shop.source') }}<input v-model="form.reference_url" type="url" maxlength="2000" :required="!!referencePrice" placeholder="https://" /></label><label>{{ t('shop.checked') }}<input v-model="form.reference_date" type="date" :required="!!referencePrice" /></label>
          <label class="shop-checkbox"><input v-model="form.active" type="checkbox" /><span>{{ t('shop.active') }}</span></label></fieldset>
          <p v-if="saveError" class="shop-error" role="alert">{{ saveError }}</p><p v-if="saved" class="shop-success" role="status">{{ t('shop.saved') }}</p>
          <div class="shop-toolbar"><button type="submit" class="shop-primary" :disabled="busy || stale">{{ busy ? t('common.submitting') : t('shop.saveProduct') }}</button><button type="button" class="shop-plain" :disabled="busy" @click="form = undefined">{{ t('shop.cancelEdit') }}</button></div>
        </form>
        <div v-else class="shop-empty">{{ t('shop.selectProduct') }}</div>
      </div>
    </section>
  </ShopLayout>
</template>
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ShopLayout from './ShopLayout.vue'
import ShopAvailabilityControl from './ShopAvailabilityControl.vue'
import ShopHomepageAdmin from './ShopHomepageAdmin.vue'
import ShopOrders from './ShopOrders.vue'
import ProductCard from './ProductCard.vue'
import { checkoutLink, priceToCents, shopAPI, type ShopProduct } from './api'
const { t } = useI18n()
const route = useRoute()
const currencies = ['USD', 'CNY', 'EUR', 'HKD', 'GBP', 'JPY']
const products = ref<ShopProduct[]>([]), form = ref<ShopProduct>()
const price = ref(''), referencePrice = ref(''), error = ref(''), saveError = ref('')
const loading = ref(false), busy = ref(false), saved = ref(false), stale = ref(false)
function edit(product?: ShopProduct) {
  form.value = product ? { ...product } : { id: '', brand: 'ChatGPT', name: '', spec: '月度订阅', description: '', requirements: '', purchase_notice: '', delivery_hours: 24, price_cents: null, payment_url: '', reference_amount: null, reference_currency: 'USD', reference_url: '', reference_date: '', active: false, sort: products.value.length * 10 + 10, version: 0 }
  price.value = product?.price_cents != null ? (product.price_cents / 100).toFixed(2) : ''
  referencePrice.value = product?.reference_amount != null ? String(product.reference_amount) : ''
  saveError.value = ''; saved.value = false; stale.value = false
}
async function load() {
  loading.value = true; error.value = ''
  try { products.value = await shopAPI.products(true); form.value = undefined; stale.value = false } catch (e) { error.value = (e as Error).message || t('shop.loadError') } finally { loading.value = false }
}
async function save() {
  if (!form.value || busy.value || stale.value) return
  saveError.value = ''; saved.value = false
  let cents: number | null
  try { cents = priceToCents(price.value) } catch { saveError.value = t('shop.invalidPrice'); return }
  if ((form.value.payment_url && !checkoutLink(form.value.payment_url)) || (form.value.reference_url && !checkoutLink(form.value.reference_url))) { saveError.value = t('shop.invalidLink'); return }
  busy.value = true
  try {
    const product = await shopAPI.save({ ...form.value, price_cents: cents, reference_amount: referencePrice.value.trim() ? Number(referencePrice.value) : null })
    products.value = [...products.value.filter(p => p.id !== product.id), product].sort((a, b) => a.sort - b.sort || a.id.localeCompare(b.id))
    edit(product); saved.value = true
  } catch (e) { saveError.value = (e as Error).message || t('shop.stale'); if ((e as { status?: number }).status === 409) stale.value = true } finally { busy.value = false }
}
onMounted(() => { if (route.path.endsWith('/products')) void load() })
watch(() => route.path.endsWith('/products'), (isProducts) => { if (isProducts) void load() })
</script>
