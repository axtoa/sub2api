import type { QualityRun, QualityTarget, QualitySettings } from './api'

export function verdictKey(run?: QualityRun | null) {
  return !run ? 'never' : run.status === 'queued' ? 'queuedStatus' : run.status === 'running' ? 'running' : run.verdict
}

export function runTone(run?: QualityRun | null) {
  if (!run || run.status !== 'finished') return ''
  return ['passed', 'cache_hit'].includes(run.verdict) ? 'good' : ['unavailable', 'degraded'].includes(run.verdict) ? 'bad' : 'warn'
}

export function probeRate(target?: QualityTarget | null) {
  if (!target) return null
  const total = target.probe_successes + target.probe_failures
  return total > 0 ? 100 * target.probe_successes / total : null
}

// Only upstream-reported validation requests count; the warm-up is excluded.
export function cachePercent(run?: QualityRun | null) {
  const result = run?.result
  return run?.status === 'finished' && ['cache_hit', 'not_observed'].includes(run.verdict) && !result?.reason && result?.cache_reported && result.cache_input_tokens > 0 && result.cache_read_tokens >= 0 && result.cache_read_tokens <= result.cache_input_tokens
    ? 100 * result.cache_read_tokens / result.cache_input_tokens : null
}

export function probeSlots(target?: QualityTarget | null): Array<QualityRun | null> {
  const recent = [...(target?.recent ?? [])].slice(0, 48).reverse()
  return [...Array<null>(48 - recent.length).fill(null), ...recent]
}

export function firstToken(target?: QualityTarget | null) {
  return target?.recent.find(run => run.result?.first_token_ms != null)?.result?.first_token_ms
    ?? target?.metrics.first_token_ms ?? null
}

export function reasonKey(run: QualityRun) {
  if (run.status !== 'finished') return `quality.${verdictKey(run)}`
  return run.result?.reason && !['real_traffic','scheduled_test'].includes(run.result.reason) ? `quality.reasons.${run.result.reason}` : `quality.${run.verdict}`
}

export function isStale(target: QualityTarget | null | undefined, settings: QualitySettings, now: number) {
  if (!target?.enabled || !target.account_enabled || !settings.auto_enabled) return false
  const p = target.overrides ?? settings
  const expired = (enabled: boolean, last: string | null | undefined, next: string, interval: number) => {
    if (!enabled) return false
    const reference = Date.parse(last || next)
    return Number.isFinite(reference) && now - reference > Math.max(interval * 2, 30) * 60000
  }
  const availability = target.recent.find(r => ['passed','unavailable'].includes(r.verdict))
  const quality = target.last_valid_quality ?? target.latest_quality
  return expired(p.availability_enabled, availability?.finished_at, target.next_availability_at, p.availability_interval_minutes)
    || expired(p.quality_enabled, quality?.finished_at, target.next_quality_at, p.quality_interval_minutes)
}
