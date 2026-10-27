import { describe, expect, it } from 'vitest'
import { cardShopLink, externalShopLink } from '../externalLinks'

describe('external purchase links', () => {
  it('uses one configured card-shop URL across callers and allows hiding it', () => {
    expect(cardShopLink()).toBe('https://catfk.com/shop/DFMROLGS')
    expect(cardShopLink({ card_shop_url: 'https://example.com/recharge' })).toBe('https://example.com/recharge')
    expect(cardShopLink({ card_shop_url: '' })).toBe('')
    expect(cardShopLink({ card_shop_url: 'javascript:alert(1)' })).toBe('')
  })

  it('shows the external marketplace only when its own switch and URL are valid', () => {
    expect(externalShopLink({ official_shop_enabled: false, external_shop_enabled: true })).toBe('https://wzyp.cn/shop/best-ai')
    expect(externalShopLink({ official_shop_enabled: true, external_shop_enabled: false })).toBe('')
    expect(externalShopLink({ external_shop_enabled: true, external_shop_url: '' })).toBe('')
    expect(externalShopLink({ external_shop_enabled: true, external_shop_url: 'https://user:pass@example.com' })).toBe('')
  })
})
