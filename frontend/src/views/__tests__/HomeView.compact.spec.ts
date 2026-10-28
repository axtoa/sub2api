import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount, RouterLinkStub } from '@vue/test-utils'

import HomeView from '../HomeView.vue'
import { shopAPI } from '@/features/shop/api'
enableAutoUnmount(afterEach)

const { appStore, authStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Fallback site',
    siteLogo: '',
    docUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  },
  authStore: {
    isAuthenticated: false,
    isAdmin: false,
    user: null as { email?: string } | null,
    checkAuth: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('vue-router', async original => ({
  ...await original<typeof import('vue-router')>(),
  useRouter: () => ({ currentRoute: { value: { path: '/home' } }, replace: vi.fn() }),
}))
vi.mock('@/features/shop/api', () => ({
  shopAPI: { products: vi.fn().mockResolvedValue([]) },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountHome(settings: Record<string, unknown> = {}) {
  appStore.cachedPublicSettings = {
    site_name: 'Test site',
    site_subtitle: 'Test subtitle',
    ...settings,
  }

  return mount(HomeView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        LocaleSwitcher: { template: '<div data-testid="locale-switcher" />' },
        Icon: { template: '<span data-testid="icon" />' },
      },
    },
  })
}

function compactDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper.get('[data-testid="compact-home"]').findComponent(RouterLinkStub).props('to')
}

function modelPlazaDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper
    .findAllComponents(RouterLinkStub)
    .find((link) => link.props('to') === '/model-plaza')
    ?.props('to')
}

describe('HomeView compact mode', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    authStore.user = null
    authStore.checkAuth.mockClear()
    appStore.fetchPublicSettings.mockClear()
    vi.mocked(shopAPI.products).mockClear()
    localStorage.clear()
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })

  it('renders custom HTML ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: '<section id="custom-home">Custom home</section>',
    })

    expect(wrapper.get('#custom-home').text()).toBe('Custom home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('renders custom URL content ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: ' https://example.com/home ',
    })

    expect(wrapper.get('iframe').attributes('src')).toBe('https://example.com/home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('treats whitespace-only custom content as empty and selects compact mode', () => {
    const wrapper = mountHome({ compact_home_enabled: true, home_content: ' \n\t ' })

    expect(wrapper.get('[data-testid="compact-home"]').text()).toContain('Test site')
  })

  it.each([undefined, false])('selects the default home when compact mode is %s', (enabled) => {
    const settings = enabled === undefined ? {} : { compact_home_enabled: enabled }
    const wrapper = mountHome(settings)

    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
    expect(wrapper.find('.hero-visual').exists()).toBe(true)
    expect(wrapper.text()).toContain('ChatGPT')
    expect(wrapper.text()).toContain('Anthropic')
    expect(wrapper.text()).not.toContain('shop.homepageTitle')
    expect(wrapper.text()).not.toContain('Gemini')
    expect(wrapper.text()).not.toContain('Antigravity')
    expect(wrapper.text()).not.toContain('subscriptionToApi')
  })

  it('keeps the default home call to action on the existing login and dashboard routes', () => {
    const anonymous = mountHome()
    expect(anonymous.findAllComponents(RouterLinkStub).some((link) => link.props('to') === '/login')).toBe(true)

    authStore.isAuthenticated = true
    const member = mountHome()
    expect(member.findAllComponents(RouterLinkStub).some((link) => link.props('to') === '/dashboard')).toBe(true)
  })
  it('shows invoice availability immediately after usage-based billing', () => {
    const wrapper = mountHome()
    expect(wrapper.get('.home-proof').findAll('span').map(item => item.text())).toEqual([
      'ChatGPT', 'Anthropic', 'home.simpleBilling', 'home.invoiceAvailable',
    ])
  })
  it('hides the promotion when both channels are closed', () => {
    const wrapper = mountHome({ official_shop_enabled: false, shop_homepage_enabled: true })
    expect(wrapper.find('.home-shop').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('shop.homepageTitle')
    expect(shopAPI.products).not.toHaveBeenCalled()
    expect(wrapper.findAllComponents(RouterLinkStub).some(link => link.props('to') === '/login')).toBe(true)
    wrapper.unmount()
  })

  it('keeps the homepage promotion closed until its own switch is enabled', () => {
    const wrapper = mountHome({ official_shop_enabled: true })
    expect(wrapper.find('.home-shop').exists()).toBe(false)
  })

  it('shows the same promotion with either channel and hides it with the independent switch', () => {
    const externalOnly = mountHome({ official_shop_enabled: false, external_shop_enabled: true, external_shop_url: 'https://example.com/shop', shop_homepage_enabled: true })
    expect(externalOnly.find('.home-shop').exists()).toBe(true)
    expect(externalOnly.get('.home-shop-cta').findComponent(RouterLinkStub).props('to')).toBe('/shop/options')
    externalOnly.unmount()

    const hidden = mountHome({ official_shop_enabled: true, shop_homepage_enabled: false })
    expect(hidden.find('.home-shop').exists()).toBe(false)
    expect(shopAPI.products).not.toHaveBeenCalled()
  })

  it('links unauthenticated visitors to login', () => {
    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/login')
  })

  it('links authenticated users to their dashboard', () => {
    authStore.isAuthenticated = true

    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/dashboard')
  })

  it('links administrators to the admin dashboard', () => {
    authStore.isAuthenticated = true
    authStore.isAdmin = true

    const wrapper = mountHome({ compact_home_enabled: true })
    expect(compactDestination(wrapper)).toBe('/admin/dashboard')
    expect(authStore.checkAuth).toHaveBeenCalledOnce()
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
  })

  it('shows the model plaza link to anonymous visitors when public access is enabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link from anonymous visitors when sign-in is required', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it('shows the model plaza link to authenticated visitors when sign-in is required', () => {
    authStore.isAuthenticated = true

    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('shows the model plaza link in the default home header', () => {
    const wrapper = mountHome({
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link when the feature is disabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: false,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })
})
