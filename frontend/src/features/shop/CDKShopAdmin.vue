<template>
  <ShopLayout :admin="true">
    <main class="shop-main cdk-admin">
      <div class="shop-section-heading"><div><h2>CDK 商品与库存</h2><p>兑换码只存储加密内容；重复导入会自动跳过。</p></div></div>
      <form class="shop-form cdk-admin-product" @submit.prevent="createProduct">
        <label>商品名称<input v-model.trim="product.name" required maxlength="120" /></label>
        <label>规格<input v-model.trim="product.spec" maxlength="120" /></label>
        <label>售价（元）<input v-model="price" required inputmode="decimal" pattern="[0-9]+(\.[0-9]{1,2})?" /></label>
        <label>赠送 Best AI 余额（美元）<input v-model="reward" inputmode="decimal" pattern="[0-9]+(\.[0-9]{1,2})?" /><small class="shop-caption">填写 5 表示赠送 $5 站点余额；仅登录购买后记录奖励，由管理员核实发放。</small></label>
        <label class="cdk-wide">商品说明<textarea v-model="product.description" rows="2" maxlength="1000" /></label>
        <label class="cdk-wide">兑换说明<textarea v-model="product.instructions" rows="2" maxlength="1000" /></label>
        <label class="shop-checkbox"><input v-model="product.active" type="checkbox" /><span>上架</span></label>
        <button class="shop-primary" :disabled="busy">新增商品</button>
      </form>
      <p v-if="message" class="shop-caption" role="status">{{ message }}</p><p v-if="error" class="shop-error" role="alert">{{ error }}</p>
      <div class="cdk-admin-products">
        <div class="shop-section-heading"><div><h2>商品与库存</h2><p>临时停售可下架，需要移除商品时可删除。历史订单与库存记录会保留。</p></div><span class="shop-caption">共 {{ products.length }} 件商品</span></div>
        <div v-if="!products.length" class="shop-empty">暂无商品，在上方添加第一件商品。</div>
        <section v-for="item in products" :key="item.id" class="cdk-admin-item">
          <div class="cdk-admin-item-info"><div class="cdk-admin-item-heading"><strong>{{ item.name }}</strong><span class="shop-status" :data-status="item.active ? 'PAID' : 'INACTIVE'">{{ item.active ? '已上架' : '已下架' }}</span></div><p>{{ item.spec }} · ¥{{ (item.price_cents/100).toFixed(2) }} · 可售库存 {{ item.stock }}</p><p v-if="item.reward_cents > 0">赠 {{ formatStorefrontReward(item.reward_cents) }} Best AI 余额</p>
            <div class="shop-toolbar cdk-admin-item-actions"><button class="shop-secondary" type="button" :disabled="busy" @click="setProductActive(item)">{{ item.active ? '下架商品' : '重新上架' }}</button><button class="shop-plain shop-delete-product" type="button" :disabled="busy" @click="deleteTarget = item"><Icon name="trash" size="sm" aria-hidden="true" />删除商品</button></div>
          </div>
          <label class="cdk-import">导入兑换码<textarea v-model="codes[item.id]" rows="3" placeholder="每行一个 CDK" /><button class="shop-secondary" type="button" :disabled="busy || !codes[item.id]?.trim()" @click="importCodes(item.id)">导入库存</button></label>
        </section>
      </div>
      <section class="cdk-rewards"><div class="shop-section-heading"><h2>余额奖励待办（美元站点余额）</h2><button class="shop-secondary" type="button" @click="loadRewards">刷新</button></div><div v-if="!rewards.length" class="shop-empty">暂无奖励待办</div><div v-for="task in rewards" :key="task.order_id" class="cdk-reward-row"><div><strong>{{ task.email }} · {{ formatStorefrontReward(task.amount_cents) }} Best AI 余额</strong><p>订单 {{ task.order_id }} · {{ task.status }} · 用户 #{{ task.user_id }}</p></div><button v-if="task.status==='pending'" class="shop-secondary" type="button" @click="issue(task.order_id)">标记已人工发放</button><span v-else>已发放</span></div></section>
    </main>
    <ConfirmDialog :show="!!deleteTarget" title="删除这件商品？" :message="`「${deleteTarget?.name || ''}」将从商城和管理列表移除，不能在此恢复。已有订单、兑换码库存及余额奖励记录仍会保留，已创建的订单仍可付款和领取兑换码。如需暂停售卖，请选择下架。`" :confirm-text="busy ? '正在删除…' : '确认删除'" danger @confirm="deleteProduct" @cancel="cancelDelete" />
  </ShopLayout>
