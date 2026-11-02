import { readonly, ref } from 'vue'

export type ThemeMode = 'light' | 'dark' | 'system'
const theme = ref<ThemeMode>('system')
const isDark = ref(false)
let initialized = false
let systemTheme: MediaQueryList | undefined

function parseTheme(value: string | null): ThemeMode {
  return value === 'light' || value === 'dark' ? value : 'system'
}

function applyTheme() {
  const dark = theme.value === 'dark' || (theme.value === 'system' && systemTheme?.matches === true)
  isDark.value = dark
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
}

function initTheme() {
  if (initialized || typeof window === 'undefined') return
  initialized = true
  systemTheme = window.matchMedia('(prefers-color-scheme: dark)')
  try { theme.value = parseTheme(localStorage.getItem('theme')) } catch { theme.value = 'system' }
  applyTheme()

  const onSystemChange = () => { if (theme.value === 'system') applyTheme() }
  systemTheme.addEventListener?.('change', onSystemChange)

  const onStorage = (event: StorageEvent) => {
    if (event.key !== 'theme' && event.key !== null) return
    if (event.storageArea && event.storageArea !== window.localStorage) return
    theme.value = parseTheme(event.newValue)
    applyTheme()
  }
  window.addEventListener('storage', onStorage)
  import.meta.hot?.dispose(() => {
    systemTheme?.removeEventListener?.('change', onSystemChange)
    window.removeEventListener('storage', onStorage)
  })
}

export function useTheme() {
  initTheme()

  function setTheme(mode: ThemeMode) {
    theme.value = mode
    applyTheme()
    try { localStorage.setItem('theme', mode) } catch { /* Keep the current page usable without storage. */ }
  }

  return { theme: readonly(theme), isDark: readonly(isDark), setTheme }
}
