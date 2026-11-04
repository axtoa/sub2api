<template>
  <section class="quality-inspection">
    <p class="quality-caption">{{ t('quality.inspector.intro') }}</p>
    <form class="quality-form" @submit.prevent="start">
      <div class="quality-form-grid">
        <label>{{ t('quality.accountModel') }}<select v-model="targetID" :disabled="submitting"><option v-for="target in targets" :key="target.id" :value="target.id">{{ target.account_name }} · {{ target.model }}</option></select></label>
        <label>{{ t('quality.inspector.mode') }}<select v-model="mode" :disabled="submitting" @change="choosePreset"><option value="quick">{{ t('quality.inspector.quick') }}</option><option value="full">{{ t('quality.inspector.full') }}</option></select></label>
      </div>
      <fieldset class="quality-inspection-checks"><legend>{{ t('quality.inspector.projects') }}</legend><label v-for="id in checkIDs" :key="id" class="quality-check"><input v-model="checks" :value="id" type="checkbox" :disabled="submitting" />{{ t(`quality.inspector.checks.${id}`) }}</label></fieldset>
      <div class="quality-custom-entry">
        <label class="quality-check"><input v-model="customEnabled" type="checkbox" :disabled="submitting" />{{ t('quality.custom.enable') }}</label>
        <template v-if="customEnabled">
          <p class="quality-caption">{{ t('quality.custom.reuseHint') }}</p>
          <button v-if="settings.inspection_questions?.length" type="button" class="quality-text-button" :disabled="submitting" @click="loadSavedQuestions">{{ t('quality.custom.loadSaved') }}</button>
          <InspectionQuestionEditor v-model="questions" :disabled="submitting" />
        </template>
      </div>
      <p class="quality-caption">{{ t('quality.inspector.cost', { count: requestCount, output: profile.max_output_tokens, prefix: profile.cache_prefix_kb }) }}</p>
      <p v-if="checks.includes('context')" class="quality-caption">{{ t('quality.inspector.contextHint', { kb: mode === 'full' ? 32 : 8 }) }}</p>
      <p class="quality-caption">{{ t('quality.inspector.billingHint') }}</p>
      <button class="quality-button primary" :disabled="submitting || !targetID || (!checks.length && !customEnabled) || (customEnabled && !questions.length) || !!active">{{ t(submitting ? 'quality.loading' : active ? 'quality.running' : 'quality.inspector.start') }}</button>
    </form>
    <p v-if="error" class="quality-message error" role="alert">{{ error }}</p>
    <p v-if="active" class="quality-message" role="status">{{ t('quality.inspector.pending') }}</p>
    <div class="quality-inspection-reports">
      <div class="quality-card-heading"><h2>{{ t('quality.inspector.reports') }}</h2><button class="quality-text-button" :disabled="loading" @click="refreshReports()">{{ t('quality.refresh') }}</button></div>
      <p v-if="!reports.length" class="quality-caption">{{ t(loading ? 'quality.loading' : 'quality.noHistory') }}</p>
      <div v-else class="quality-history-list"><button v-for="report in reports" :key="report.id" class="quality-history-row" :aria-pressed="run?.id === report.id" @click="selectReport(report.id)"><span>{{ date(report.created_at) }} · {{ t(`quality.inspector.${report.inspection?.mode || 'quick'}`) }}<small>#{{ report.id.slice(0, 8) }}</small></span><span :class="runTone(report)">{{ t(`quality.${verdictKey(report)}`) }}</span></button></div>
    </div>
    <article v-if="run" class="quality-inspection-report">
      <div class="quality-card-heading"><strong :class="runTone(run)">{{ t(`quality.${verdictKey(run)}`) }}</strong><button v-if="run.status === 'finished'" class="quality-text-button" @click="exportReport">{{ t('quality.inspector.export') }}</button></div>
      <p class="quality-caption">{{ reportTarget?.account_name }} · {{ reportTarget?.model }} · {{ date(run.created_at) }} · #{{ run.id }}</p>
      <p v-if="run.result">{{ t('quality.inspector.summary', { requests: run.result.requests, planned: run.result.planned, tokens: run.result.budget_tokens, duration: (run.result.duration_ms / 1000).toFixed(1) }) }}</p>
      <p v-if="run.result?.reason" class="quality-message">{{ reason(run.result.reason) }}</p>
      <p class="quality-caption">{{ t('quality.inspector.identityHint') }}</p>
      <section v-for="check in run.result?.checks ?? []" :key="check.id" class="quality-inspection-result">
        <div class="quality-card-heading"><h3>{{ t(`quality.inspector.checks.${check.id}`) }}</h3><strong :class="check.verdict === 'passed' ? 'good' : check.verdict === 'failed' ? 'bad' : 'warn'">{{ t(`quality.inspector.verdicts.${check.verdict}`) }}</strong></div>
        <p v-if="check.reason" class="quality-caption">{{ reason(check.reason) }}</p>
        <p class="quality-caption">{{ t('quality.inspector.requests', { count: check.requests }) }}<span v-if="check.median_first_token_ms != null"> · {{ t('quality.inspector.median') }} {{ check.median_first_token_ms }} ms</span><span v-if="check.tokens_per_second != null"> · {{ check.tokens_per_second.toFixed(1) }} token/s</span><span v-if="check.cache_ratio != null"> · {{ t('quality.validationRatio') }} {{ check.cache_ratio.toFixed(2) }}%</span></p>
        <p v-if="check.id === 'speed'" class="quality-caption">{{ t('quality.inspector.speedHint') }}</p>
        <details v-for="sample in samples(check.id)" :key="sample.question_id" class="quality-protocol"><summary>{{ sample.name || t('quality.view') }} · <template v-if="check.id === 'custom'">{{ t(sample.correct == null ? sample.grade_reason === 'manual_review' ? 'quality.custom.review' : 'quality.ungraded' : sample.correct ? 'quality.correct' : 'quality.incorrect') }} · </template>HTTP {{ sample.probe.http_status }} · {{ sample.probe.duration_ms }} ms</summary>
          <p v-if="sample.match" class="quality-caption">{{ t('quality.custom.match') }}: {{ t(`quality.custom.${sample.match}`) }}</p>
          <p class="quality-caption">{{ sample.probe.protocol || '—' }} · {{ t('quality.actualModel') }} {{ sample.probe.model || '—' }}</p>
          <img v-if="sample.image_data_url" :src="sample.image_data_url" :alt="t('quality.inspector.checks.vision')" class="quality-inspection-image" />
          <h3>{{ t('quality.question') }}</h3><pre class="quality-question">{{ sample.prompt }}</pre>
          <template v-if="sample.expected"><h3>{{ t(sample.match === 'manual' ? 'quality.custom.reference' : sample.match === 'contains' ? 'quality.custom.keywords' : 'quality.expected') }}</h3><pre class="quality-question">{{ sample.expected }}</pre></template>
          <h3>{{ t('quality.answer') }}</h3><pre class="quality-question">{{ sample.probe.text || '—' }}</pre>
          <pre v-if="sample.probe.tool_calls?.length" class="quality-question">{{ JSON.stringify(sample.probe.tool_calls, null, 2) }}</pre>
          <p v-if="sample.probe.reason" class="quality-caption">{{ reason(sample.probe.reason) }} {{ sample.probe.error_param || '' }}</p>
          <p class="quality-caption">{{ t('quality.usage') }}: {{ t('quality.input') }} {{ sample.probe.input_tokens }} / {{ t('quality.output') }} {{ sample.probe.output_tokens }} / {{ t('quality.cacheRead') }} {{ sample.probe.cache_read_tokens }}</p>
          <p v-if="!sample.probe.usage_reported" class="quality-caption">{{ t('quality.usageMissing') }}</p>
        </details>
      </section>
      <details v-if="run.result?.cache_prefix" class="quality-protocol"><summary>{{ t('quality.cachePrefix') }}</summary><pre class="quality-question">{{ run.result.cache_prefix }}</pre></details>
      <p v-if="run.profile_hash" class="quality-caption">{{ t('quality.fingerprint') }}: {{ run.profile_hash }}</p>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import InspectionQuestionEditor from './InspectionQuestionEditor.vue'
