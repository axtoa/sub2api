import type { Wallet } from './api'

export const currencies = ['USD', 'CNY', 'EUR', 'HKD', 'GBP', 'JPY', 'CREDIT']
export function money(value: string | null | undefined) {
  if (value == null || !/^-?\d+(\.\d+)?$/.test(value)) return '—'
  const [whole, fraction = ''] = value.split('.')
  const trimmed = fraction.replace(/0+$/, '').padEnd(2, '0')
  return `${whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}.${trimmed}`
}
// Never add key quotas or subscription allowances to actual wallet funds.
export function actualTotals(wallets: Wallet[]) {
  const totals = new Map<string, bigint>()
  for (const w of wallets) {
    const o = w.observation
    if (w.mode !== 'sub2api' || w.status !== 'ok' || w.stale || !o || o.scope !== 'wallet' || o.currency === 'CREDIT') continue
    if (!/^-?\d+(\.\d{1,6})?$/.test(o.amount)) continue
    const [whole, fraction = ''] = o.amount.replace('-', '').split('.')
    const micros = (BigInt(whole) * 1000000n + BigInt(fraction.padEnd(6, '0'))) * (o.amount.startsWith('-') ? -1n : 1n)
    totals.set(o.currency, (totals.get(o.currency) ?? 0n) + micros)
  }
  return [...totals].sort(([a], [b]) => a.localeCompare(b)).map(([currency, micros]) => {
    const absolute = micros < 0 ? -micros : micros
    return { currency, amount: `${micros < 0 ? '-' : ''}${absolute / 1000000n}.${String(absolute % 1000000n).padStart(6, '0')}` }
  })
}
export function safeURL(value: string) { try { const u = new URL(value); return u.protocol === 'https:' && !u.username && !u.password } catch { return false } }
