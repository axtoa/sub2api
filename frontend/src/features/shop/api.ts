import { apiClient } from '@/api/client'
import type { BasePaginationResponse } from '@/types'

export interface ShopProduct {
  id: string; brand: 'ChatGPT' | 'Anthropic'; name: string; spec: string
  description: string; requirements: string; purchase_notice: string
  delivery_hours: number; price_cents: number | null; payment_url: string
  reference_amount: number | null; reference_currency: string; reference_url: string; reference_date: string
  active: boolean; sort: number; version: number
}
export interface ShopEvent { actor_id: number; action: string; note: string; at: string }
export interface ShopOrder {
  id: string; user_id: number; applicant_email: string; target_email: string; contact_email: string
  product: ShopProduct; amount_cents: number; payment_status: string; fulfillment_status: string
  external_reference: string; note: string; delivery_note: string; created_at: string
  paid_at?: string; completed_at?: string; valid_until: string; version: number; events: ShopEvent[]
}
export interface ShopCreateOrder {
  product_id: string; product_version: number; target_email: string; note: string; accepted: boolean; request_key: string
}
export interface ShopAction {
  action: string; version: number; note?: string; external_reference?: string; target_email?: string; valid_until?: string
}
export interface ShopSettings { enabled: boolean }
export const shopMoney = (cents: number) => `¥${(cents / 100).toFixed(2)}`
export const shopDate = (date?: string) => date ? new Date(date).toLocaleString() : '—'
export function checkoutLink(value: string): string {
  try { const url = new URL(value); return url.protocol === 'https:' && !url.username && !url.password ? url.href : '' } catch { return '' }
}
export function priceToCents(value: string): number | null {
  if (!value.trim()) return null
  if (!/^\d+(?:\.\d{1,2})?$/.test(value.trim())) throw new Error('invalid_price')
  const [whole, decimal = ''] = value.trim().split('.')
  const cents = Number(whole) * 100 + Number(decimal.padEnd(2, '0'))
  if (!Number.isSafeInteger(cents) || cents < 1 || cents > 10000000) throw new Error('invalid_price')
  return cents
}
export const paymentStates = ['PENDING', 'PAID', 'CANCELLED', 'REFUNDED'] as const
export const fulfillmentStates = ['WAITING', 'PROCESSING', 'NEEDS_INFO', 'COMPLETED', 'CLOSED'] as const
const statusKeys: Record<string, string> = { PENDING: 'pending', PAID: 'paid', CANCELLED: 'cancelled', REFUNDED: 'refunded', WAITING: 'waiting', PROCESSING: 'processing', NEEDS_INFO: 'needsInfo', COMPLETED: 'completed', CLOSED: 'closed' }
export const statusKey = (status: string) => `shop.${statusKeys[status] || 'stale'}`
const base = (admin = false) => admin ? '/admin/shop' : '/shop'
export const shopAPI = {
  async settings(): Promise<ShopSettings> { return (await apiClient.get<ShopSettings>('/admin/shop/settings')).data },
  async saveSettings(enabled: boolean): Promise<ShopSettings> {
    return (await apiClient.put<ShopSettings>('/admin/shop/settings', { enabled })).data
  },
  async products(admin = false): Promise<ShopProduct[]> {
    return (await apiClient.get<{ items: ShopProduct[] }>(`${base(admin)}/products`)).data.items
  },
  async save(product: ShopProduct): Promise<ShopProduct> {
    return product.id
      ? (await apiClient.put<ShopProduct>(`/admin/shop/products/${product.id}`, product)).data
      : (await apiClient.post<ShopProduct>('/admin/shop/products', product)).data
  },
  async create(body: ShopCreateOrder): Promise<ShopOrder> { return (await apiClient.post<ShopOrder>('/shop/orders', body)).data },
  async orders(admin: boolean, params: Record<string, string | number>): Promise<BasePaginationResponse<ShopOrder>> {
    return (await apiClient.get<BasePaginationResponse<ShopOrder>>(`${base(admin)}/orders`, { params })).data
  },
  async order(id: string, admin: boolean): Promise<ShopOrder> { return (await apiClient.get<ShopOrder>(`${base(admin)}/orders/${id}`)).data },
  async action(id: string, body: ShopAction, admin: boolean): Promise<ShopOrder> {
    return (await apiClient.post<ShopOrder>(`${base(admin)}/orders/${id}/actions`, body)).data
  },
}
