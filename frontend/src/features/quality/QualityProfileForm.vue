<template>
  <div class="quality-profile">
    <div v-for="kind in kinds" :key="kind" class="quality-schedule-row">
      <label class="quality-check"><input :checked="modelValue[enabledKey(kind)]" type="checkbox" @change="update(enabledKey(kind), ($event.target as HTMLInputElement).checked)" /><span>{{ t(`quality.${kind}Enabled`) }}</span></label>
      <label>{{ t(`quality.${kind}Interval`) }}<input :value="modelValue[intervalKey(kind)]" type="number" :min="kind === 'availability' ? 1 : 5" max="10080" step="1" required @input="update(intervalKey(kind), Number(($event.target as HTMLInputElement).value))" /></label>
    </div>
    <p class="quality-caption">{{ t('quality.cacheDefault') }}</p>
    <details class="quality-advanced" :open="advancedOpen"><summary>{{ t('quality.advanced') }}</summary>
      <div class="quality-form-grid">
        <label v-for="field in numericFields" :key="field.key">{{ t(`quality.${field.label}`) }}<input :value="modelValue[field.key]" type="number" :min="field.min" :max="field.max" step="1" required @input="update(field.key, Number(($event.target as HTMLInputElement).value))" /></label>
        <label>{{ t('quality.reasoning') }}<select :value="modelValue.reasoning_effort" @change="update('reasoning_effort', ($event.target as HTMLSelectElement).value)"><option value="">{{ t('quality.native') }}</option><option v-for="effort in ['low','medium','high']" :key="effort" :value="effort">{{ t(`quality.${effort}`) }}</option></select></label>
      </div>
      <p class="quality-caption">{{ t('quality.outputHint') }}</p>
    </details>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { CheckKind, QualityProfile } from './api'

const props = defineProps<{ modelValue: QualityProfile; advancedOpen?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: QualityProfile] }>()
const { t } = useI18n()
const kinds: CheckKind[] = ['availability', 'quality', 'cache']
const enabledKey = (kind: CheckKind): `${CheckKind}_enabled` => `${kind}_enabled`
const intervalKey = (kind: CheckKind): `${CheckKind}_interval_minutes` => `${kind}_interval_minutes`
function update(key: keyof QualityProfile, value: boolean | number | string) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}
const numericFields: Array<{ key: keyof QualityProfile; label: string; min: number; max: number }> = [
  { key: 'questions', label: 'questions', min: 1, max: 8 },
  { key: 'pass_percent', label: 'passPercent', min: 50, max: 100 },
  { key: 'retry_count', label: 'retryCount', min: 0, max: 3 },
  { key: 'retry_interval_minutes', label: 'retryInterval', min: 1, max: 1440 },
  { key: 'max_output_tokens', label: 'maxOutput', min: 256, max: 8192 },
  { key: 'cache_prefix_kb', label: 'prefixKB', min: 8, max: 64 },
]
</script>
