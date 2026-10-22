import { describe, expect, it } from 'vitest'
import { checkoutLink, priceToCents, shopMoney } from '../api'
describe('shop money and external links', () => {
  it('converts decimal prices into exact integer cents', () => {
    expect(priceToCents('123')).toBe(12300)
    expect(priceToCents('0.29')).toBe(29)
    expect(priceToCents(' 12.3 ')).toBe(1230)
    expect(priceToCents('100000.00')).toBe(10000000)
    expect(priceToCents('')).toBeNull()
    expect(shopMoney(12300)).toBe('¥123.00')
  })
  it.each(['0', '-1', 'NaN', 'Infinity', '1e3', '1.001', '123abc', '100000.01', '0x10', '1,000'])('rejects invalid price %s', value => {
    expect(() => priceToCents(value)).toThrow()
  })
  it.each(['javascript:alert(1)', 'http://example.com', 'https://user:password@example.com', '//example.com', 'data:text/html,test', ''])('rejects unsafe checkout %s', value => {
    expect(checkoutLink(value)).toBe('')
  })
  it('keeps valid HTTPS checkout links', () => { expect(checkoutLink('https://example.com/product?id=1')).toBe('https://example.com/product?id=1') })
})
