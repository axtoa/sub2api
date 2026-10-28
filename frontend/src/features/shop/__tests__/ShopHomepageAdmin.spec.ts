import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ShopHomepageAdmin from '../ShopHomepageAdmin.vue'
import { shopAPI } from '../api'

enableAutoUnmount(afterEach)
const applyHomepageContent = vi.fn()
vi.mock('../availability', () => ({ useShopAvailability: () => ({ applyHomepageContent }) }))
vi.mock('../api', () => ({ shopAPI: { homepageContent: vi.fn(), saveHomepageContent: vi.fn() } }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const content = {
  title: '官方代购', lead: '查看方案',
  cards: [{ title: 'Plus', summary: '月度订阅', price_hint: '约 ¥200 / 月', bonus_cents: 500 }],
}

describe('homepage promotion editor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(shopAPI.homepageContent).mockResolvedValue(content)
    vi.mocked(shopAPI.saveHomepageContent).mockImplementation(async value => value)
  })

  it('saves an independently configured bonus in cents', async () => {
    const wrapper = mount(ShopHomepageAdmin, { global: { stubs: { Icon: true } } }); await flushPromises()
    expect(wrapper.text()).toContain('shop.homepageManualBonus')
    expect((wrapper.get('.shop-homepage-card-editor input[inputmode="decimal"]').element as HTMLInputElement).value).toBe('5.00')
    await wrapper.get('.shop-homepage-card-editor input[inputmode="decimal"]').setValue('3.50')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.saveHomepageContent).toHaveBeenCalledWith({ ...content, cards: [{ ...content.cards[0], bonus_cents: 350 }] })
    expect(applyHomepageContent).toHaveBeenCalledWith(expect.objectContaining({ cards: [expect.objectContaining({ bonus_cents: 350 })] }), true)
  })

  it('rejects an invalid bonus before calling the server', async () => {
    const wrapper = mount(ShopHomepageAdmin, { global: { stubs: { Icon: true } } }); await flushPromises()
    await wrapper.get('.shop-homepage-card-editor input[inputmode="decimal"]').setValue('3.555')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.saveHomepageContent).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('shop.homepageInvalid')
  })
})
