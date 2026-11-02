<template>
  <ShopLayout :embedded="false" :storefront="true">
    <main class="shop-main cdk-storefront">
      <header class="shop-heading"><p class="cdk-storefront-eyebrow">{{ t('shop.storefrontEyebrow') }}</p><h1>{{ t('shop.storefrontTitle') }}</h1><p>支付成功后自动发放兑换码，前往兑换站完成充值。</p></header>
      <p v-if="error" class="shop-error" role="alert">{{ error }}</p>
      <p v-if="!loading && !storeEnabled && !order" class="shop-paused" role="status">商城暂停售卖，已创建订单仍可查询。</p>
      <ol v-if="!order" class="cdk-workflow" aria-label="购买流程"><li><span>1</span>选择商品</li><li><span>2</span>支付领取兑换码</li><li><span>3</span>前往兑换站充值</li></ol>
      <div v-if="loading" class="shop-empty">{{ t('common.loading') }}</div>
      <template v-else-if="!order">
        <p v-if="requestedProductMissing" class="shop-paused" role="status">{{ t('shop.storefrontProductMissing') }}</p>
        <div v-if="!products.length" class="shop-empty">暂时没有可购买的商品</div>
        <div v-else class="shop-checkout-grid">
          <section><div class="shop-section-heading"><h2>选择商品</h2></div><div class="shop-product-grid">
            <button v-for="item in products" :key="item.id" class="shop-product-choice" type="button" :aria-pressed="selected?.id === item.id" :disabled="item.stock < 1" @click="selected=item">
              <article class="shop-product" :class="{selected:selected?.id===item.id}"><div class="shop-product-top"><span class="shop-brand-label">{{ t('shop.storefrontCodeDelivery') }}</span><span>{{ item.stock > 0 ? `库存 ${item.stock}` : '暂时售罄' }}</span></div><h3>{{ item.name }}</h3><p class="shop-muted">{{ item.spec }}</p><p>{{ item.description }}</p><strong class="shop-price">¥{{ (item.price_cents/100).toFixed(2) }}</strong><span v-if="item.reward_cents > 0" class="cdk-balance-reward">{{ t('shop.storefrontReward', { amount: formatStorefrontReward(item.reward_cents) }) }}</span></article>
            </button>
          </div></section>
          <section v-if="selected" id="shop-product-details" ref="checkoutCard" class="cdk-checkout-card"><h2>确认订单</h2><p class="shop-muted">{{ selected.name }} · {{ selected.spec }}</p><form class="shop-form" @submit.prevent="checkout">
            <label>接收邮箱<input v-model.trim="email" required type="email" maxlength="254" autocomplete="email" placeholder="用于找回订单" /></label>
            <p class="shop-product-description">{{ selected.instructions }}</p>
            <p v-if="selected.stock < 1" class="shop-paused" role="status">{{ t('shop.storefrontSoldOutHint') }}</p>
            <div class="cdk-checkout-total"><span>应付金额</span><strong>¥{{ (selected.price_cents/100).toFixed(2) }}</strong></div>
            <div v-if="selected.reward_cents > 0" class="cdk-checkout-reward"><span class="cdk-balance-reward">{{ t('shop.storefrontReward', { amount: formatStorefrontReward(selected.reward_cents) }) }}</span><p class="shop-caption">{{ t(auth.isAuthenticated ? 'shop.storefrontRewardTerms' : 'shop.storefrontRewardGuest') }}</p><router-link v-if="!auth.isAuthenticated" class="shop-text-link cdk-reward-login" :to="rewardLoginLink">{{ t('shop.storefrontRewardLogin') }}</router-link></div>
            <button class="shop-primary" :disabled="busy || !storeEnabled || selected.stock<1">{{ !storeEnabled ? '暂停售卖' : busy ? '正在创建订单…' : `支付宝付款 · ¥${(selected.price_cents/100).toFixed(2)}` }}</button>
            <small class="shop-caption">兑换码将在支付确认后显示，也可凭订单恢复码找回。</small>
          </form></section>
        </div>
      </template>
      <details v-if="!order" id="shop-order-lookup" class="cdk-recovery" :open="route.query.lookup === '1'"><summary>已有订单？查询 / 找回兑换码</summary><form class="shop-recovery-form" @submit.prevent="recoverOrder"><label>找回已有订单<input v-model.trim="recoverID" required autocomplete="off" placeholder="订单号" /></label><label>订单恢复码<input v-model.trim="recoverKey" required autocomplete="off" /></label><button class="shop-secondary" :disabled="busy">查询订单</button></form></details>
      <div v-if="selected && selected.stock > 0 && storeEnabled && !order && !loading && !checkoutInView" class="cdk-mobile-summary">
        <div><strong>{{ selected.name }}</strong><span>¥{{ (selected.price_cents / 100).toFixed(2) }}</span></div>
        <button type="button" class="shop-primary" @click="goToCheckout">{{ t('shop.storefrontGoToCheckout') }}</button>
      </div>
      <section v-if="!loading && order" class="cdk-order-result">
        <h2>{{ order.payment_status === 'paid' ? '支付成功' : '订单已创建' }}</h2>
        <p>{{ order.product_name }} · ¥{{ (order.price_cents/100).toFixed(2) }}</p><p>订单号：{{ order.id }}</p>
        <p v-if="order.payment_status !== 'paid'">请完成付款后刷新订单，确认到账后即可领取兑换码。</p>
        <div class="shop-toolbar">
          <a v-if="order.payment_status === 'pending' && paymentURL" class="shop-primary" :href="paymentURL" target="_blank" rel="noopener noreferrer">前往支付</a>
          <button v-else-if="order.payment_status === 'pending' && !order.qr_code" class="shop-primary" type="button" :disabled="busy" @click="retryPayment">{{ busy ? '正在连接支付…' : '重新获取支付链接' }}</button>
          <button class="shop-secondary" type="button" :disabled="busy" @click="refreshOrder">刷新订单</button>
        </div>
        <img v-if="order.payment_status === 'pending' && qrImage" class="cdk-order-qrcode" :src="qrImage" alt="支付二维码" />
        <div v-if="order.cdk" class="shop-cdk-code"><span>兑换码</span><code>{{ order.cdk }}</code><a href="https://cdk.aisale.best/recharge" target="_blank" rel="noopener noreferrer">前往兑换</a></div>
        <p v-if="recoveryKey" class="shop-caption">订单恢复码：<code>{{ recoveryKey }}</code>，请与订单号一起保存。恢复码可用于领取兑换码，请勿分享。</p>
        <button class="shop-secondary" type="button" :disabled="busy" @click="reset">继续选购</button>
      </section>
    </main>
  </ShopLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import { apiClient } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { sanitizeUrl } from '@/utils/url'
