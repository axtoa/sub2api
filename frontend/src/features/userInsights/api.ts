import { apiClient } from '@/api/client'

export interface Summary {
  amount: string
  codes: number
  users: number
}

export interface InsightUser {
  id: number
  email: string
  username: string
  status: string
  balance: string
  redeemed_amount: string
  redeem_count: number
  last_redeemed_at: string | null
  usage_30d: string
  calls_30d: number
  total_calls: number
  last_used_at: string | null
}

export interface InsightList {
  items: InsightUser[]
  total: number
  page: number
  page_size: number
  summary: Summary
  filtered_amount: string
}

export interface InsightDetail extends InsightUser {
  first_redeemed_at: string | null
  admin_added_amount: string
  redemptions: Array<{ value: string; used_at: string }>
  models: Array<{ name: string; calls: number; cost: string }>
}

export type Segment = 'all' | 'redeemed' | 'repeat' | 'inactive30'
export type SortKey = 'email' | 'redeemed_amount' | 'redeem_count' | 'last_redeemed_at' | 'balance' | 'usage_30d' | 'calls_30d' | 'total_calls' | 'last_used_at'
export type SortOrder = 'asc' | 'desc'

export const userInsightsAPI = {
  async list(page: number, search: string, segment: Segment, sortBy: SortKey, sortOrder: SortOrder, signal?: AbortSignal) {
    const { data } = await apiClient.get<InsightList>('/admin/user-insights', {
      params: { page, page_size: 20, search, segment, sort_by: sortBy, sort_order: sortOrder }, signal
    })
    return data
  },
  async detail(id: number, signal?: AbortSignal) {
    const { data } = await apiClient.get<InsightDetail>(`/admin/user-insights/${id}`, { signal })
    return data
  }
}
