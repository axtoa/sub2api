import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import AppSidebar from '../AppSidebar.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import zhShop from '@/i18n/locales/zh/shop'
import enShop from '@/i18n/locales/en/shop'

const state = vi.hoisted(() => ({
  auth: { isAdmin: false, isSimpleMode: false },
  app: {
    sidebarCollapsed: false, mobileOpen: false, backendModeEnabled: false,
    sidebarScrollTop: 0, siteName: 'Best AI', siteLogo: '', siteVersion: '1.0.0',
    publicSettingsLoaded: true, cachedPublicSettings: { custom_menu_items: [], official_shop_enabled: true },
    toggleSidebar: vi.fn(), setMobileOpen: vi.fn(),
  },
  admin: { customMenuItems: [], fetch: vi.fn() },
  toggleTheme: vi.fn(),
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => state.auth,
  useAppStore: () => state.app,
  useAdminSettingsStore: () => state.admin,
  useOnboardingStore: () => ({ isCurrentStep: () => false }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => state.app }))
vi.mock('@/composables/useBatchImageAccess', async () => {
  const { ref } = await import('vue')
  return { useBatchImageAccess: () => ({ canUseCreativeStudio: ref(false), refreshBatchImageAccess: vi.fn() }) }
})
vi.mock('@/composables/useTheme', async () => {
  const { ref } = await import('vue')
  return { useTheme: () => ({ isDark: ref(false), toggleTheme: state.toggleTheme }) }
})
vi.mock('vue-i18n', async original => ({
  ...await original<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

const wrappers: VueWrapper[] = []
async function mountSidebar() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
  })
  await router.push(state.auth.isAdmin ? '/admin/dashboard' : '/dashboard')
  const wrapper = mount(AppSidebar, {
    global: { plugins: [router], stubs: { VersionBadge: true, Icon: true } },
  })
  wrappers.push(wrapper)
  await flushPromises()
  return { wrapper, router }
}

beforeEach(() => {
  vi.clearAllMocks()
  state.auth.isAdmin = false
  state.auth.isSimpleMode = false
  state.app.sidebarCollapsed = false
  state.app.mobileOpen = false
  state.app.backendModeEnabled = false
  state.app.cachedPublicSettings.official_shop_enabled = true
})
afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount())
  vi.useRealTimers()
})

