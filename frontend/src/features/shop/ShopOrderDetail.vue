<template>
  <section>
    <div class="shop-section-heading"><router-link class="shop-text-link" :to="admin ? '/admin/shop' : '/shop/orders'"><Icon name="arrowLeft" size="sm" />{{ t(admin ? 'shop.allOrders' : 'shop.myOrders') }}</router-link><button class="shop-icon-button" type="button" :disabled="loading || busy" :title="t('shop.refresh')" :aria-label="t('shop.refresh')" @click="load"><Icon name="refresh" size="sm" /></button></div>
    <p v-if="error" class="shop-error" role="alert">{{ error }}</p>
    <div v-if="loading" class="shop-empty">{{ t('common.loading') }}</div>
    <template v-else-if="order">
      <div class="shop-order-heading"><div><p class="shop-caption">{{ t('shop.orderId') }}</p><h2 class="shop-order-id">{{ order.id }}</h2></div><strong class="shop-price">{{ shopMoney(order.amount_cents) }}</strong></div>
      <div class="shop-order-status"><span class="shop-status" :data-status="order.payment_status">{{ t(statusKey(order.payment_status)) }}</span><span class="shop-status" :data-status="order.fulfillment_status">{{ t(statusKey(order.fulfillment_status)) }}</span></div>
      <div class="shop-detail-grid">
        <section>
          <h3>{{ order.product.brand }} {{ order.product.name }} · {{ order.product.spec }}</h3>
          <dl class="shop-facts"><div><dt>{{ t('shop.applicant') }}</dt><dd>{{ order.applicant_email }}</dd></div><div><dt>{{ t('shop.target') }}</dt><dd>{{ order.target_email }}</dd></div><div><dt>{{ t('shop.contact') }}</dt><dd>{{ order.contact_email }}</dd></div><div><dt>{{ t('shop.createdAt') }}</dt><dd>{{ shopDate(order.created_at) }}</dd></div><div v-if="order.external_reference"><dt>{{ t('shop.externalReference') }}</dt><dd>{{ order.external_reference }}</dd></div><div v-if="order.note"><dt>{{ t('shop.note') }}</dt><dd class="shop-prewrap">{{ order.note }}</dd></div><div v-if="order.delivery_note"><dt>{{ t('shop.deliveryNote') }}</dt><dd class="shop-prewrap">{{ order.delivery_note }}</dd></div><div v-if="order.valid_until"><dt>{{ t('shop.validUntil') }}</dt><dd>{{ shopDate(order.valid_until) }}</dd></div></dl>
          <details class="shop-snapshot"><summary>{{ t('shop.purchaseNotice') }}</summary><p>{{ order.product.description }}</p><p>{{ order.product.requirements }}</p><p class="shop-prewrap">{{ order.product.purchase_notice }}</p></details>
          <section class="shop-history"><h3>{{ t('shop.history') }}</h3><ol><li v-for="(event, index) in order.events" :key="index"><div><strong>{{ t(`shop.${event.action}`) }}</strong><time>{{ shopDate(event.at) }}</time></div><p v-if="event.note" class="shop-prewrap">{{ event.note }}</p></li></ol></section>
        </section>
        <section class="shop-order-actions">
          <template v-if="!admin && order.payment_status === 'PENDING'">
            <h3>{{ t('shop.paymentLink') }}</h3><p class="shop-muted">{{ t('shop.paymentHint') }}</p>
            <a v-if="paymentURL" :href="paymentURL" target="_blank" rel="noopener noreferrer" class="shop-primary">{{ t('shop.paymentLink') }}<Icon name="externalLink" size="sm" /></a>
          </template>
          <form v-if="actions.length" class="shop-form" @submit.prevent="submit">
            <label>{{ t('shop.operation') }}<select v-model="action" :disabled="busy" @change="resetAction"><option v-for="choice in actions" :key="choice" :value="choice">{{ t(choice === 'payment_reference' ? 'shop.submitReference' : `shop.${choice}`) }}</option></select></label>
            <label v-if="action === 'payment_reference' || action === 'confirm_paid'">{{ t('shop.externalReference') }}<input v-model="reference" required maxlength="200" :disabled="busy" /></label>
            <label v-if="action === 'supplement'">{{ t('shop.target') }}<input v-model="target" type="email" required maxlength="254" :disabled="busy" /></label>
            <label v-if="action !== 'payment_reference'">{{ t('shop.actionNote') }}<textarea v-model="note" :required="['needs_info', 'complete', 'confirm_refunded'].includes(action)" maxlength="3000" rows="3" :disabled="busy"></textarea></label>
            <label v-if="action === 'complete'">{{ t('shop.validUntil') }}<input v-model="validUntil" type="datetime-local" :disabled="busy" /></label>
            <p v-if="action === 'confirm_refunded'" class="shop-caption">{{ t('shop.refundHint') }}</p>
            <label v-if="['cancel', 'confirm_paid', 'confirm_refunded', 'complete'].includes(action)" class="shop-checkbox"><input v-model="confirmed" type="checkbox" required :disabled="busy" /><span>{{ t(`shop.confirm_${action}`) }}</span></label>
            <p v-if="actionError" class="shop-error" role="alert">{{ actionError }}</p>
            <p v-if="saved" class="shop-success" role="status">{{ t('shop.saved') }}</p>
            <button type="submit" class="shop-primary" :disabled="busy || stale">{{ busy ? t('common.submitting') : t('shop.saveAction') }}</button>
          </form>
          <p v-else class="shop-caption">{{ t('shop.noActions') }}</p>
        </section>
      </div>
    </template>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { checkoutLink, shopAPI, shopDate, shopMoney, statusKey, type ShopOrder } from './api'
