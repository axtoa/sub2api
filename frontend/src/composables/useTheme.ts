import { ref } from 'vue'

const isDark = ref(false)
let initialized = false

function applyTheme(dark: boolean) {
  isDark.value = dark
  document.documentElement.classList.toggle('dark', dark)
}

function initTheme() {
  if (initialized || typeof window === 'undefined') return
  initialized = true

  const savedTheme = localStorage.getItem('theme')
  applyTheme(savedTheme === 'dark' || (
    savedTheme !== 'light' && window.matchMedia('(prefers-color-scheme: dark)').matches
  ))

  window.addEventListener('storage', (event) => {
    if (event.key === 'theme') applyTheme(event.newValue === 'dark')
  })
}

export function useTheme() {
  initTheme()

  function toggleTheme() {
    const nextTheme = !isDark.value
    applyTheme(nextTheme)
    localStorage.setItem('theme', nextTheme ? 'dark' : 'light')
  }

  return { isDark, toggleTheme }
}
