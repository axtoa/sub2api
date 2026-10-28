<template>
  <section v-if="run" class="quality-evidence">
    <div class="quality-card-heading"><strong :class="runTone(run)">{{ t(`quality.${verdictKey(run)}`) }}</strong><span class="quality-caption">{{ date(run.finished_at || run.created_at) }}</span></div>
    <p v-if="run.result?.reason" class="quality-message">{{ reason(run.result.reason) }}</p>
    <div v-if="run.kind === 'cache'" class="quality-cache-evidence">
      <p>{{ t('quality.validationRatio') }}: <strong>{{ cachePercent(run) == null ? '—' : `${cachePercent(run)!.toFixed(2)}%` }}</strong></p>
      <div v-for="(item, index) in items" :key="item.question_id" class="quality-probe-row"><span>{{ t('quality.requestNumber', { count: index + 1 }) }} · {{ t(index === 0 ? 'quality.warmup' : 'quality.validation') }}</span><span>{{ t('quality.input') }} {{ item.probe.input_tokens }} / {{ t('quality.cacheRead') }} {{ item.probe.cache_reported ? item.probe.cache_read_tokens : '—' }} token</span></div>
    </div>
    <template v-if="items.length">
      <div class="quality-question-tabs" role="group" :aria-label="t('quality.question')"><button v-for="(item, index) in items" :key="item.question_id" class="quality-button" :aria-pressed="sampleIndex === index" @click="sampleIndex = index">{{ t('quality.questionNumber', { count: index + 1 }) }} · {{ t(run.kind !== 'quality' ? item.probe.complete ? 'quality.completed' : 'quality.incompleteShort' : item.correct === true ? 'quality.correct' : item.correct === false ? 'quality.incorrect' : 'quality.ungraded') }}</button></div>
      <template v-if="sample">
        <h3>{{ sample.name || (te(`quality.families.${sample.family}`) ? t(`quality.families.${sample.family}`) : t('quality.question')) }}</h3>
        <pre class="quality-question">{{ sample.prompt }}</pre>
        <div class="quality-answer-grid" :class="{'quality-answer-single':run.kind!=='quality'}"><div v-if="run.kind==='quality'"><h3>{{ t('quality.expected') }}</h3><pre>{{ sample.expected }}</pre><p v-if="sample.explanation" class="quality-caption">{{ sample.explanation }}</p></div><div><h3>{{ t('quality.answer') }}</h3><pre>{{ sample.probe.text || '—' }}</pre><p v-if="sample.probe.reason" class="quality-caption">{{ reason(sample.probe.reason) }}<span v-if="sample.probe.error_param"> · {{ sample.probe.error_param }}</span></p></div></div>
        <p v-if="run.kind === 'quality'" :class="sample.correct === false ? 'warn' : 'quality-caption'">{{ sample.grade_reason ? reason(sample.grade_reason) : t(sample.correct === true ? 'quality.matchReason' : sample.correct === false ? 'quality.mismatchReason' : 'quality.incompleteReason') }}</p>
        <details class="quality-protocol"><summary>{{ t('quality.profile') }}</summary><dl class="quality-facts"><div><dt>{{ t('quality.requestedModel') }}</dt><dd>{{ target?.model || '—' }}</dd></div><div><dt>{{ t('quality.actualModel') }}</dt><dd>{{ sample.probe.model || '—' }}</dd></div><div><dt>HTTP</dt><dd>{{ sample.probe.http_status }}</dd></div><div><dt>{{ t('quality.ttft') }}</dt><dd>{{ sample.probe.first_token_ms == null ? '—' : `${sample.probe.first_token_ms} ms` }}</dd></div><div><dt>{{ t('quality.maxOutput') }}</dt><dd>{{ sample.probe.native_output_limit ? t('quality.nativeLimit') : run.profile?.max_output_tokens }}</dd></div><div><dt>{{ t('quality.reasoning') }}</dt><dd>{{ run.profile?.reasoning_effort || t('quality.native') }}</dd></div></dl><p class="quality-caption">{{ t('quality.usage') }}: {{ t('quality.input') }} {{ sample.probe.input_tokens }} / {{ t('quality.output') }} {{ sample.probe.output_tokens }} / {{ t('quality.cacheRead') }} {{ sample.probe.cache_read_tokens }} / {{ t('quality.cacheWrite') }} {{ sample.probe.cache_write_tokens }}</p><p v-if="!sample.probe.usage_reported" class="quality-caption">{{ t('quality.usageMissing') }}</p><p class="quality-caption">{{ t('quality.fingerprint') }}: {{ run.profile_hash }}</p><details v-if="run.result?.cache_prefix"><summary>{{ t('quality.cachePrefix') }}</summary><pre>{{ run.result.cache_prefix }}</pre></details><p class="quality-caption">{{ t('quality.referenceHint') }}</p></details>
      </template>
    </template>
    <p v-else class="quality-caption">{{ t(run.status === 'finished' ? 'quality.noEvidence' : 'quality.waitingEvidence') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { QualityRun, QualityTarget } from './api'
import { cachePercent, runTone, verdictKey } from './display'
const props = defineProps<{ run: QualityRun | null; target: QualityTarget | null }>()
const { t, te } = useI18n()
const sampleIndex = ref(0)
const items = computed(() => props.run?.result?.items ?? [])
const sample = computed(() => items.value[sampleIndex.value])
watch(() => props.run?.id, () => { sampleIndex.value = 0 })
function date(value?: string | null) { return value ? new Date(value).toLocaleString() : '—' }
function reason(code: string) { return te(`quality.reasons.${code}`) ? t(`quality.reasons.${code}`) : t('quality.unknown') }
</script>
