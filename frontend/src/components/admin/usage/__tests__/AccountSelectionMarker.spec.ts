import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountSelectionMarker from '../AccountSelectionMarker.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('AccountSelectionMarker', () => {
  it.each([
    ['priority', 'bg-[#16a34a]', 'dark:bg-[#4ade80]'],
    ['normal', 'bg-[#2563eb]', 'dark:bg-[#60a5fa]'],
    ['fallback', 'bg-[#ea580c]', 'dark:bg-[#fb923c]']
  ])('renders %s with its agreed light/dark colors and accessible explanation', (reason, light, dark) => {
    const wrapper = mount(AccountSelectionMarker, { props: { reason } })
    const dot = wrapper.get('[role="img"]')
    expect(dot.classes()).toContain(light)
    expect(dot.classes()).toContain(dark)
    expect(dot.attributes('aria-label')).toBeTruthy()
    expect(dot.text()).toBe('')
  })

  it.each([undefined, null, '', 'unknown'])('does not mislabel historical or unknown records (%s)', (reason) => {
    const wrapper = mount(AccountSelectionMarker, { props: { reason } })
    expect(wrapper.find('[role="img"]').exists()).toBe(false)
  })
})
