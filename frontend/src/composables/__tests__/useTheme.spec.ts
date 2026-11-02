import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

let media: { matches: boolean; addEventListener: ReturnType<typeof vi.fn>; removeEventListener: ReturnType<typeof vi.fn> }
let onStorage: (event: StorageEvent) => void

beforeEach(() => {
  vi.resetModules()
  localStorage.clear()
  document.documentElement.classList.remove('dark')
  media = { matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }
  vi.stubGlobal('matchMedia', vi.fn(() => media))
  const addListener = window.addEventListener.bind(window)
  vi.spyOn(window, 'addEventListener').mockImplementation((type, listener, options) => {
    if (type === 'storage') onStorage = listener as (event: StorageEvent) => void
    else addListener(type, listener, options)
  })
})
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

async function setup(saved?: string, dark = false) {
  if (saved) localStorage.setItem('theme', saved)
  media.matches = dark
  const { useTheme } = await import('@/composables/useTheme')
  return { ...useTheme(), useTheme }
}
function changeSystem(dark: boolean) {
  media.matches = dark
  media.addEventListener.mock.calls[0][1]()
}
function storage(key: string | null, newValue: string | null, storageArea = window.localStorage) {
  onStorage({ key, newValue, storageArea } as StorageEvent)
}

describe('useTheme', () => {
  it.each([undefined, 'system', 'invalid'])('defaults %s to system and follows live OS changes', async saved => {
    const theme = await setup(saved, true)
    expect(theme.theme.value).toBe('system')
    expect(theme.isDark.value).toBe(true)
    expect(document.documentElement.style.colorScheme).toBe('dark')
    changeSystem(false)
    expect(theme.isDark.value).toBe(false)
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it.each(['light', 'dark'] as const)('preserves saved %s regardless of the OS', async mode => {
    const theme = await setup(mode, mode !== 'dark')
    expect(theme.theme.value).toBe(mode)
    expect(theme.isDark.value).toBe(mode === 'dark')
    changeSystem(true)
    changeSystem(false)
    expect(theme.isDark.value).toBe(mode === 'dark')
  })

  it('shares preferences, persists all three modes and resumes following the OS', async () => {
    const first = await setup('light', true)
    const second = first.useTheme()
    first.setTheme('dark')
    expect(second.isDark.value).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    second.setTheme('light')
    expect(first.isDark.value).toBe(false)
    first.setTheme('system')
    expect(second.theme.value).toBe('system')
    expect(second.isDark.value).toBe(true)
    expect(localStorage.getItem('theme')).toBe('system')
    changeSystem(false)
    expect(second.isDark.value).toBe(false)
    expect(media.addEventListener).toHaveBeenCalledOnce()
  })

  it('syncs other tabs and returns to the OS after removing or clearing the preference', async () => {
    const theme = await setup('light', true)
    storage('theme', 'dark')
    expect(theme.isDark.value).toBe(true)
    storage('theme', 'system')
    changeSystem(false)
    expect(theme.isDark.value).toBe(false)
    storage('theme', 'dark')
    storage('theme', null)
    expect(theme.theme.value).toBe('system')
    expect(theme.isDark.value).toBe(false)
    storage('theme', 'dark')
    storage(null, null)
    expect(theme.theme.value).toBe('system')
    expect(theme.isDark.value).toBe(false)
  })

  it('ignores unrelated storage updates', async () => {
    const theme = await setup('light')
    storage('other', 'dark')
    storage('theme', 'dark', sessionStorage)
    expect(theme.theme.value).toBe('light')
  })

  it('still applies preferences when browser storage is unavailable', async () => {
    vi.spyOn(localStorage, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    const theme = await setup(undefined, true)
    expect(theme.isDark.value).toBe(true)
    vi.spyOn(localStorage, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    expect(() => theme.setTheme('light')).not.toThrow()
    expect(theme.isDark.value).toBe(false)
  })
})
