import { apiClient } from '@/api/client'

export interface CDKProduct {
  id: string
  name: string
  spec: string
  description: string
  instructions: string
  price_cents: number
  reward_cents: number
  stock: number
  active: boolean
  version: number
}

export function amountCents(value: string) {
  if (!/^\d+(\.\d{1,2})?$/.test(value)) throw new Error('金额格式错误')
  const [whole, fraction = ''] = value.split('.')
  const result = Number(whole) * 100 + Number(fraction.padEnd(2, '0'))
  if (!Number.isSafeInteger(result)) throw new Error('金额超出范围')
  return result
}

const errors: Record<string, string> = {
  admin_required: '仅管理员可执行此操作，请重新登录。',
  invalid_product: '请检查商品名称、说明和金额；售价需大于零，金额不得超过 100,000。',
  product_not_found: '商品已被删除，请刷新列表。',
  product_conflict: '商品已被其他操作修改，请取消编辑后重新打开，以最新内容为准。',
  product_update_failed: '商品保存失败，请重试。',
  product_delete_failed: '删除失败，请重试。',
  inventory_unavailable: '库存加载失败，请重试。',
  inventory_decrypt_failed: '兑换码读取失败，请检查商城加密配置。',
}

export async function cdkAdminCall<T = Record<string, unknown>>(path: string, body?: unknown, method = body === undefined ? 'GET' : 'POST', signal?: AbortSignal): Promise<T> {
  const token = (await apiClient.post<{ token: string }>('/auth/cdkshop-token')).data.token
  signal?.throwIfAborted()
  const response = await fetch(`/api/v1/cdkshop${path}`, {
    method,
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    cache: 'no-store',
    signal,
  })
  const data = await response.json()
  if (!response.ok) throw new Error(errors[data.error] || '请求失败，请重试。')
  return data as T
}
