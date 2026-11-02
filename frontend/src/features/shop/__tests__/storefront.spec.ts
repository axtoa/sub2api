import { describe, expect, it } from 'vitest'
import { storefrontLoginReturnTo } from '../storefront'

describe('shop login destination', () => {
  it('keeps the selected product and checkout anchor', () => {
    expect(storefrontLoginReturnTo('/shop?product=claude#shop-product-details')).toBe('/shop?product=claude#shop-product-details')
  })
  it.each([undefined, ['/shop'], '/dashboard', '/shop/login', '//other.example/shop', 'https://other.example/shop', 'javascript:alert(1)', '/shop/../dashboard'])('defaults to the public shop for %s', value => {
    expect(storefrontLoginReturnTo(value)).toBe('/shop')
  })
})
