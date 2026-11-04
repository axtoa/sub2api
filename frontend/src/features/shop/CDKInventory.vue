<template>
  <BaseDialog :show="true" :title="`${product.name} · 库存兑换码`" width="wide" panel-class="best-shop cdk-admin-dialog" @close="emit('close')">
    <div class="cdk-inventory">
      <p class="shop-caption">仅管理员可查看完整兑换码。已预留或已发货的码已关联订单，请勿重复发放。</p>
      <div class="shop-toolbar cdk-inventory-toolbar">
        <label>库存状态
          <select :value="status" :disabled="loading" @change="changeStatus">
            <option value="">全部</option><option value="available">可售</option><option value="reserved">已预留</option><option value="delivered">已发货</option>
          </select>
        </label>
        <button class="shop-secondary" type="button" :disabled="loading" @click="load">刷新库存</button>
        <span class="shop-caption">共 {{ total }} 条</span>
      </div>
      <p v-if="error" class="shop-error" role="alert">{{ error }}</p>
      <p v-else-if="loading" class="shop-empty" role="status">正在加载库存…</p>
      <p v-else-if="!items.length" class="shop-empty">当前筛选下暂无兑换码</p>
      <div v-else class="cdk-inventory-list">
        <article v-for="item in items" :key="item.id" class="cdk-inventory-row">
          <p v-if="item.code_error" class="shop-error" role="alert">此条兑换码无法解密，请核对商城加密配置或原始导入记录。</p>
          <div v-else class="cdk-inventory-code"><code>{{ item.code }}</code><ShopCopyButton :text="item.code" label="复制兑换码" /></div>
          <div class="cdk-inventory-meta"><span class="shop-status" :data-status="item.status === 'available' ? 'PAID' : 'INACTIVE'">{{ statusLabel[item.status] || item.status }}</span><span>#{{ item.id }} · 导入于 {{ new Date(item.created_at).toLocaleString() }}</span></div>
          <p v-if="item.order_id" class="cdk-inventory-order">关联订单：{{ item.order_id }}</p>
        </article>
      </div>
      <div class="shop-toolbar cdk-inventory-pagination">
        <button class="shop-secondary" type="button" :disabled="loading || page <= 1" @click="changePage(page - 1)">上一页</button>
        <span class="shop-caption">第 {{ page }} / {{ Math.max(1, Math.ceil(total / pageSize)) }} 页</span>
        <button class="shop-secondary" type="button" :disabled="loading || page * pageSize >= total" @click="changePage(page + 1)">下一页</button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ShopCopyButton from './ShopCopyButton.vue'
import { cdkAdminCall, type CDKProduct } from './cdkAdmin'

type InventoryCode = { id: number; code: string; code_error?: string; status: string; order_id: string; created_at: string }
const props = defineProps<{ product: CDKProduct }>()
const emit = defineEmits<{ close: [] }>()
const items = ref<InventoryCode[]>([]), total = ref(0), page = ref(1), pageSize = ref(50)
const status = ref(''), loading = ref(false), error = ref('')
const statusLabel: Record<string, string> = { available: '可售', reserved: '已预留', delivered: '已发货' }
let controller: AbortController | undefined
async function load() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true; error.value = ''; items.value = []
  try {
    const query = new URLSearchParams({ page: String(page.value), status: status.value })
    const data = await cdkAdminCall<{ items: InventoryCode[]; total: number; page_size: number }>(
      `/admin/products/${encodeURIComponent(props.product.id)}/cdks?${query}`, undefined, 'GET', request.signal,
    )
    if (request.signal.aborted) return
    items.value = data.items; total.value = data.total; pageSize.value = data.page_size
  } catch (e) {
    if (!request.signal.aborted) error.value = (e as Error).message
  } finally { if (!request.signal.aborted) loading.value = false }
}
function changeStatus(event: Event) {
  status.value = (event.target as HTMLSelectElement).value
  page.value = 1
  void load()
}
function changePage(value: number) { page.value = value; void load() }
onMounted(load)
onUnmounted(() => { controller?.abort(); items.value = [] })
</script>
