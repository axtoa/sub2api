<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { setConsumePriority } from '@/api/admin/accounts'
import { useAppStore } from '@/stores/app'
import type { Account } from '@/types'

const props = defineProps<{ account: Pick<Account, 'id' | 'platform' | 'extra'> }>()
const emit = defineEmits<{ updated: [id: number, enabled: boolean] }>()
const { t } = useI18n()
const appStore = useAppStore()
const saving = ref(false)
const enabled = computed(() => props.account.extra?.consume_priority_enabled === true)
const supported = computed(() => ['openai', 'anthropic'].includes(props.account.platform))

async function toggle() {
  if (saving.value) return
  const id = props.account.id
  saving.value = true
  try {
    const result = await setConsumePriority(id, !enabled.value)
    emit('updated', id, result.enabled)
  } catch {
    appStore.showError(t('admin.accounts.consumePrioritySaveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <button
    v-if="supported"
    type="button"
    role="switch"
    :aria-checked="enabled"
    :aria-label="t('admin.accounts.consumePriority')"
    :title="t(enabled ? 'admin.accounts.consumePriorityEnabled' : 'admin.accounts.consumePriorityDisabled')"
    :disabled="saving"
    class="relative inline-flex h-4 w-7 shrink-0 items-center rounded-full transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500 disabled:cursor-wait disabled:opacity-50"
    :class="enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-500'"
    @click.stop="toggle"
  >
    <span class="pointer-events-none h-3 w-3 rounded-full bg-white shadow-sm transition-transform" :class="enabled ? 'translate-x-3.5' : 'translate-x-0.5'" />
  </button>
</template>
