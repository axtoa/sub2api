<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

const props = defineProps<{ reason?: string | null }>()
const { t } = useI18n()
const marker = computed(() => {
  switch (props.reason) {
    case 'priority': return { label: t('admin.accounts.selectionPriority'), color: 'bg-[#16a34a] dark:bg-[#4ade80]' }
    case 'normal': return { label: t('admin.accounts.selectionNormal'), color: 'bg-[#2563eb] dark:bg-[#60a5fa]' }
    case 'fallback': return { label: t('admin.accounts.selectionFallback'), color: 'bg-[#ea580c] dark:bg-[#fb923c]' }
    default: return null
  }
})
</script>

<template>
  <HelpTooltip v-if="marker" :content="marker.label" width-class="w-64" class="shrink-0">
    <template #trigger>
      <span role="img" :aria-label="marker.label" :data-selection-reason="reason" class="inline-block h-[7px] w-[7px] rounded-full" :class="marker.color" />
    </template>
  </HelpTooltip>
</template>
