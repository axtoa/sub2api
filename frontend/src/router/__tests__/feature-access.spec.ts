import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

type NavigationGuard = (
  to: Record<string, any>,
  from: Record<string, any>,
  next: ReturnType<typeof vi.fn>
) => Promise<void>

const routerHarness = vi.hoisted(() => ({
  guard: null as NavigationGuard | null,
  routes: [] as { path: string; meta?: Record<string, unknown> }[],
}))

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: true,
  isAdmin: false,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  publicSettingsLoaded: false,
  cachedPublicSettings: null as null | {
    payment_enabled?: boolean
    risk_control_enabled?: boolean
    creative_workbench_enabled?: boolean
    custom_menu_items?: []
  },
  fetchPublicSettings: vi.fn(),
}))

vi.mock('vue-router', () => ({
  createWebHistory: vi.fn(() => ({})),
  createRouter: vi.fn((options: { routes: typeof routerHarness.routes }) => {
    routerHarness.routes = options.routes
    return {
    beforeEach: vi.fn((guard: NavigationGuard) => {
      routerHarness.guard = guard
    }),
    afterEach: vi.fn(),
    onError: vi.fn(),
    }
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))

vi.mock('@/stores/adminCompliance', () => ({
  useAdminComplianceStore: () => ({
    initialized: true,
    fetchStatus: vi.fn(),
    requireAcknowledgement: vi.fn(),
  }),
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function runGuard(meta: Record<string, unknown>, path: string) {
  if (!routerHarness.guard) {
    throw new Error('router guard was not registered')
  }

  const next = vi.fn()
  const navigation = routerHarness.guard(
    {
      path,
      fullPath: path,
      name: 'FeatureRoute',
      params: {},
      query: {},
      meta: { requiresAuth: true, ...meta },
    },
    {},
    next
  )
  return { navigation, next }
}

describe('feature route guard', () => {
  beforeAll(async () => {
    const { initI18n } = await import('@/i18n')
    await initI18n()
    await import('@/router')
  })

  beforeEach(() => {
    sessionStorage.clear()
    authStore.isAuthenticated = true
    authStore.isAdmin = false
    authStore.isSimpleMode = false
    appStore.backendModeEnabled = false
    appStore.publicSettingsLoaded = false
    appStore.cachedPublicSettings = null
    appStore.fetchPublicSettings.mockReset()
  })

  it('requires login for the shop catalogue and all orders, with administration restricted to admins', () => {
    expect(routerHarness.routes.find(route => route.path === '/shop')?.meta?.requiresAuth).toBe(true)
    expect(routerHarness.routes.find(route => route.path === '/shop/orders/:id?')?.meta?.requiresAuth).toBe(true)
    for (const path of ['/admin/shop', '/admin/shop/products', '/admin/shop/orders/:id']) {
      expect(routerHarness.routes.find(route => route.path === path)?.meta).toMatchObject({ requiresAuth: true, requiresAdmin: true })
    }
  })

  it.each(['/shop', '/shop/orders/:id?', '/admin/shop', '/admin/shop/products', '/admin/shop/orders/:id', '/admin/quality-monitor', '/admin/ledger', '/admin/upstream-balances'])('requires login for %s', async path => {
    authStore.isAuthenticated = false
    const meta = routerHarness.routes.find(route => route.path === path)!.meta!
    const { navigation, next } = runGuard(meta, path)
    await navigation
    expect(next).toHaveBeenCalledWith({ path: '/login', query: { redirect: path } })
  })

  it.each(['/admin/shop', '/admin/shop/products', '/admin/shop/orders/:id', '/admin/quality-monitor', '/admin/ledger', '/admin/upstream-balances'])('denies ordinary users access to %s', async path => {
    const meta = routerHarness.routes.find(route => route.path === path)!.meta!
    const { navigation, next } = runGuard(meta, path)
    await navigation
    expect(next).toHaveBeenCalledWith('/dashboard')
  })

  it('keeps login reachable for non-admin shop visitors in backend-only mode', async () => {
    authStore.isAuthenticated = false
    const entry = runGuard({ requiresAuth: true }, '/shop')
    await entry.navigation
    expect(entry.next).toHaveBeenCalledWith({ path: '/login', query: { redirect: '/shop' } })
    authStore.isAuthenticated = true
    appStore.backendModeEnabled = true
    const blocked = runGuard({ requiresAuth: true }, '/shop')
    await blocked.navigation
    expect(blocked.next).toHaveBeenCalledWith('/login')
    const login = runGuard({ requiresAuth: false }, '/login')
    await login.navigation
    expect(login.next).toHaveBeenCalledOnce()
    expect(login.next).toHaveBeenCalledWith()
  })

  it('does not couple shop ordering to the gateway payment switch', async () => {
    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = { payment_enabled: false }
    const meta = routerHarness.routes.find(route => route.path === '/shop/orders/:id?')!.meta!
    const { navigation, next } = runGuard(meta, '/shop/orders')
    await navigation
    expect(next).toHaveBeenCalledWith()
  })

  it('waits for the first public-settings request before deciding payment access', async () => {
    const deferred = createDeferred<{ payment_enabled: boolean }>()
    appStore.fetchPublicSettings.mockImplementation(async () => {
      const settings = await deferred.promise
      appStore.cachedPublicSettings = settings
      appStore.publicSettingsLoaded = true
      return settings
    })

    const { navigation, next } = runGuard({ requiresPayment: true }, '/purchase')

    await vi.waitFor(() => expect(appStore.fetchPublicSettings).toHaveBeenCalledTimes(1))
    expect(next).not.toHaveBeenCalled()

    deferred.resolve({ payment_enabled: true })
    await navigation
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it.each([
    ['payment', { requiresPayment: true }, '/purchase'],
    ['risk control', { requiresRiskControl: true }, '/admin/risk-control'],
    ['creative workbench', { requiresCreativeWorkbench: true }, '/studio'],
  ])('does not treat a failed %s settings load as explicitly disabled', async (_name, meta, path) => {
    authStore.isAdmin = meta.requiresRiskControl === true
    appStore.fetchPublicSettings.mockResolvedValue(null)

    const { navigation, next } = runGuard(meta, path)
    await navigation

    expect(appStore.publicSettingsLoaded).toBe(false)
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it.each([
    ['payment', { requiresPayment: true }, { payment_enabled: false }, '/dashboard'],
    [
      'risk control',
      { requiresRiskControl: true },
      { risk_control_enabled: false },
      '/admin/settings',
    ],
    [
      'creative workbench',
      { requiresCreativeWorkbench: true },
      { creative_workbench_enabled: false },
      '/dashboard',
    ],
  ])('redirects when loaded settings explicitly disable %s', async (_name, meta, settings, target) => {
    authStore.isAdmin = meta.requiresRiskControl === true
    appStore.cachedPublicSettings = settings
    appStore.publicSettingsLoaded = true

    const { navigation, next } = runGuard(meta, '/feature')
    await navigation

    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith(target)
  })

  it('redirects admins to settings when the creative workbench route is disabled', async () => {
    authStore.isAdmin = true
    appStore.cachedPublicSettings = { creative_workbench_enabled: false }
    appStore.publicSettingsLoaded = true

    const { navigation, next } = runGuard({ requiresCreativeWorkbench: true }, '/studio')
    await navigation

    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith('/admin/settings')
  })
})
