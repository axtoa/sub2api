<template>
  <section class="quality-custom-questions">
    <div class="quality-card-heading"><h3>{{ t('quality.custom.title') }}</h3><button type="button" class="quality-text-button" :disabled="disabled || modelValue.length >= 8" @click="add">{{ t('quality.addQuestion') }}</button></div>
    <p class="quality-caption">{{ t('quality.custom.hint') }}</p>
    <article v-for="(question, index) in modelValue" :key="index" class="quality-custom-question">
      <div class="quality-form-grid">
        <label>{{ t('quality.questionName') }}<input :value="question.name" maxlength="100" required :disabled="disabled" @input="update(index, 'name', ($event.target as HTMLInputElement).value)" /></label>
        <label>{{ t('quality.custom.match') }}<select :value="question.match" :disabled="disabled" @change="update(index, 'match', ($event.target as HTMLSelectElement).value)"><option value="exact">{{ t('quality.custom.exact') }}</option><option value="contains">{{ t('quality.custom.contains') }}</option><option value="manual">{{ t('quality.custom.manual') }}</option></select></label>
      </div>
      <label>{{ t('quality.question') }}<textarea :value="question.prompt" rows="3" maxlength="4000" required :disabled="disabled" @input="update(index, 'prompt', ($event.target as HTMLTextAreaElement).value)" /></label>
      <label>{{ t(question.match === 'contains' ? 'quality.custom.keywords' : question.match === 'manual' ? 'quality.custom.reference' : 'quality.expected') }}<textarea :value="question.expected" rows="2" maxlength="1000" :required="question.match !== 'manual'" :disabled="disabled" @input="update(index, 'expected', ($event.target as HTMLTextAreaElement).value)" /></label>
      <p v-if="question.match === 'manual'" class="quality-caption">{{ t('quality.custom.manualHint') }}</p>
      <button type="button" class="quality-text-button" :disabled="disabled" @click="emit('update:modelValue', modelValue.filter((_, i) => i !== index))">{{ t('quality.removeQuestion') }}</button>
    </article>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { InspectionQuestion } from './api'
const props = defineProps<{ modelValue: InspectionQuestion[]; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: InspectionQuestion[]] }>()
const { t } = useI18n()
function add() {
  if (props.disabled || props.modelValue.length >= 8) return
  emit('update:modelValue', [...props.modelValue, { name: '', prompt: '', match: 'manual', expected: '' }])
}
function update(index: number, field: keyof InspectionQuestion, value: string) {
  emit('update:modelValue', props.modelValue.map((q, i) => i === index ? { ...q, [field]: value } : q))
}
</script>
