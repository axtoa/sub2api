import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ConsumePriorityToggle from '../ConsumePriorityToggle.vue'

const { save, showError } = vi.hoisted(() => ({ save: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin/accounts', () => ({ setConsumePriority: save }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('ConsumePriorityToggle', () => {
  beforeEach(() => { vi.resetAllMocks() })

  it('defaults off and commits only after the server succeeds, blocking duplicate clicks', async () => {
    let resolve!: (value: { enabled: boolean }) => void
    save.mockReturnValue(new Promise((done) => { resolve = done }))
    const wrapper = mount(ConsumePriorityToggle, { props: { account: { id: 1, platform: 'openai' } } })
    const button = wrapper.get('button')
    expect(button.attributes('aria-checked')).toBe('false')
    await button.trigger('click')
    await button.trigger('click')
    expect(save).toHaveBeenCalledOnce()
    expect(save).toHaveBeenCalledWith(1, true)
    expect(button.attributes('disabled')).toBeDefined()
    expect(wrapper.emitted('updated')).toBeUndefined()
    resolve({ enabled: true })
    await flushPromises()
    expect(wrapper.emitted('updated')).toEqual([[1, true]])
    expect(button.attributes('disabled')).toBeUndefined()
  })

  it('can turn off and does not change state on a failed save', async () => {
    const wrapper = mount(ConsumePriorityToggle, { props: { account: { id: 2, platform: 'anthropic', extra: { consume_priority_enabled: true } } } })
    save.mockRejectedValueOnce(new Error('network failure'))
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.get('button').attributes('aria-checked')).toBe('true')
    expect(wrapper.emitted('updated')).toBeUndefined()
    expect(showError).toHaveBeenCalledOnce()
    save.mockResolvedValueOnce({ enabled: false })
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(save).toHaveBeenLastCalledWith(2, false)
    expect(wrapper.emitted('updated')).toEqual([[2, false]])
  })

  it('does not offer unsupported platforms a switch', () => {
    const wrapper = mount(ConsumePriorityToggle, { props: { account: { id: 3, platform: 'gemini' } } })
    expect(wrapper.find('button').exists()).toBe(false)
  })
})