import { qualityAPI, type InspectionCheckID, type InspectionOptions, type InspectionQuestion, type QualityRun, type QualitySettings, type QualityTarget } from './api'
import { runTone, verdictKey } from './display'

const props = defineProps<{ targets: QualityTarget[]; settings: QualitySettings }>()
const { t, te } = useI18n()
const checkIDs: InspectionCheckID[] = ['protocol_stream', 'protocol_nonstream', 'tools', 'structured', 'reasoning', 'speed', 'cache', 'context', 'vision']
const mode = ref<InspectionOptions['mode']>('quick')
const checks = ref<InspectionCheckID[]>(checkIDs.slice(0, 6))
const customEnabled = ref(false), questions = ref<InspectionQuestion[]>([])
const targetID = ref(props.targets[0]?.id || '')
const reports = ref<QualityRun[]>([]), run = ref<QualityRun | null>(null)
const submitting = ref(false), loading = ref(false), error = ref('')
const profile = computed(() => props.targets.find(target => target.id === targetID.value)?.overrides ?? props.settings)
const reportTarget = computed(() => props.targets.find(target => target.id === run.value?.target_id))
const active = computed(() => reports.value.find(report => report.status !== 'finished'))
const requestCount = computed(() => checks.value.reduce((sum, id) => sum + (id === 'tools' ? 2 : id === 'cache' ? 3 : id === 'reasoning' ? mode.value === 'full' ? 4 : 2 : id === 'speed' ? mode.value === 'full' ? 5 : 2 : 1), 0) + (customEnabled.value ? questions.value.length : 0))
let timer: ReturnType<typeof setTimeout> | undefined, disposed = false, revision = 0, controller: AbortController | undefined
function choosePreset() { checks.value = mode.value === 'full' ? [...checkIDs] : checkIDs.slice(0, 6) }
function loadSavedQuestions() { questions.value = (props.settings.inspection_questions ?? []).map(question => ({ ...question })) }
function date(value: string) { return new Date(value).toLocaleString() }
function reason(code: string) { return code === 'native_protocol' ? t('quality.inspector.nativeProtocol') : te(`quality.reasons.${code}`) ? t(`quality.reasons.${code}`) : t('quality.unknown') }
function samples(id: InspectionCheckID) { return run.value?.result?.items?.filter(item => item.check_id === id) ?? [] }
function poll() { clearTimeout(timer); if (!disposed && active.value && !document.hidden) timer = setTimeout(() => refreshReports(), 3000) }
async function refreshReports(preferred?: string) {
  const current = ++revision, id = targetID.value
  clearTimeout(timer); controller?.abort(); controller = new AbortController(); const signal = controller.signal; loading.value = true
  try {
    const items = await qualityAPI.history(id, 'inspection', signal)
    if (disposed || current !== revision) return
    reports.value = items
    const selected = preferred || (run.value?.target_id === id ? run.value.id : items[0]?.id)
    const detail = selected ? await qualityAPI.detail(selected, signal) : null
    if (disposed || current !== revision) return
    run.value = detail
    error.value = ''
  } catch { if (!disposed && current === revision && !signal.aborted) error.value = t('quality.loadError') }
  finally { if (!disposed && current === revision) { loading.value = false; poll() } }
}
async function selectReport(id: string) { await refreshReports(id) }
async function start() {
  if (submitting.value || active.value || !targetID.value || (!checks.value.length && !customEnabled.value)) return
  if (customEnabled.value && (!questions.value.length || questions.value.some(q => !q.name.trim() || !q.prompt.trim() || (q.match !== 'manual' && !q.expected.trim())))) { error.value = t('quality.custom.invalid'); return }
  submitting.value = true; error.value = ''
  const id = targetID.value
  try {
    const options: InspectionOptions = { mode: mode.value, checks: [...checks.value] }
    if (customEnabled.value) { options.checks.push('custom'); options.questions = questions.value.map(q => ({ ...q })) }
    const job = await qualityAPI.inspect(id, options)
    if (disposed || targetID.value !== id) return
    reports.value.unshift(job); run.value = job; await refreshReports(job.id)
  } catch { if (!disposed) error.value = t('quality.inspector.submitError') }
  finally { submitting.value = false }
}
function exportReport() {
  if (!run.value) return
  const blob = new Blob([JSON.stringify({ account: reportTarget.value?.account_name, model: reportTarget.value?.model, report: run.value }, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob), link = document.createElement('a')
  link.href = url; link.download = `channel-inspection-${run.value.id}.json`; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000)
}
function visibilityChanged() { if (document.hidden) clearTimeout(timer); else if (active.value) void refreshReports() }
watch(targetID, () => { run.value = null; reports.value = []; if (targetID.value) void refreshReports() }, { immediate: true })
document.addEventListener('visibilitychange', visibilityChanged)
onUnmounted(() => { disposed = true; revision++; clearTimeout(timer); controller?.abort(); document.removeEventListener('visibilitychange', visibilityChanged) })
</script>
