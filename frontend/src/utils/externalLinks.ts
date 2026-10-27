import type { PublicSettings } from '@/types'

export const DEFAULT_CARD_SHOP_URL = 'https://catfk.com/shop/DFMROLGS'
export const DEFAULT_EXTERNAL_SHOP_URL = 'https://wzyp.cn/shop/best-ai'

export function safeExternalLink(value: string | undefined, fallback: string): string {
  const link = value === undefined ? fallback : value.trim()
  if (!link) return ''
  try {
    const url = new URL(link)
    return url.protocol === 'https:' && url.hostname && !url.username && !url.password ? url.href : ''
  } catch {
    return ''
  }
}

export function cardShopLink(settings?: Partial<PublicSettings> | null): string {
  return safeExternalLink(settings?.card_shop_url, DEFAULT_CARD_SHOP_URL)
}

export function externalShopLink(settings?: Partial<PublicSettings> | null): string {
  return settings?.external_shop_enabled === true
    ? safeExternalLink(settings.external_shop_url, DEFAULT_EXTERNAL_SHOP_URL)
    : ''
}
