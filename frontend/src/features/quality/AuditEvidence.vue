<template>
  <section v-if="audit" class="quality-audit-evidence">
    <h3>{{ t('quality.audit.title') }}</h3>
    <p class="quality-caption">{{ t('quality.audit.limit') }}</p>
    <p class="quality-caption">{{ t('quality.audit.seed') }}: {{ audit.seed }} · {{ audit.version }}</p>
    <p v-if="!audit.comparisons.some(comparison => comparison.reference !== 'repeat')" class="quality-message">{{ t('quality.audit.noReference') }}</p>
    <div class="quality-audit-table"><table v-if="audit.comparisons.length">
      <thead><tr><th>{{ t('quality.audit.reference') }}</th><th>{{ t('quality.audit.sample') }}</th><th>{{ t('quality.audit.input') }}</th><th>{{ t('quality.audit.delta') }}</th><th>{{ t('quality.audit.extra') }}</th><th>{{ t('quality.audit.share') }}</th><th>{{ t('quality.audit.result') }}</th></tr></thead>
      <tbody><tr v-for="(comparison,index) in audit.comparisons" :key="index"><td>{{ referenceName(comparison.reference) }}</td><td>{{ comparison.pair + 1 }}</td><td>{{ comparison.extra_percent == null ? '—' : comparison.primary_input + ' / ' + comparison.reference_input }}</td><td>{{ comparison.extra_percent == null ? '—' : signed(comparison.difference) }}</td><td>{{ percent(comparison.extra_percent) }}</td><td>{{ percent(comparison.reported_share_percent) }}</td><td :class="comparison.status === 'difference' ? 'warn' : ''">{{ t('quality.audit.statuses.' + comparison.status) }}</td></tr></tbody>
    </table></div>
    <p v-for="fit in audit.fits ?? []" :key="fit.reference" class="quality-caption">{{ t('quality.audit.fit', {reference:referenceName(fit.reference), n:fit.samples, min:fit.input_min, max:fit.input_max, fixed:fit.fixed_extra.toFixed(1), growth:(fit.growth_extra * 100).toFixed(2), error:fit.rmse.toFixed(1)}) }}</p>
    <div v-for="lane in lanes" :key="lane" class="quality-message">
      <strong>{{ referenceName(lane) }}</strong>
      <p class="quality-caption">{{ t('quality.audit.observed', {correct:laneSamples(lane).filter(sample => sample.correct === true).length, graded:laneSamples(lane).filter(sample => sample.correct != null).length, total:laneSamples(lane).length}) }}</p>
    </div>
    <details v-for="(sample,index) in audit.samples" :key="index" class="quality-protocol"><summary>{{ referenceName(sample.lane) }} · {{ t('quality.inspector.checks.' + sample.check_id) }} #{{ sample.pair + 1 }} · {{ t(sample.correct == null ? 'quality.ungraded' : sample.correct ? 'quality.correct' : 'quality.incorrect') }}</summary>
      <p v-if="sample.reason" class="warn">{{ t('quality.audit.referenceChanged') }}</p>
      <pre class="quality-question">{{ sample.prompt }}</pre>
      <template v-if="sample.probe">
        <pre class="quality-question">{{ sample.probe.text || '—' }}</pre>
        <p v-if="sample.probe.usage_anomalies?.length" class="warn">{{ t('quality.assurance.issues.invalid_usage') }}</p>
        <p class="quality-caption">{{ sample.probe.protocol }} · {{ sample.probe.model }} · HTTP {{ sample.probe.http_status }}</p>
        <p>{{ t('quality.input') }} {{ sample.probe.usage_reported ? sample.probe.input_tokens : '—' }} · {{ t('quality.output') }} {{ sample.probe.usage_reported ? sample.probe.output_tokens : '—' }} · {{ t('quality.audit.reasoning') }} {{ sample.probe.reasoning_tokens ?? '—' }} · {{ t('quality.cacheRead') }} {{ sample.probe.cache_reported ? sample.probe.cache_read_tokens : '—' }}</p>
        <p class="quality-caption quality-audit-hash">{{ t('quality.audit.requestHash') }}: {{ sample.probe.request_hash || '—' }}</p>
        <p v-if="!sample.probe.controlled_input" class="warn">{{ t('quality.audit.native') }}</p>
      </template>
    </details>
  </section>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { QualityRun } from './api'
const props = defineProps<{run: QualityRun}>()
const { t } = useI18n()
const audit = computed(() => props.run.result?.audit)
const lanes = computed(() => [...new Set(audit.value?.samples.filter(sample => sample.check_id === 'stability_audit').map(sample => sample.lane))])
function laneSamples(lane: string) { return audit.value?.samples.filter(sample => sample.lane === lane && sample.check_id === 'stability_audit') ?? [] }
function referenceName(lane: string) {
  if (lane === 'primary') return t('quality.audit.primary')
  if (lane === 'repeat') return t('quality.audit.repeat')
  if (lane.startsWith('manual:')) return t('quality.assurance.manualReferences') + ' · ' + (audit.value?.manual_references?.find(reference => reference.id === lane.slice(7))?.name ?? lane.slice(7))
  if (lane.startsWith('history:')) return t('quality.audit.historical') + ' #' + lane.slice(8,16)
  const ref = props.run.inspection?.audit?.references?.find(reference => reference.id === lane)
  return ref ? ref.name + ' · ' + t('quality.assurance.sources.' + ref.source) : lane
}
function signed(value: number) { return (value > 0 ? '+' : '') + value }
function percent(value: number | null) { return value == null ? '—' : signed(Number(value.toFixed(2))) + '%' }
</script>
