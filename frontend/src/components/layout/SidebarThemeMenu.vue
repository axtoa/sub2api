<template>
  <div ref="container" class="relative mb-1" @keydown.esc.stop.prevent="close(true)">
    <button
      ref="trigger"
      type="button"
      class="sidebar-link w-full"
      :class="{ 'sidebar-link-collapsed': collapsed }"
      :title="label"
      :aria-label="label"
      aria-haspopup="menu"
      :aria-expanded="open"
      aria-controls="sidebar-theme-menu"
      @click="open ? close() : show()"
      @keydown.down.prevent="show()"
      @keydown.up.prevent="show()"
    >
      <Icon :name="current.icon" class="h-5 w-5 flex-shrink-0" />
      <span class="sidebar-label" :class="{ 'sidebar-label-collapsed': collapsed }" :aria-hidden="collapsed ? 'true' : 'false'">{{ t(current.label) }}</span>
      <Icon v-if="!collapsed" name="chevronUp" size="xs" class="ml-auto shrink-0 text-gray-400" />
    </button>
    <div
      v-if="open"
      id="sidebar-theme-menu"
      ref="menu"
      role="menu"
      :aria-label="t('nav.theme')"
      class="absolute bottom-full left-0 z-50 mb-2 w-52 rounded-xl border border-gray-200 bg-white p-1.5 shadow-lg dark:border-dark-700 dark:bg-dark-800"
      @keydown="onMenuKeydown"
    >
      <button
        v-for="option in options"
        :key="option.value"
        type="button"
        role="menuitemradio"
        :aria-checked="theme === option.value"
        :tabindex="theme === option.value ? 0 : -1"
        class="flex min-h-10 w-full items-center gap-2.5 rounded-lg px-3 text-left text-sm text-gray-700 hover:bg-gray-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-gray-200 dark:hover:bg-dark-700"
        :class="{ 'bg-primary-50 dark:bg-primary-900/20': theme === option.value }"
        @click="select(option.value)"
      >
        <Icon :name="option.icon" size="sm" />
        <span>{{ t(option.label) }}</span>
        <Icon v-if="theme === option.value" name="check" size="sm" class="ml-auto text-primary-600 dark:text-primary-400" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useTheme, type ThemeMode } from '@/composables/useTheme'

const props = defineProps<{ collapsed: boolean }>()
const { t } = useI18n()
const { theme, setTheme } = useTheme()
const options = [
  { value: 'light', label: 'nav.lightMode', icon: 'sun' },
  { value: 'dark', label: 'nav.darkMode', icon: 'moon' },
  { value: 'system', label: 'nav.systemMode', icon: 'desktop' },
] as const
const current = computed(() => options.find(option => option.value === theme.value) || options[2])
const label = computed(() => `${t('nav.theme')} · ${t(current.value.label)}`)
const open = ref(false)
const container = ref<HTMLElement>()
const menu = ref<HTMLElement>()
const trigger = ref<HTMLButtonElement>()

function close(restoreFocus = false) {
  open.value = false
  if (restoreFocus) trigger.value?.focus()
}
async function show() {
  open.value = true
  await nextTick()
  menu.value?.querySelector<HTMLElement>('[aria-checked="true"]')?.focus()
}
function select(mode: ThemeMode) { setTheme(mode); close(true) }
function onMenuKeydown(event: KeyboardEvent) {
  if (event.key === 'Tab') { close(true); return }
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const items = Array.from(menu.value?.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]') || [])
  const index = items.indexOf(document.activeElement as HTMLButtonElement)
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1
    : (index + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length
  items[next]?.focus()
}
onClickOutside(container, () => close())
watch(() => props.collapsed, () => close())
</script>
