<template>
  <button type="button" class="shop-secondary shop-copy-button" :aria-label="copied ? '已复制' : label" @click="copy">
    <Icon :name="copied ? 'check' : 'copy'" size="sm" aria-hidden="true" />
    <span aria-live="polite">{{ copied ? '已复制' : label }}</span>
  </button>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'

const props = defineProps<{ text: string; label: string }>()
const { copied, copyToClipboard } = useClipboard()
async function copy() {
  await copyToClipboard(props.text, `${props.label.replace(/^复制/, '')}已复制`)
}
</script>
