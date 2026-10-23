import { apiClient } from '@/api/client'

export type CheckKind = 'availability' | 'quality' | 'cache'

export interface QualityProfile {
  availability_enabled: boolean
  availability_interval_minutes: number
  quality_enabled: boolean
  quality_interval_minutes: number
  cache_enabled: boolean
  cache_interval_minutes: number
  questions: number
  pass_percent: number
  retry_count: number
  retry_interval_minutes: number
  max_output_tokens: number
  reasoning_effort: '' | 'low' | 'medium' | 'high'
  cache_prefix_kb: number
}

export interface QualitySettings extends QualityProfile {
  auto_enabled: boolean
  daily_requests: number
  daily_tokens: number
  version: number
}

export interface Probe {
  text: string
  model: string
  http_status: number
  complete: boolean
  reason: string
  duration_ms: number
  first_token_ms: number | null
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  usage_reported: boolean
  cache_reported: boolean
}

export interface QualityRun {
  id: string
  target_id: string
  kind: CheckKind
  status: 'queued' | 'running' | 'finished'
  manual: boolean
  verdict: string
  profile_hash: string
  profile?: QualityProfile
  result?: {
    reason: string
    correct: number
    graded: number
    planned: number
    requests: number
    budget_tokens: number
    first_token_ms: number | null
    duration_ms: number
    items: Array<{
      question_id: string
      family: string
      prompt: string
      expected: string
	  explanation: string
      correct: boolean | null
      probe: Probe
    }> | null
	cache_reported: boolean
	cache_input_tokens: number
	cache_read_tokens: number
	cache_prefix?: string
  }
  created_at: string
  started_at: string | null
  finished_at: string | null
}

export interface QualityTarget {
  id: string
  account_id: number
  account_name: string
  platform: string
  account_status: string
  model: string
  enabled: boolean
  overrides: QualityProfile | null
  version: number
  next_availability_at: string
  next_quality_at: string
  next_cache_at: string
  last_availability_at: string | null
  last_quality_at: string | null
  last_cache_at: string | null
  retry_attempt: number
  metrics: {
    requests: number
    successes: number
    errors: number
    first_token_ms: number | null
    billed_input_tokens: number
    billed_cache_tokens: number
    last_success_at: string | null
  }
  recent: QualityRun[]
  latest_quality: QualityRun | null
  latest_cache: QualityRun | null
  pending: QualityRun | null
  account_enabled: boolean
  probe_successes: number
  probe_failures: number
}

export interface MonitoredAccount {
  id: number
  name: string
  platform: string
  status: string
  enabled: boolean
  version: number
  groups: Array<{ id: number; name: string }>
  unsupported: boolean
}

export interface QualitySnapshot {
  settings: QualitySettings
  defaults: QualitySettings
  budget: { day: string; timezone: string; requests: number; tokens: number; unconfirmed_requests: number; limited: boolean; manual_requests: number; manual_tokens: number; manual_unconfirmed: number }
  accounts: MonitoredAccount[]
  targets: QualityTarget[]
  estimated_daily_requests: number
  server_time: string
}

export interface AccountChoice { id: number; name: string; platform: string; status: string }

const base = '/admin/quality-monitor'
export const qualityAPI = {
  async sync(initial = false) {
    const { data } = await apiClient.post<{ created: number }>(`${base}/sync`, { initial })
    return data
  },
  async saveAccount(account: MonitoredAccount, enabled: boolean) {
    await apiClient.put(`${base}/accounts/${account.id}`, { enabled, version: account.version })
  },
  async snapshot(signal?: AbortSignal) {
    const { data } = await apiClient.get<QualitySnapshot>(`${base}/snapshot`, { signal })
    return data
  },
  async saveSettings(settings: QualitySettings) {
    const { data } = await apiClient.put<QualitySettings>(`${base}/settings`, settings)
    return data
  },
  async accounts(search = '', signal?: AbortSignal) {
    const { data } = await apiClient.get<{ items: AccountChoice[] }>(`${base}/accounts`, { params: { search }, signal })
    return data.items
  },
  async add(account_ids: number[], models: string[]) {
    const { data } = await apiClient.post<{ created: number; failures: Array<{ account_id: number; model: string; reason: string }> }>(`${base}/targets`, { account_ids, models })
    return data
  },
  async saveTarget(target: QualityTarget, overrides: QualityProfile | null, enabled = target.enabled) {
    const { data } = await apiClient.put<QualityTarget>(`${base}/targets/${target.id}`, { enabled, overrides, version: target.version })
    return data
  },
  async run(target_ids: string[], kind: CheckKind) {
    const { data } = await apiClient.post<{ queued: number; skipped: number; jobs: QualityRun[] }>(`${base}/runs`, { target_ids, kind })
    return data
  },
  async history(id: string) {
    const { data } = await apiClient.get<{ items: QualityRun[] }>(`${base}/targets/${id}/history`)
    return data.items
  },
  async detail(id: string) {
    const { data } = await apiClient.get<QualityRun>(`${base}/runs/${id}`)
    return data
  },
}

export function profileOf(settings: QualitySettings): QualityProfile {
  return {
    availability_enabled: settings.availability_enabled,
    availability_interval_minutes: settings.availability_interval_minutes,
    quality_enabled: settings.quality_enabled,
    quality_interval_minutes: settings.quality_interval_minutes,
    cache_enabled: settings.cache_enabled,
    cache_interval_minutes: settings.cache_interval_minutes,
    questions: settings.questions, pass_percent: settings.pass_percent,
    retry_count: settings.retry_count, retry_interval_minutes: settings.retry_interval_minutes,
    max_output_tokens: settings.max_output_tokens, reasoning_effort: settings.reasoning_effort,
    cache_prefix_kb: settings.cache_prefix_kb,
  }
}

export function attention(target: QualityTarget): boolean {
  return ['suspected', 'degraded'].includes(target.latest_quality?.verdict ?? '') || target.recent[0]?.verdict === 'unavailable'
}
