import type { QualityRun, QualityTarget } from './api'

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
  const recent = [...(target?.recent ?? [])].slice(0, 180).reverse()
  return [...Array<null>(180 - recent.length).fill(null), ...recent]
}

export function firstToken(target?: QualityTarget | null) {
  return target?.recent.find(run => run.result?.first_token_ms != null)?.result?.first_token_ms
    ?? target?.metrics.first_token_ms ?? null
}
