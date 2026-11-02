import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
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
const mountEditor = () => mount(ShopHomepageAdmin, { global: { stubs: { RouterLink: RouterLinkStub } } })

describe('homepage promotion editor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(shopAPI.homepageContent).mockResolvedValue(content)
    vi.mocked(shopAPI.saveHomepageContent).mockImplementation(async value => value)
  })

  it('edits only the heading and lead and retires the old promotional product configuration', async () => {
    const wrapper = mountEditor(); await flushPromises()
    expect(wrapper.find('.shop-homepage-card-editor').exists()).toBe(false)
    expect(wrapper.find('input[inputmode="decimal"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('shop.homepageCatalogHint')
    expect(wrapper.findComponent(RouterLinkStub).props('to')).toBe('/admin/shop/cdk')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.saveHomepageContent).toHaveBeenCalledWith({ ...content, cards: [] })
    expect(applyHomepageContent).toHaveBeenCalledWith({ ...content, cards: [] }, true)
  })

  it('saves a multiline homepage lead', async () => {
    const wrapper = mountEditor(); await flushPromises()
    await wrapper.get('textarea').setValue('官方订阅/续费，也省下一笔。\n登录完成代充，赠送站点余额。')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.saveHomepageContent).toHaveBeenCalledWith({ ...content, cards: [], lead: '官方订阅/续费，也省下一笔。\n登录完成代充，赠送站点余额。' })
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.find('[role="status"]').exists()).toBe(true)
  })

  it('rejects an empty title before calling the server', async () => {
    const wrapper = mountEditor(); await flushPromises()
    await wrapper.get('input').setValue('  ')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(shopAPI.saveHomepageContent).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('shop.homepageInvalid')
  })

  it('prevents overwriting saved content when loading fails and supports retrying', async () => {
    vi.mocked(shopAPI.homepageContent).mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountEditor(); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('offline')
    expect(wrapper.find('form').exists()).toBe(false)
    expect(shopAPI.saveHomepageContent).not.toHaveBeenCalled()
    await wrapper.get('button').trigger('click'); await flushPromises()
    expect((wrapper.get('input').element as HTMLInputElement).value).toBe(content.title)
  })
})