describe('AppSidebar official product purchasing', () => {
  it.each([false, true])('places upstream balances below the ledger for admins (simple: %s)', async simple => {
    state.auth.isAdmin = true; state.auth.isSimpleMode = simple
    const { wrapper, router } = await mountSidebar()
    const paths = wrapper.findAll('nav a').map(a => a.attributes('href'))
    expect(paths.indexOf('/admin/upstream-balances')).toBe(paths.indexOf('/admin/ledger') + 1)
    const link = wrapper.get('nav a[href="/admin/upstream-balances"]')
    expect(link.attributes('target')).toBeUndefined()
    await link.trigger('click'); await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/upstream-balances')
  })
  it('hides upstream wallet administration from ordinary users', async () => {
    const { wrapper } = await mountSidebar()
    expect(wrapper.find('a[href="/admin/upstream-balances"]').exists()).toBe(false)
  })
  it.each([false, true])('opens the admin ledger below shop in the existing console (simple: %s)', async simple => {
    state.auth.isAdmin = true
    state.auth.isSimpleMode = simple
    const { wrapper, router } = await mountSidebar()
    const ledger = wrapper.get('nav a[href="/admin/ledger"]')
    expect(ledger.element.previousElementSibling?.getAttribute('href')).toBe('/admin/shop')
    expect(ledger.attributes('target')).toBeUndefined()
    await ledger.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/ledger')
  })
  it('does not show the ledger to ordinary users', async () => {
    const { wrapper } = await mountSidebar()
    expect(wrapper.find('a[href="/admin/ledger"]').exists()).toBe(false)
  })
  it.each([false, true])('hides the customer entry when closed without hiding admin management (admin: %s)', async isAdmin => {
    state.auth.isAdmin = isAdmin
    state.app.cachedPublicSettings.official_shop_enabled = false
    const { wrapper } = await mountSidebar()
    expect(wrapper.find('.sidebar-shop-entry').exists()).toBe(false)
    expect(wrapper.find('a[href="/admin/shop"]').exists()).toBe(isAdmin)
    expect(wrapper.find('a[href="/redeem"]').exists()).toBe(true)
    expect(wrapper.find('a[href="https://catfk.com/shop/DFMROLGS"]').exists()).toBe(true)
  })
  it('uses short menu names while keeping the full page headings', () => {
    expect(zhShop.shop.entry).toBe('官方代购')
    expect(zhShop.shop.adminEntry).toBe('代购管理')
    expect(zhShop.shop.title).toBe('官方产品代购')
    expect(zhShop.shop.homepageTitle).toBe(zhShop.shop.title)
    expect(zhShop.shop.adminTitle).toBe('官方产品代购管理')
    expect(enShop.shop.entry).toBe('Official Products')
  })

  it.each([false, true])('pins one customer entry above the theme divider (admin: %s)', async isAdmin => {
    state.auth.isAdmin = isAdmin
    const { wrapper } = await mountSidebar()
    const shop = wrapper.get('.sidebar-shop-entry')
    expect(wrapper.findAll('a[href="/shop"]')).toHaveLength(1)
    expect(wrapper.find('nav a[href="/shop"]').exists()).toBe(false)
    expect(shop.element.parentElement?.previousElementSibling?.tagName).toBe('NAV')
    const controls = shop.element.nextElementSibling
    expect(controls?.classList.contains('border-t')).toBe(true)
    expect(controls?.querySelector('button[aria-label="nav.darkMode"]')).not.toBeNull()
    const link = shop.get('a')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
    expect(link.attributes('title')).toContain('shop.openInNewTab')
    expect(link.find('icon-stub[name="externalLink"]').exists()).toBe(false)
    const icon = link.getComponent(PlatformIcon)
    expect(icon.props()).toMatchObject({ platform: 'openai', size: 'md' })
    expect(icon.get('svg').attributes('fill')).toBe('currentColor')
    expect(icon.get('svg').classes()).toEqual(expect.arrayContaining(['w-4', 'h-4']))
    expect(icon.get('svg').element.parentElement?.className).toContain('h-5 w-5')
  })

  it.each([false, true])('keeps admin management below settings and opens it in a new tab (simple: %s)', async simple => {
    state.auth.isAdmin = true
    state.auth.isSimpleMode = simple
    const { wrapper, router } = await mountSidebar()
    const management = wrapper.get('nav a[href="/admin/shop"]')
    expect(management.element.previousElementSibling?.getAttribute('href')).toBe('/admin/settings')
    expect(management.attributes('target')).toBe('_blank')
    expect(management.attributes('rel')).toBe('noopener noreferrer')
    expect(management.find('icon-stub[name="externalLink"]').exists()).toBe(false)
    const icon = management.getComponent(PlatformIcon)
    expect(icon.props()).toMatchObject({ platform: 'openai', size: 'md' })
    expect(icon.get('svg').attributes('fill')).toBe('currentColor')
    expect(icon.get('svg').element.parentElement?.className).toContain('h-5 w-5')
    await management.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/dashboard')
    expect(wrapper.find('.sidebar-shop-entry').exists()).toBe(true)
  })

  it('preserves the current console on shop clicks and keeps other console links in the same tab', async () => {
    const { wrapper, router } = await mountSidebar()
    await wrapper.get('.sidebar-shop-entry a').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(wrapper.get('a[href="/profile"]').attributes('target')).toBeUndefined()
    expect(wrapper.get('a[href="/profile"]').findComponent(PlatformIcon).exists()).toBe(false)
    await wrapper.get('a[href="/profile"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/profile')
    await wrapper.get('button[aria-label="nav.darkMode"]').trigger('click')
    expect(state.toggleTheme).toHaveBeenCalledOnce()
  })

  it('retains an accessible shop link in collapsed mode', async () => {
    state.app.sidebarCollapsed = true
    const { wrapper } = await mountSidebar()
    const link = wrapper.get('.sidebar-shop-entry a')
    expect(link.classes()).toContain('sidebar-link-collapsed')
    expect(link.attributes('aria-label')).toContain('shop.entry')
    expect(link.get('.sidebar-label').attributes('aria-hidden')).toBe('true')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.getComponent(PlatformIcon).props('platform')).toBe('openai')
    expect(link.getComponent(PlatformIcon).props('size')).toBe('md')
  })

  it('closes the mobile menu without replacing the console route', async () => {
    vi.useFakeTimers()
    state.app.mobileOpen = true
    const { wrapper, router } = await mountSidebar()
    await wrapper.get('.sidebar-shop-entry a').trigger('click')
    await vi.advanceTimersByTimeAsync(150)
    expect(state.app.setMobileOpen).toHaveBeenCalledWith(false)
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('does not expose a customer entry when regular-user routes are disabled', async () => {
    state.app.backendModeEnabled = true
    const { wrapper } = await mountSidebar()
    expect(wrapper.find('.sidebar-shop-entry').exists()).toBe(false)
    expect(wrapper.find('a[href="https://catfk.com/shop/DFMROLGS"]').exists()).toBe(false)
  })
})

describe('AppSidebar external balance top up', () => {
  const topUpURL = 'https://catfk.com/shop/DFMROLGS'

  it.each([false, true])('places the external dollar link immediately below redeem (admin: %s)', async isAdmin => {
    state.auth.isAdmin = isAdmin
    const { wrapper, router } = await mountSidebar()
    const link = wrapper.get(`a[href="${topUpURL}"]`)
    expect(wrapper.findAll(`a[href="${topUpURL}"]`)).toHaveLength(1)
    expect(link.element.previousElementSibling?.getAttribute('href')).toBe('/redeem')
    expect(link.text()).toBe('nav.balanceTopUp')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
    expect(link.attributes('to')).toBeUndefined()
    expect(link.find('icon-stub[name="dollar"]').exists()).toBe(true)
    expect(link.classes()).not.toContain('sidebar-link-active')
    const currentPath = router.currentRoute.value.path
    await link.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe(currentPath)
    await wrapper.get('a[href="/redeem"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/redeem')
  })

  it.each([false, true])('follows redeem visibility in simple mode (admin: %s)', async isAdmin => {
    state.auth.isAdmin = isAdmin
    state.auth.isSimpleMode = true
    const { wrapper } = await mountSidebar()
    expect(wrapper.find('a[href="/redeem"]').exists()).toBe(false)
    expect(wrapper.find(`a[href="${topUpURL}"]`).exists()).toBe(false)
  })

  it('keeps the collapsed link labelled and preserves the external URL', async () => {
    state.app.sidebarCollapsed = true
    const { wrapper } = await mountSidebar()
    const link = wrapper.get(`a[href="${topUpURL}"]`)
    expect(link.classes()).toContain('sidebar-link-collapsed')
    expect(link.attributes('title')).toBe('nav.balanceTopUp')
    expect(link.get('.sidebar-label').attributes('aria-hidden')).toBe('true')
  })

  it('closes the mobile menu without navigating the console', async () => {
    vi.useFakeTimers()
    state.app.mobileOpen = true
    const { wrapper, router } = await mountSidebar()
    await wrapper.get(`a[href="${topUpURL}"]`).trigger('click')
    await vi.advanceTimersByTimeAsync(150)
    expect(state.app.setMobileOpen).toHaveBeenCalledWith(false)
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })
})
