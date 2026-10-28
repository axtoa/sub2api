import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { reactive, ref } from 'vue'
import type { PublicSettings } from '@/types'
import ShopOptionsPage from '../ShopOptionsPage.vue'

enableAutoUnmount(afterEach)
const app = reactive({
  cachedPublicSettings: null as Partial<PublicSettings> | null,
  fetchPublicSettings: vi.fn(async () => app.cachedPublicSettings),
})
const officialEnabled = ref(true)
vi.mock('@/stores/app', () => ({ useAppStore: () => app }))
vi.mock('../availability', () => ({ useShopAvailability: () => ({ enabled: officialEnabled }) }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const mountOptions = () => mount(ShopOptionsPage, {
  global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true, RouterLink: RouterLinkStub } },
})

describe('shop channel selection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    officialEnabled.value = true
    app.cachedPublicSettings = { external_shop_enabled: true, external_shop_url: 'https://example.com/shop' }
  })

  it('shows both enabled channels in the current tab', async () => {
    const wrapper = mountOptions(); await flushPromises()
    expect(wrapper.findAll('.shop-option')).toHaveLength(2)
    expect(wrapper.get('.shop-option').findComponent(RouterLinkStub).props('to')).toBe('/shop')
    expect(wrapper.findAll('.shop-option')[1].attributes('href')).toBe('https://example.com/shop')
    expect(wrapper.findAll('.shop-option')[1].attributes('target')).toBeUndefined()
  })

  it('keeps the external channel available when the on-site channel is closed', async () => {
    officialEnabled.value = false
    const wrapper = mountOptions(); await flushPromises()
    expect(wrapper.findAll('.shop-option')).toHaveLength(1)
    expect(wrapper.get('a.shop-option').attributes('href')).toBe('https://example.com/shop')
    expect(wrapper.text()).not.toContain('shop.noChannels')
  })

  it('shows no dead links when both channels are closed', async () => {
    officialEnabled.value = false
    app.cachedPublicSettings = { external_shop_enabled: false, external_shop_url: 'https://example.com/shop' }
    const wrapper = mountOptions(); await flushPromises()
    expect(wrapper.find('.shop-option').exists()).toBe(false)
    expect(wrapper.text()).toContain('shop.noChannels')
  })

  it('explains manual verification only when a bonus is advertised', async () => {
    app.cachedPublicSettings = {
      external_shop_enabled: true,
      external_shop_url: 'https://example.com/shop',
      shop_homepage_enabled: true,
      shop_homepage_content: { title: '代购', lead: '选择', cards: [{ title: 'Plus', summary: '订阅', price_hint: '', bonus_cents: 500 }] },
    }
    const wrapper = mountOptions(); await flushPromises()
    expect(wrapper.text()).toContain('shop.homepageManualBonus')
    app.cachedPublicSettings.shop_homepage_enabled = false
    await flushPromises()
    expect(wrapper.text()).not.toContain('shop.homepageManualBonus')
  })
})
