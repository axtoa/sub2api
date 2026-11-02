import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { ref } from 'vue'
import SidebarThemeMenu from '../SidebarThemeMenu.vue'

const state = vi.hoisted(() => ({ setTheme: vi.fn() }))
const theme = ref<'light' | 'dark' | 'system'>('system')
vi.mock('@/composables/useTheme', () => ({ useTheme: () => ({ theme, setTheme: state.setTheme }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
let wrapper: VueWrapper
beforeEach(() => { theme.value = 'system'; state.setTheme.mockClear() })
afterEach(() => { wrapper.unmount() })
function render(collapsed = false) {
  wrapper = mount(SidebarThemeMenu, { props: { collapsed }, attachTo: document.body })
  return wrapper.get('button[aria-haspopup="menu"]')
}

describe('SidebarThemeMenu', () => {
  it('opens three choices from the existing footer entry and restores focus after selection', async () => {
    const trigger = render()
    expect(trigger.attributes('aria-label')).toBe('nav.theme · nav.systemMode')
    await trigger.trigger('click'); await flushPromises()
    const options = wrapper.findAll('[role="menuitemradio"]')
    expect(options.map(option => option.text())).toEqual(['nav.lightMode', 'nav.darkMode', 'nav.systemMode'])
    expect(options[2].attributes('aria-checked')).toBe('true')
    expect(document.activeElement).toBe(options[2].element)
    await options[1].trigger('click')
    expect(state.setTheme).toHaveBeenCalledWith('dark')
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)
  })

  it('supports arrow navigation and Escape in collapsed mode', async () => {
    const trigger = render(true)
    expect(trigger.attributes('aria-label')).toContain('nav.systemMode')
    await trigger.trigger('keydown', { key: 'ArrowUp' }); await flushPromises()
    const options = wrapper.findAll('[role="menuitemradio"]')
    await options[2].trigger('keydown', { key: 'ArrowDown' })
    expect(document.activeElement).toBe(options[0].element)
    await options[0].trigger('keydown', { key: 'End' })
    expect(document.activeElement).toBe(options[2].element)
    await options[2].trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)
  })

  it('updates the footer label with the preference and closes on sidebar collapse', async () => {
    const trigger = render()
    theme.value = 'dark'; await flushPromises()
    expect(trigger.attributes('aria-label')).toContain('nav.darkMode')
    await trigger.trigger('click')
    await wrapper.setProps({ collapsed: true })
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
  })
})
