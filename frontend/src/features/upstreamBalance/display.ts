import type { Wallet } from './api'

export const currencies = ['USD', 'CNY', 'EUR', 'HKD', 'GBP', 'JPY', 'CREDIT']
export function money(value: string | null | undefined) {
  if (value == null || !/^-?\d+(\.\d+)?$/.test(value)) return '—'
  const [whole, fraction = ''] = value.split('.')
  const trimmed = fraction.replace(/0+$/, '').padEnd(2, '0')
  return `${whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}.${trimmed}`
}
export function money2(value: string | null | undefined) {
  if (value == null || !/^-?\d+(\.\d+)?$/.test(value)) return '—'
  const negative = value.startsWith('-')
  const [whole, fraction = ''] = (negative ? value.slice(1) : value).split('.')
  let cents = BigInt(whole) * 100n + BigInt(fraction.slice(0, 2).padEnd(2, '0'))
  if (fraction.length > 2 && fraction[2]! >= '5') cents++
  const integer = String(cents / 100n).replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return `${negative && cents !== 0n ? '-' : ''}${integer}.${String(cents % 100n).padStart(2, '0')}`
}
// Never add key quotas or subscription allowances to actual wallet funds.
export function actualTotals(wallets: Wallet[]) {
  const totals = new Map<string, bigint>()
  for (const w of wallets) {
    const o = w.observation
    if (w.mode === 'manual' || w.status !== 'ok' || w.stale || !o || o.scope !== 'wallet' || o.currency === 'CREDIT') continue
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

export function walletURL(wallet: Wallet) {
  const source = wallet.query_account_id != null
    ? wallet.accounts.find(account => account.id === wallet.query_account_id)
    : wallet.accounts.find(account => account.upstream_url)
  try {
    const url = new URL(source?.upstream_url ?? '')
    if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password) return ''
    url.search = ''; url.hash = ''
    return url.href
  } catch { return '' }
}