</template>
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { apiClient } from '@/api/client'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import ShopLayout from './ShopLayout.vue'
import { formatStorefrontReward } from './storefront'
type Product={id:string;name:string;spec:string;price_cents:number;reward_cents:number;stock:number;active:boolean}
type Reward={order_id:string;user_id:number;email:string;amount_cents:number;status:string}
const products=ref<Product[]>([]),rewards=ref<Reward[]>([]),codes=reactive<Record<string,string>>({}),price=ref(''),reward=ref('0'),busy=ref(false),message=ref(''),error=ref('')
const product=reactive({name:'',spec:'',description:'',instructions:'',active:true})
const deleteTarget = ref<Product>()
function cents(v:string){if(!/^\d+(\.\d{1,2})?$/.test(v))throw new Error('金额格式错误');const [a,b='']=v.split('.');const n=Number(a)*100+Number(b.padEnd(2,'0'));if(!Number.isSafeInteger(n))throw new Error('金额超出范围');return n}
async function shopToken(){return (await apiClient.post<{token:string}>('/auth/cdkshop-token')).data.token}
async function call(path:string,body?:unknown,method=body===undefined?'GET':'POST'){const token=await shopToken();const res=await fetch(`/api/v1/cdkshop${path}`,{method,headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});const data=await res.json();if(!res.ok){const errors:Record<string,string>={product_not_found:'商品已被删除，请刷新列表。',product_update_failed:'商品状态保存失败，请重试。',product_delete_failed:'删除失败，请重试。'};throw new Error(errors[data.error]||data.error||'请求失败')}return data}
async function setProductActive(item: Product) {
  if (busy.value) return
  busy.value = true; error.value = ''; message.value = ''
  try {
    const data = await call(`/admin/products/${encodeURIComponent(item.id)}`, { active: !item.active }, 'PATCH')
    item.active = data.active
    message.value = data.active ? '商品已重新上架' : '商品已下架，已有订单仍可继续处理'
  } catch (e) { error.value = (e as Error).message } finally { busy.value = false }
}
function cancelDelete() { if (!busy.value) deleteTarget.value = undefined }
async function deleteProduct() {
  if (busy.value || !deleteTarget.value) return
  const item = deleteTarget.value
  busy.value = true; error.value = ''; message.value = ''
  try {
    await call(`/admin/products/${encodeURIComponent(item.id)}`, undefined, 'DELETE')
    products.value = products.value.filter(product => product.id !== item.id)
    delete codes[item.id]
    deleteTarget.value = undefined
    message.value = '商品已删除，历史订单与库存记录已保留'
  } catch (e) { error.value = (e as Error).message; deleteTarget.value = undefined } finally { busy.value = false }
}
async function load(){
  const results=await Promise.allSettled([
    call('/admin/products').then(data=>{products.value=data.items||[]}),
    loadRewards(),
  ])
  const failure=results.find(result=>result.status==='rejected')
  if(failure?.status==='rejected')error.value=(failure.reason as Error).message
}
async function loadRewards(){try{const data=await call('/admin/rewards');rewards.value=data.items||[]}catch(e){error.value=(e as Error).message}}
async function createProduct(){busy.value=true;error.value='';try{await call('/admin/products',{...product,price_cents:cents(price.value),reward_cents:cents(reward.value)});product.name='';message.value='商品已创建';await load()}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
async function importCodes(id:string){busy.value=true;error.value='';try{const lines=(codes[id]||'').split(/\r?\n/).map(v=>v.trim()).filter(Boolean);const data=await call(`/admin/products/${id}/cdks`,{codes:lines});message.value=`导入处理完成：${data.imported} 条`;codes[id]='';await load()}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
async function issue(id:string){try{await call(`/admin/rewards/${id}/issue`,{note:'管理员已线下发放余额'});message.value='已标记为人工发放';await loadRewards()}catch(e){error.value=(e as Error).message}}
onMounted(load)
</script>
