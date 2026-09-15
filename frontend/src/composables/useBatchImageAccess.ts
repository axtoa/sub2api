import { computed, ref } from 'vue'
import { keysAPI } from '@/api/keys'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import type { ApiKey } from '@/types'

const loaded = ref(false)
const loading = ref(false)
const hasAllowedBatchImageKey = ref(false)
const hasAllowedCreativeVideoKey = ref(false)
let pendingLoad: Promise<boolean> | null = null
const pageSize = 100

export function keyAllowsBatchImage(key: ApiKey): boolean {
  return (
    key.status === 'active' &&
    key.group?.platform === 'gemini' &&
    key.group?.allow_batch_image_generation === true
  )
}

export function keyAllowsCreativeVideo(key: ApiKey): boolean {
  return (
    key.status === 'active' &&
    (key.group?.platform === 'grok' || key.group?.platform === 'composite')
  )
}

async function loadBatchImageAccess(force = false): Promise<boolean> {
  const authStore = useAuthStore()
  if (!authStore.isAuthenticated) {
    loaded.value = true
    hasAllowedBatchImageKey.value = false
    hasAllowedCreativeVideoKey.value = false
    return false
  }

  if (loaded.value && !force) {
    return hasAllowedBatchImageKey.value
  }

  if (pendingLoad && !force) {
    return pendingLoad
  }

  loading.value = true
  pendingLoad = (async () => {
    let page = 1
    let foundBatchImageKey = false
    let foundCreativeVideoKey = false
    while (true) {
      const response = await keysAPI.list(page, pageSize, {
        status: 'active',
        sort_by: 'created_at',
        sort_order: 'desc'
      })

      for (const key of response.items || []) {
        if (keyAllowsBatchImage(key)) foundBatchImageKey = true
        if (keyAllowsCreativeVideo(key)) foundCreativeVideoKey = true
      }

      // Keep scanning until both capabilities are known so video-only users
      // can see the studio without changing the image access flag.
      if (
        (foundBatchImageKey && foundCreativeVideoKey) ||
        page >= response.pages ||
        (response.items || []).length === 0
      ) {
        hasAllowedBatchImageKey.value = foundBatchImageKey
        hasAllowedCreativeVideoKey.value = foundCreativeVideoKey
        loaded.value = true
        return foundBatchImageKey
      }

      page += 1
    }
  })()
    .catch(() => {
      hasAllowedBatchImageKey.value = false
      hasAllowedCreativeVideoKey.value = false
      loaded.value = true
      return false
    })
    .finally(() => {
      loading.value = false
      pendingLoad = null
    })

  return pendingLoad
}

export function useBatchImageAccess() {
  const appStore = useAppStore()
  const canUseBatchImage = computed(() => hasAllowedBatchImageKey.value)
  const canUseCreativeStudio = computed(
    () =>
      appStore.cachedPublicSettings?.creative_workbench_enabled !== false &&
      (hasAllowedBatchImageKey.value || hasAllowedCreativeVideoKey.value),
  )

  return {
    canUseBatchImage,
    canUseCreativeStudio,
    batchImageAccessLoaded: computed(() => loaded.value),
    batchImageAccessLoading: computed(() => loading.value),
    refreshBatchImageAccess: loadBatchImageAccess,
  }
}
