import { onMounted, onUnmounted, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { createShopEntryId, hasShopEntryResult, removeShopEntryResult, shopEntryResultPrefix } from './entryFlow'

/** The anchor opens the new tab in the click itself, never after an asynchronous login. */
export function useHomeShopEntry(enabled: Readonly<Ref<boolean>>) {
  const router = useRouter()
  const auth = useAuthStore()
  const entryId = createShopEntryId()
  let startedAt: number | undefined
  let active = true
  let verifying = false

  function entryTo(product?: string) {
    return { path: '/shop', query: { ...(product ? { product } : {}), ...(entryId ? { home_entry: entryId } : {}) } }
  }

  function trackEntry(event: MouseEvent) {
    if (!enabled.value) { event.preventDefault(); return }
    if (entryId && !event.defaultPrevented && (event.button === 0 || event.button === 1)) startedAt = Date.now()
  }

  async function checkResult() {
    if (!active || verifying || !entryId || startedAt === undefined || !enabled.value ||
        router.currentRoute.value.path !== '/home' || !hasShopEntryResult(entryId, startedAt)) return
    verifying = true
    try {
      // A cross-tab signal is not authentication. Restore the shared session, then verify it with the server.
      auth.checkAuth()
      await auth.refreshUser()
      if (active && auth.isAuthenticated && enabled.value && router.currentRoute.value.path === '/home') {
        const failure = await router.replace(auth.isAdmin ? '/admin/dashboard' : '/dashboard')
        if (!failure) {
          startedAt = undefined
          removeShopEntryResult(entryId)
        }
      }
    } catch { /* Leave the homepage in place on an expired session or network error; focus can retry. */ }
    finally { verifying = false }
  }

  const onStorage = (event: StorageEvent) => {
    if (event.key === shopEntryResultPrefix + entryId) void checkResult()
  }
  const onFocus = () => { void checkResult() }
  onMounted(() => {
    window.addEventListener('storage', onStorage)
    window.addEventListener('focus', onFocus)
  })
  onUnmounted(() => {
    active = false
    window.removeEventListener('storage', onStorage)
    window.removeEventListener('focus', onFocus)
    if (entryId) removeShopEntryResult(entryId)
  })
  return { entryTo, trackEntry }
}
