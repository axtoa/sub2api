import { apiClient } from '@/api/client'

export type Kind = 'income' | 'expense' | 'gift'
export interface Entry {
  id: string; kind: Kind; amount: string; currency: string; category: string; occurred_on: string
  counterparty: string; note: string; resource_id: string | null; voided: boolean; version: number; created_at: string
}
export interface Resource {
  id: string; name: string; kind: 'domain' | 'server' | 'other'; provider: string; expires_at: string
  renewal_amount: string | null; currency: string; management_url: string; note: string; active: boolean; version: number
}
export type ResourceInput = Omit<Resource, 'id'> & { request_key?: string }
export type EntryInput = Omit<Entry, 'id' | 'voided' | 'created_at'> & { new_resource?: ResourceInput }
export interface Settings { email_enabled: boolean; recipients: string[]; reminder_days: number[]; version: number }
export interface Delivery { day: string; recipient: string; status: 'sent' | 'sending' | 'failed' }
export interface SettingsResponse { settings: Settings; deliveries: Delivery[]; timezone: string }
export interface Total { currency: string; income: string; expense: string; gift: string; net: string }
export interface EntryList { items: Entry[]; total: number; totals: Total[]; categories: string[]; today: string; page_size: number }
export interface Filters { month: string; q: string; kind: string; currency: string; resource_id: string; status: string; page: number }
export interface Event { actor_id: number; action: string; before: unknown; after: unknown; at: string }
export interface RenewalInput { version: number; expires_at: string; record_expense: boolean; amount: string; currency: string; occurred_on: string; note: string }
const base = '/admin/ledger'
export const ledgerAPI = {
  async entries(params: Filters, signal?: AbortSignal) { return (await apiClient.get<EntryList>(`${base}/entries`, { params, signal })).data },
  async resources(signal?: AbortSignal) { return (await apiClient.get<{ items: Resource[]; timezone: string; server_time: string }>(`${base}/resources`, { signal })).data },
  async settings() { return (await apiClient.get<SettingsResponse>(`${base}/settings`)).data },
  async saveSettings(input: Settings) { return (await apiClient.put<Settings>(`${base}/settings`, input)).data },
  async saveEntry(id: string, input: EntryInput, key: string) {
    const body = { ...input, request_key: key }
    return (await (id ? apiClient.put<Entry>(`${base}/entries/${id}`, body) : apiClient.post<Entry>(`${base}/entries`, body))).data
  },
  async saveResource(id: string, input: ResourceInput, key: string) {
    const body = { ...input, request_key: key }
    return (await (id ? apiClient.put<Resource>(`${base}/resources/${id}`, body) : apiClient.post<Resource>(`${base}/resources`, body))).data
  },
  async renew(id: string, input: RenewalInput, key: string) { return (await apiClient.post(`${base}/resources/${id}/renew`, { ...input, request_key: key })).data },
  async voidEntry(entry: Entry, key: string) { return (await apiClient.post(`${base}/entries/${entry.id}/void`, { version: entry.version, request_key: key })).data },
  async history(id: string) { return (await apiClient.get<{ items: Event[] }>(`${base}/history/${id}`)).data.items },
  async export(params: Filters) { return (await apiClient.get<Blob>(`${base}/export`, { params, responseType: 'blob' })).data },
}
