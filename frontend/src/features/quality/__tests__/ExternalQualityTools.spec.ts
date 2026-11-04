import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ExternalQualityTools from '../ExternalQualityTools.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
describe('External quality tools', () => {
  it('opens fixed public links in isolated tabs and includes both model families', async () => {
    const wrapper = mount(ExternalQualityTools)
    expect(wrapper.findAll('article')).toHaveLength(5)
    for (const link of wrapper.findAll('a')) {
      const url = new URL(link.attributes('href'))
      expect(url.protocol).toBe('https:')
      expect(url.search).toBe('')
      expect(link.attributes('target')).toBe('_blank')
      expect(link.attributes('rel')).toBe('noopener noreferrer')
      expect(link.attributes('referrerpolicy')).toBe('no-referrer')
    }
    await wrapper.get('select').setValue('gpt')
    expect(wrapper.findAll('article')).toHaveLength(5)
    await wrapper.get('select').setValue('claude')
    expect(wrapper.findAll('article')).toHaveLength(5)
    expect(wrapper.text()).toContain('quality.external.benchmark')
    expect(wrapper.text()).toContain('quality.external.channel')
    wrapper.unmount()
  })
})
