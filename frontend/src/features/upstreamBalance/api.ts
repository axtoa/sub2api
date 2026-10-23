import { apiClient } from '@/api/client'

export interface WalletConfig {
  name: string; provider: string; mode: 'sub2api' | 'manual'; query_account_id: number | null; account_ids: number[]
  currency: string; auto_enabled: boolean; interval_minutes: number; estimate_enabled: boolean
  low_threshold: string | null; management_url: string; note: string
}
export interface AccountOption { id: number; name: string; platform: string; type: string; status: string; wallet_id: string; today_cost?: string; week_cost?: string; month_cost?: string }
export interface Observation { amount: string; currency: string; scope: 'wallet' | 'key_quota' | 'subscription' | 'manual'; at: string }
export interface Wallet extends WalletConfig {
  id: string; version: number; observation: Observation | null; status: string; last_error: string
  last_attempt_at: string | null; next_query_at: string; accounts: AccountOption[]
  today_cost: string; week_cost: string; month_cost: string; since_baseline_cost: string
  estimated_balance: string | null; days_remaining: string | null; stale: boolean; low: boolean; usage_incomplete: boolean
}
export interface WalletInput extends WalletConfig { version: number; initial_balance: string | null }
export interface Snapshot { items: Wallet[]; server_time: string; timezone: string }
export interface Settings { email_enabled: boolean; recipients: string[]; version: number }
export interface Delivery { day: string; recipient: string; status: 'sending' | 'sent' | 'failed' }
export interface SettingsResponse { settings: Settings; deliveries: Delivery[]; timezone: string }
export interface Event { actor_id: number; action: 'save' | 'calibrate'; detail: unknown; at: string }
const base = '/admin/upstream-balances'
export const balanceAPI = {
  async list(signal?: AbortSignal) { return (await apiClient.get<Snapshot>(base, { signal })).data },
  async accounts() { return (await apiClient.get<{ items: AccountOption[] }>(`${base}/accounts`)).data.items },
  async save(id: string, input: WalletInput, key: string) {
    const body = { ...input, request_key: key }
    return (await (id ? apiClient.put<Wallet>(`${base}/${id}`, body) : apiClient.post<Wallet>(base, body))).data
  },
  async calibrate(id: string, amount: string, version: number, key: string) { return (await apiClient.post<Wallet>(`${base}/${id}/calibrate`, { amount, version, request_key: key })).data },
  async query(id: string) { return (await apiClient.post<Wallet>(`${base}/${id}/query`)).data },
  async history(id: string) { return (await apiClient.get<{ items: Event[] }>(`${base}/${id}/history`)).data.items },
  async settings() { return (await apiClient.get<SettingsResponse>(`${base}/settings`)).data },
  async saveSettings(input: Settings) { return (await apiClient.put<Settings>(`${base}/settings`, input)).data },
}
