<template>
  <section class="quality-form quality-library-picker">
    <div class="quality-card-heading"><h3>{{ t('quality.library.choose') }}</h3><button type="button" class="quality-text-button" :disabled="loading || disabled" @click="load">{{ t('quality.refresh') }}</button></div>
    <p class="quality-caption">{{ t('quality.library.selection', { count: modelValue.length, total: used }) }}</p>
    <input v-model="search" :placeholder="t('quality.library.search')" :aria-label="t('quality.library.search')" />
    <p v-if="error" class="quality-message error" role="alert">{{ error }}</p>
    <p v-if="loading" class="quality-caption">{{ t('quality.loading') }}</p>
    <div v-if="modelValue.length" class="quality-library-selected"><button v-for="q in modelValue" :key="q.id" type="button" class="quality-button" :disabled="disabled" :aria-label="t('quality.library.unselect', { name: q.name })" @click="toggle(q)">{{ q.name }} ×</button></div>
    <div class="quality-library-list">
      <label v-for="item in visible" :key="item.id" class="quality-library-item quality-library-choice"><input type="checkbox" :checked="selected(item.id)" :disabled="disabled || (!selected(item.id) && used >= 8)" @change="toggle(item)" /><span><strong>{{ item.name }}</strong><small class="quality-caption">{{ t(`quality.custom.${item.match}`) }}</small><span class="quality-library-preview">{{ item.prompt }}</span></span></label>
    </div>
    <p v-if="!loading && !error && !visible.length" class="quality-caption">{{ t(items.length ? 'quality.noMatches' : 'quality.library.empty') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { qualityAPI, type InspectionQuestionTemplate } from './api'
import { questionSnapshot } from './inspectionQuestions'
const props = defineProps<{ modelValue: InspectionQuestionTemplate[]; used: number; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: InspectionQuestionTemplate[]] }>()
const { t } = useI18n()
const items = ref<InspectionQuestionTemplate[]>([]), search = ref(''), error = ref(''), loading = ref(false)
const visible = computed(() => items.value.filter(q => `${q.name} ${q.prompt}`.toLowerCase().includes(search.value.trim().toLowerCase())))
const controller = new AbortController()
let disposed = false
function selected(id: string) { return props.modelValue.some(q => q.id === id) }
function toggle(item: InspectionQuestionTemplate) {
  if (props.disabled) return
  if (selected(item.id)) emit('update:modelValue', props.modelValue.filter(q => q.id !== item.id))
  else if (props.used < 8) emit('update:modelValue', [...props.modelValue, { id: item.id, ...questionSnapshot(item) }])
}
async function load() {
  loading.value = true; error.value = ''
  try { const bank = await qualityAPI.inspectionQuestions(controller.signal); if (!disposed) items.value = bank.items }
  catch { if (!disposed) error.value = t('quality.loadError') }
  finally { loading.value = false }
}
onMounted(load)
onUnmounted(() => { disposed = true; controller.abort() })
</script>