const props = defineProps<{ admin?: boolean }>()
const { t } = useI18n()
const route = useRoute()
const order = ref<ShopOrder>()
const loading = ref(true), busy = ref(false), stale = ref(false), confirmed = ref(false), saved = ref(false)
const error = ref(''), actionError = ref(''), action = ref(''), note = ref(''), reference = ref(''), target = ref(''), validUntil = ref('')
const paymentURL = computed(() => checkoutLink(order.value?.product.payment_url || ''))
const actions = computed(() => {
  const o = order.value
  if (!o) return []
  const pending = o.payment_status === 'PENDING', paid = o.payment_status === 'PAID'
  if (!props.admin) return [...(pending ? ['payment_reference', 'cancel'] : []), ...(pending || (paid && ['WAITING', 'NEEDS_INFO'].includes(o.fulfillment_status)) ? ['supplement'] : [])]
  return [...(pending || o.payment_status === 'CANCELLED' ? ['confirm_paid'] : []), ...(pending ? ['cancel'] : []), ...(paid && ['WAITING', 'NEEDS_INFO'].includes(o.fulfillment_status) ? ['process'] : []), ...(paid && ['WAITING', 'PROCESSING', 'NEEDS_INFO'].includes(o.fulfillment_status) ? ['needs_info'] : []), ...(paid && o.fulfillment_status === 'PROCESSING' ? ['complete'] : []), ...(paid ? ['confirm_refunded'] : [])]
})
function resetAction() { note.value = action.value === 'supplement' ? order.value?.note || '' : ''; reference.value = order.value?.external_reference || ''; target.value = order.value?.target_email || ''; confirmed.value = false; validUntil.value = ''; saved.value = false; actionError.value = '' }
async function load() {
  loading.value = true; error.value = ''
  try { order.value = await shopAPI.order(String(route.params.id), !!props.admin); action.value = actions.value[0] || ''; resetAction(); stale.value = false } catch (e) { error.value = (e as Error).message || t('shop.loadError') } finally { loading.value = false }
}
async function submit() {
  if (!order.value || busy.value || stale.value || !actions.value.includes(action.value)) return
  if (['cancel', 'confirm_paid', 'confirm_refunded', 'complete'].includes(action.value) && !confirmed.value) return
  busy.value = true; actionError.value = ''; saved.value = false
  try {
    const expiry = validUntil.value ? new Date(validUntil.value).toISOString() : ''
    order.value = await shopAPI.action(order.value.id, { action: action.value, version: order.value.version, note: note.value.trim(), external_reference: reference.value.trim(), target_email: target.value.trim(), valid_until: expiry }, !!props.admin)
    action.value = actions.value[0] || ''; resetAction(); saved.value = true
  } catch (e) {
    actionError.value = (e as Error).message || t('shop.stale')
    if ((e as { status?: number }).status === 409) { stale.value = true; actionError.value += ` ${t('shop.stale')}` }
  } finally { busy.value = false }
}
onMounted(load)
</script>
