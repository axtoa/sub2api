import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import ShopLayout from '../ShopLayout.vue'

vi.mock('@/stores/app', () => ({ useAppStore: () => ({ siteName: 'Best AI', siteLogo: '', cachedPublicSettings: null }) }))
const auth = vi.hoisted(() => ({ isAuthenticated: false }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
beforeEach(() => { auth.isAuthenticated = false })

vi.mock('vue-i18n', async original => ({
  ...await original<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

async function mountLayout(admin: boolean) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: ['/shop', '/shop/orders', '/admin/shop', '/admin/shop/products', '/admin/shop/settings', '/admin/shop/cdk', '/admin/shop/homepage'].map(path => ({
      path,
      component: { template: '<div />' },
    })),
  })
  await router.push(admin ? '/admin/shop' : '/shop')
  const wrapper = mount(ShopLayout, {
    props: { admin },
    slots: { default: '<p data-test="shop-content">Content</p>' },
    global: {
      plugins: [router],
      stubs: { AppLayout: { template: '<div data-test="console-layout"><slot /></div>' } },
    },
  })
  return { wrapper, router }
}

async function mountPublicStorefront() {
  const router = createRouter({ history:createMemoryHistory(), routes:[{path:'/shop',component:{template:'<div />'}},{path:'/shop/login',name:'ShopLogin',component:{template:'<div />'}},{path:'/',component:{template:'<div />'}}] })
  await router.push('/shop')
  const wrapper = mount(ShopLayout,{props:{embedded:false,storefront:true,siteName:'Best AI'},slots:{default:'<p data-test="shop-content">Content</p>'},global:{plugins:[router],stubs:{AppLayout:{template:'<div data-test="console-layout"><slot /></div>'}}}})
  return wrapper
}

describe('ShopLayout', () => {
  it('renders guest storefront without the console shell', async () => {
    const wrapper=await mountPublicStorefront()
    expect(wrapper.find('[data-test="console-layout"]').exists()).toBe(false)
    expect(wrapper.get('.cdk-public-brand').text()).toBe('Best AI')
    expect(wrapper.get('.cdk-public-logo svg').exists()).toBe(true)
    expect(wrapper.get('.cdk-public-brand').attributes('href')).toBe('/shop')
    expect(wrapper.get('.cdk-public-login').attributes('href')).toBe('/shop/login?redirect=/shop')
    expect(wrapper.get('a[href*="lookup=1"]').attributes('href')).toBe('/shop?lookup=1#shop-order-lookup')
    expect(wrapper.get('[data-test="shop-content"]').text()).toBe('Content')
  })
  it('preserves the current product in the header login entry', async () => {
    const wrapper = await mountPublicStorefront()
    await wrapper.setProps({ loginReturnTo: '/shop?product=claude#shop-product-details' })
    const href = new URL(wrapper.get('.cdk-public-login').attributes('href'), 'https://shop.example')
    expect(href.pathname).toBe('/shop/login')
    expect(href.searchParams.get('redirect')).toBe('/shop?product=claude#shop-product-details')
  })
  it('shows signed-in status instead of the login button for authenticated buyers', async () => {
    auth.isAuthenticated = true
    const wrapper = await mountPublicStorefront()
    expect(wrapper.find('.cdk-public-login').exists()).toBe(false)
    expect(wrapper.get('.cdk-public-account').text()).toBe('shop.storefrontSignedIn')
  })
  it.each([
    { admin: false, target: '/shop/orders' },
    { admin: true, target: '/admin/shop/products' },
    { admin: true, target: '/admin/shop/settings' },
  ])('shows the $admin shop inside the console and keeps tab navigation', async ({ admin, target }) => {
    const { wrapper, router } = await mountLayout(admin)
    expect(wrapper.get('[data-test="console-layout"] [data-test="shop-content"]').text()).toBe('Content')
    expect(wrapper.find('.shop-header').exists()).toBe(false)
    expect(wrapper.find('main').exists()).toBe(false)
    const link = wrapper.get(`.shop-tabs a[href="${target}"]`)
    await link.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe(target)
    expect(link.classes()).toContain('active')
  })
})