import ShopLayout from './ShopLayout.vue'
import { formatStorefrontReward, type StorefrontProduct } from './storefront'

type Order={id:string;product_name:string;price_cents:number;payment_status:string;delivery_status:string;pay_url?:string;qr_code?:string;cdk?:string}
const {t}=useI18n(),route=useRoute(),auth=useAuthStore()
const products=ref<StorefrontProduct[]>([]),selected=ref<StorefrontProduct>(),email=ref(auth.user?.email||''),loading=ref(true),busy=ref(false),error=ref(''),order=ref<Order>(),recoveryKey=ref(''),recoverID=ref(''),recoverKey=ref(''),storeEnabled=ref(false),qrImage=ref('')
const checkoutCard = ref<HTMLElement>()
const checkoutInView = ref(false)
let checkoutObserver: IntersectionObserver | undefined
watch(checkoutCard, element => {
  checkoutObserver?.disconnect()
  checkoutInView.value = false
  if (element && typeof IntersectionObserver !== 'undefined') {
    checkoutObserver = new IntersectionObserver(([entry]) => { checkoutInView.value = entry.isIntersecting }, { threshold: 0.15 })
    checkoutObserver.observe(element)
  }
})
onUnmounted(() => checkoutObserver?.disconnect())
const rewardLoginLink = computed(() => ({ path: '/shop/login', query: { redirect: `/shop?${new URLSearchParams({ product: selected.value?.id || '' })}#shop-product-details` } }))
function goToCheckout() { checkoutCard.value?.scrollIntoView?.({ block: 'start', behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth' }) }
const requestedProductMissing = computed(() => !!route.query.product && !products.value.some(item => item.id === route.query.product))

async function selectRequestedProduct() {
  const requested = products.value.find(item => item.id === route.query.product)
  selected.value = requested || products.value.find(item => item.id === selected.value?.id) || products.value.find(item => item.stock > 0) || products.value[0]
  if (requested) {
    await nextTick()
    checkoutCard.value?.scrollIntoView?.({ block: 'nearest', behavior: 'instant' })
  }
}

let attempt: {fingerprint:string;id:string} | undefined
const paymentURL=computed(()=>sanitizeUrl(order.value?.pay_url||''))
const keyName=(id:string)=>`cdkshop-recovery:${id}`
const errors:Record<string,string>={store_paused:'商城暂停售卖，已有订单仍可查询。',out_of_stock:'该商品已售罄，请刷新库存后重试。',order_access_denied:'无法访问该订单，请核对订单号和恢复码，或登录购买时使用的账号。',order_not_found:'没有找到该订单。',shop_session_expired:'商城登录凭证已过期，请刷新页面后重试。',payment_unavailable:'支付服务暂不可用，订单已保留，可稍后重试。',invalid_order:'请检查商品与接收邮箱。'}
async function request(path:string,init:Parameters<typeof fetch>[1]={}) {
  const response=await fetch(`/api/v1/cdkshop${path}`,{...init,cache:'no-store',headers:{'Content-Type':'application/json',...(init.headers||{})}})
  const body=await response.json()
  if(!response.ok)throw new Error(errors[body.error]||body.error||'请求失败')
  return body
}
async function accessHeaders():Promise<Record<string,string>> {
  const headers:Record<string,string>={}
  if(recoveryKey.value)headers['X-Order-Recovery-Key']=recoveryKey.value
  if(auth.isAuthenticated){const response=await apiClient.post<{token:string}>('/auth/cdkshop-token');headers.Authorization=`Bearer ${response.data.token}`}
  return headers
}
function storeRecovery(id:string,key:string){try{sessionStorage.setItem(keyName(id),key)}catch{/* Recovery key is also displayed for manual saving. */}}
async function load(){
  loading.value=true
  try{const [data,status]=await Promise.all([request('/products'),request('/status')]);products.value=data.items||[];storeEnabled.value=status.enabled===true}
  catch(e){error.value=(e as Error).message}
  finally{loading.value=false}
  await selectRequestedProduct()
}
async function checkout(){
  if(!selected.value||busy.value||!storeEnabled.value||selected.value.stock<1)return
  busy.value=true;error.value=''
  try{
    const fingerprint=JSON.stringify([selected.value.id,email.value.trim().toLowerCase(),auth.user?.id||0])
    if(attempt?.fingerprint!==fingerprint)attempt={fingerprint,id:crypto.randomUUID()}
    const data=await request('/orders',{method:'POST',headers:await accessHeaders(),body:JSON.stringify({product_id:selected.value.id,email:email.value,request_id:attempt.id})})
    order.value=data.order;recoveryKey.value=data.recovery_key||''
    if(recoveryKey.value)storeRecovery(order.value!.id,recoveryKey.value)
    history.replaceState(null,'',`/shop?order=${encodeURIComponent(order.value!.id)}`)
  }catch(e){error.value=(e as Error).message}finally{busy.value=false}
}
async function loadOrder(id:string){
  try{recoveryKey.value=sessionStorage.getItem(keyName(id))||recoveryKey.value}catch{/* Logged-in buyers can still use their account. */}
  order.value=await request(`/orders/${encodeURIComponent(id)}`,{headers:await accessHeaders()})
}
async function refreshOrder(){if(!order.value||busy.value)return;busy.value=true;error.value='';try{await loadOrder(order.value.id)}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
async function recoverOrder(){
  if(busy.value)return
  busy.value=true;error.value=''
  try{
    const data=await request(`/orders/${encodeURIComponent(recoverID.value)}`,{headers:{'X-Order-Recovery-Key':recoverKey.value}})
    order.value=data;recoveryKey.value=recoverKey.value;storeRecovery(data.id,recoverKey.value)
    history.replaceState(null,'',`/shop?order=${encodeURIComponent(data.id)}`)
  }catch(e){error.value=(e as Error).message}finally{busy.value=false}
}
async function retryPayment(){if(!order.value||busy.value)return;busy.value=true;error.value='';try{order.value=await request(`/orders/${encodeURIComponent(order.value.id)}/payment`,{method:'POST',headers:await accessHeaders()})}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
function reset(){order.value=undefined;recoveryKey.value='';attempt=undefined;error.value='';history.replaceState(null,'','/shop');void load()}
watch(() => route.query.product, () => { if (!loading.value && !order.value) void selectRequestedProduct() })
watch(()=>order.value?.qr_code,async value=>{qrImage.value='';if(value){try{const rendered=await QRCode.toDataURL(value,{width:220,margin:2});if(order.value?.qr_code===value)qrImage.value=rendered}catch{error.value='支付二维码无法显示，请刷新订单或联系管理员。'}}})
onMounted(async()=>{const id=String(route.query.order||'');if(id){try{await loadOrder(id)}catch(e){error.value=(e as Error).message;await load()}finally{loading.value=false}}else await load()})
</script>
