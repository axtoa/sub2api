export const currencies = ['CNY', 'USD', 'EUR', 'HKD', 'GBP', 'JPY']
// Never round stored six-decimal amounts, even for currencies normally shown with two.
export function money(amount: string) {
  const [whole = '0', decimal = ''] = amount.split('.')
  const fraction = decimal.replace(/0+$/, '').padEnd(2, '0')
  return `${whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}.${fraction}`
}
export function localDatetime(iso: string) {
  const date = new Date(iso)
  if (!Number.isFinite(date.getTime())) return ''
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
export function isoDatetime(local: string) {
  const date = new Date(local)
  if (!Number.isFinite(date.getTime())) throw new Error('Invalid date')
  return date.toISOString()
}
export function calendarDays(expiry: string, now: Date, timezone: string) {
  const date = (value: Date) => {
    const parts = new Intl.DateTimeFormat('en', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(value)
    const part = (key: string) => Number(parts.find(p => p.type === key)?.value)
    return Date.UTC(part('year'), part('month') - 1, part('day'))
  }
  return Math.round((date(new Date(expiry)) - date(now)) / 86400000)
}
