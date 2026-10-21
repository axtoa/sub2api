import { beforeEach, describe, expect, it } from 'vitest'
import { useTheme } from '@/composables/useTheme'

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.setItem('theme', 'light')
    document.documentElement.classList.remove('dark')
  })

  it('shares theme state and persists changes for every page', () => {
    const homeTheme = useTheme()
    const authTheme = useTheme()

    homeTheme.toggleTheme()

    expect(authTheme.isDark.value).toBe(true)
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')

    authTheme.toggleTheme()

    expect(homeTheme.isDark.value).toBe(false)
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(localStorage.getItem('theme')).toBe('light')
  })
})
