import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import ShopLayout from '../ShopLayout.vue'

vi.mock('vue-i18n', async original => ({
  ...await original<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

async function mountLayout(admin: boolean) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: ['/shop', '/shop/orders', '/admin/shop', '/admin/shop/products'].map(path => ({
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

describe('ShopLayout', () => {
  it.each([
    { admin: false, target: '/shop/orders' },
    { admin: true, target: '/admin/shop/products' },
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
