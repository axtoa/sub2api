import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useAppStore } from '@/stores/app'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import type { ShopHomepageContent } from '@/types'

const changeKey = 'best-ai:official-shop-change'

export function useShopAvailability() {
  const app = useAppStore()
  const fallback = ref<boolean>()
  const enabled = computed(() => typeof app.cachedPublicSettings?.official_shop_enabled === 'boolean'
    ? isFeatureFlagEnabled(FeatureFlags.officialShop)
    : fallback.value ?? isFeatureFlagEnabled(FeatureFlags.officialShop))
  const homepageFallback = ref<boolean>()
  const homepageEnabled = computed(() => app.cachedPublicSettings?.shop_homepage_enabled ?? homepageFallback.value ?? false)
  const refresh = () => { void app.fetchPublicSettings(true) }
  const onStorage = (event: StorageEvent) => {
    if (event.key === changeKey) refresh()
  }

  function apply(enabled: boolean, notify = false) {
    fallback.value = enabled
    if (app.cachedPublicSettings) {
      app.cachedPublicSettings = { ...app.cachedPublicSettings, official_shop_enabled: enabled }
    }
    if (window.__APP_CONFIG__) {
      window.__APP_CONFIG__ = { ...window.__APP_CONFIG__, official_shop_enabled: enabled }
    }
    if (notify) {
      // Other tabs fetch authoritative settings, not the event's payload.
      try { localStorage.setItem(changeKey, crypto.randomUUID()) } catch { /* Focus still refreshes settings when storage is unavailable. */ }
    }
  }

  function applyExternal(enabled: boolean, url: string, notify = false) {
    if (app.cachedPublicSettings) {
      app.cachedPublicSettings = { ...app.cachedPublicSettings, external_shop_enabled: enabled, external_shop_url: url }
    }
    if (window.__APP_CONFIG__) {
      window.__APP_CONFIG__ = { ...window.__APP_CONFIG__, external_shop_enabled: enabled, external_shop_url: url }
    }
    if (notify) {
      try { localStorage.setItem(changeKey, crypto.randomUUID()) } catch { /* Focus still refreshes settings when storage is unavailable. */ }
    }
  }

  function applyHomepage(enabled: boolean, notify = false) {
    homepageFallback.value = enabled
    if (app.cachedPublicSettings) {
      app.cachedPublicSettings = { ...app.cachedPublicSettings, shop_homepage_enabled: enabled }
    }
    if (window.__APP_CONFIG__) {
      window.__APP_CONFIG__ = { ...window.__APP_CONFIG__, shop_homepage_enabled: enabled }
    }
    if (notify) {
      try { localStorage.setItem(changeKey, crypto.randomUUID()) } catch { /* Focus still refreshes settings when storage is unavailable. */ }
    }
  }

  function applyHomepageContent(content: ShopHomepageContent, notify = false) {
    if (app.cachedPublicSettings) {
      app.cachedPublicSettings = { ...app.cachedPublicSettings, shop_homepage_content: content }
    }
    if (window.__APP_CONFIG__) {
      window.__APP_CONFIG__ = { ...window.__APP_CONFIG__, shop_homepage_content: content }
    }
    if (notify) {
      try { localStorage.setItem(changeKey, crypto.randomUUID()) } catch { /* Focus still refreshes settings when storage is unavailable. */ }
    }
  }

  function applyLabel(channel: 'official' | 'external', name: string, notify = false) {
    const key = channel === 'official' ? 'official_shop_name' : 'external_shop_name'
    if (app.cachedPublicSettings) {
      app.cachedPublicSettings = { ...app.cachedPublicSettings, [key]: name }
    }
    if (window.__APP_CONFIG__) {
      window.__APP_CONFIG__ = { ...window.__APP_CONFIG__, [key]: name }
    }
    if (notify) {
      try { localStorage.setItem(changeKey, crypto.randomUUID()) } catch { /* Focus still refreshes settings when storage is unavailable. */ }
    }
  }

  onMounted(() => {
    window.addEventListener('storage', onStorage)
    window.addEventListener('focus', refresh)
  })
  onUnmounted(() => {
    window.removeEventListener('storage', onStorage)
    window.removeEventListener('focus', refresh)
  })
  return { enabled, homepageEnabled, apply, applyExternal, applyHomepage, applyHomepageContent, applyLabel }
}

export function isShopDisabled(error: unknown): boolean {
  return (error as { reason?: string } | null)?.reason === 'SHOP_DISABLED'
}
