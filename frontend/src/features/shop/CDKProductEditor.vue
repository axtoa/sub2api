<template>
  <BaseDialog :show="true" title="编辑 CDK 商品" width="wide" panel-class="best-shop cdk-admin-dialog" :show-close-button="!saving" :close-on-escape="!saving" @close="emit('close')">
    <form class="shop-form cdk-product-edit" @submit.prevent="save">
      <label>商品名称<input v-model.trim="draft.name" required maxlength="120" :disabled="saving" /></label>
      <label>规格<input v-model.trim="draft.spec" maxlength="120" :disabled="saving" /></label>
      <label>售价（元）<input v-model="price" required inputmode="decimal" pattern="[0-9]+(\.[0-9]{1,2})?" :disabled="saving" /></label>
      <label>赠送 Best AI 余额（美元）<input v-model="reward" required inputmode="decimal" pattern="[0-9]+(\.[0-9]{1,2})?" :disabled="saving" /></label>
      <label class="cdk-wide">商品说明<textarea v-model="draft.description" rows="3" maxlength="1000" :disabled="saving" /></label>
      <label class="cdk-wide">兑换说明<textarea v-model="draft.instructions" rows="3" maxlength="1000" :disabled="saving" /></label>
      <label class="shop-checkbox cdk-wide"><input v-model="draft.active" type="checkbox" :disabled="saving" /><span>上架</span></label>
      <p class="shop-caption cdk-wide">修改仅用于之后创建的订单，已有订单保留原商品名称、价格和奖励金额。</p>
      <p v-if="error" class="shop-error cdk-wide" role="alert">{{ error }}</p>
      <div class="shop-toolbar cdk-wide">
        <button class="shop-primary" type="submit" :disabled="saving">{{ saving ? '正在保存…' : '保存修改' }}</button>
        <button class="shop-secondary" type="button" :disabled="saving" @click="emit('close')">取消</button>
      </div>
    </form>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { amountCents, cdkAdminCall, type CDKProduct } from './cdkAdmin'

const props = defineProps<{ product: CDKProduct }>()
const emit = defineEmits<{ close: []; saved: []; refresh: [] }>()
const draft = reactive({
  name: props.product.name, spec: props.product.spec,
  description: props.product.description, instructions: props.product.instructions,
  active: props.product.active,
})
const price = ref((props.product.price_cents / 100).toFixed(2))
const reward = ref((props.product.reward_cents / 100).toFixed(2))
const saving = ref(false), error = ref('')
// Retain the version the administrator actually reviewed, even if the list refreshes.
const version = props.product.version
async function save() {
  if (saving.value) return
  saving.value = true; error.value = ''
  try {
    await cdkAdminCall(`/admin/products/${encodeURIComponent(props.product.id)}`, {
      ...draft, price_cents: amountCents(price.value), reward_cents: amountCents(reward.value), version,
    }, 'PUT')
    emit('saved')
  } catch (e) {
    error.value = (e as Error).message
    emit('refresh')
  } finally { saving.value = false }
}
</script>
