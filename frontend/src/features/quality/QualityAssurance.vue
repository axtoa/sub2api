<template>
  <section class="quality-assurance">
    <div class="quality-card-heading"><div><h2>{{ t('quality.assurance.title') }}</h2><p class="quality-caption">{{ t('quality.assurance.intro') }}</p></div><button class="quality-button" :aria-expanded="expanded" @click="expanded = !expanded">{{ t(expanded ? 'quality.assurance.collapse' : 'quality.assurance.expand') }}</button></div>
    <template v-if="expanded">
      <div class="quality-toolbar">
        <label>{{ t('quality.assurance.scenario') }}<select v-model="scenario" @change="refresh"><option v-for="id in scenarios" :key="id" :value="id">{{ t('quality.assurance.scenarios.' + id) }}</option></select></label>
        <label>{{ t('quality.assurance.window') }}<select v-model.number="days" @change="refresh"><option v-for="n in [1,7,30]" :key="n" :value="n">{{ t('quality.assurance.days', {n}) }}</option></select></label>
        <label>{{ t('quality.accountModel') }}<select v-model="cohort"><option value="">{{ t('quality.assurance.allCohorts') }}</option><option v-for="group in cohorts" :key="group.id" :value="group.id">{{ group.label }}</option></select></label>
        <button class="quality-button" :disabled="loading" @click="refresh">{{ t('quality.refresh') }}</button>
        <button class="quality-button" :disabled="!config" :aria-expanded="editing" @click="editing = !editing">{{ t('quality.assurance.rules') }}</button>
        <button class="quality-button" :disabled="!board" @click="exportBoard">{{ t('quality.assurance.export') }}</button>
      </div>
      <p v-if="error" class="quality-message error" role="alert">{{ error }}</p>
      <p v-if="notice" class="quality-caption" role="status">{{ notice }}</p>
      <form v-if="editing && draft" class="quality-form quality-assurance-settings" @submit.prevent="saveRules">
        <h3>{{ t('quality.assurance.rules') }}</h3><p class="quality-caption">{{ t('quality.assurance.ruleHint') }}</p>
        <label>{{ t('quality.assurance.savedScenario') }}<select v-model="draft.policy.scenario"><option v-for="id in scenarios" :key="id" :value="id">{{ t('quality.assurance.scenarios.' + id) }}</option></select></label>
        <div class="quality-form-grid">
          <label v-for="id in scoreDimensions" :key="id">{{ t('quality.assurance.dimensions.' + id) }} (%)<input v-model.number="draft.policy.weights[id]" type="number" min="1" max="80" required /></label>
          <label v-for="field in ruleFields" :key="field.key">{{ t('quality.assurance.fields.' + field.key) }}<input v-model.number="draft.policy[field.key]" type="number" :min="field.min" :max="field.max" required /></label>
        </div>
        <fieldset class="quality-inspection-checks"><legend>{{ t('quality.assurance.required') }}</legend><label v-for="id in requiredChoices" :key="id" class="quality-check"><input v-model="draft.policy.required_checks" type="checkbox" :value="id" />{{ t('quality.inspector.checks.' + id) }}</label></fieldset>
        <h3>{{ t('quality.assurance.references') }}</h3><p class="quality-caption">{{ t('quality.assurance.referenceHint') }}</p>
        <div v-for="reference in draft.references" :key="reference.id" class="quality-form-grid quality-reference-row">
          <label>{{ t('quality.external.name') }}<input v-model="reference.name" maxlength="100" required /></label>
          <label>{{ t('quality.accountModel') }}<select v-model="reference.target_id" required><option value="" disabled>{{ t('quality.assurance.chooseReference') }}</option><option v-for="target in targets" :key="target.id" :value="target.id">{{ target.account_name }} · {{ target.model }}</option></select></label>
          <label>{{ t('quality.assurance.source') }}<select v-model="reference.source"><option v-for="source in ['official','trusted','other']" :key="source" :value="source">{{ t('quality.assurance.sources.' + source) }}</option></select></label>
          <button type="button" class="quality-text-button" @click="draft.references = draft.references.filter(item => item.id !== reference.id)">{{ t('quality.assurance.removeReference') }}</button>
        </div>
        <h3>{{ t('quality.assurance.manualReferences') }}</h3><p class="quality-caption">{{ t('quality.assurance.manualHint') }}</p>
        <div v-for="manual in draft.manual_references" :key="manual.id" class="quality-form-grid quality-reference-row">
          <label>{{ t('quality.assurance.receipt') }}<input v-model="manual.name" maxlength="100" required /></label>
          <label>{{ t('quality.model') }}<input v-model="manual.model" maxlength="100" required /></label>
          <label>{{ t('quality.assurance.protocol') }}<select v-model="manual.protocol"><option value="chat">Chat Completions</option><option value="responses">Responses</option><option value="anthropic">Anthropic Messages</option></select></label>
          <label>{{ t('quality.audit.requestHash') }}<input v-model="manual.request_hash" pattern="[a-f0-9]{64}" required /></label>
          <label>{{ t('quality.assurance.totalInput') }}<input v-model.number="manual.input_tokens" type="number" min="1" max="1000000000" required /></label>
          <label>{{ t('quality.assurance.capturedAt') }}<input :value="manual.captured_at.slice(0,10)" type="date" @change="setDate(manual, $event)" required /></label>
          <button type="button" class="quality-text-button" @click="draft.manual_references = draft.manual_references?.filter(item => item.id !== manual.id)">{{ t('quality.assurance.removeReference') }}</button>
        </div>
        <button type="button" class="quality-button" :disabled="(draft.manual_references?.length ?? 0) >= 50" @click="addManualReference">{{ t('quality.assurance.addManual') }}</button>
        <div class="quality-toolbar"><button type="button" class="quality-button" :disabled="saving" @click="reloadRules">{{ t('quality.assurance.reloadRules') }}</button><button type="button" class="quality-button" :disabled="saving" @click="discardRules">{{ t('quality.assurance.discardRules') }}</button><button class="quality-button" type="button" :disabled="draft.references.length >= 8" @click="addReference">{{ t('quality.assurance.addReference') }}</button><button class="quality-button primary" :disabled="saving || weightTotal !== 100">{{ t('quality.save') }} · {{ weightTotal }} / 100</button></div>
      </form>
      <details v-if="board" class="quality-protocol quality-assessment-batch"><summary>{{ t('quality.assurance.batchTitle') }}</summary>
        <form class="quality-form" @submit.prevent="startBatch">
          <p class="quality-caption">{{ t('quality.assurance.batchHint') }}</p>
          <p v-if="!cohort" class="quality-message warning">{{ t('quality.assurance.selectCohort') }}</p>
          <button type="button" class="quality-button" :disabled="!cohort || batchBusy || batchActive" @click="selectBatch">{{ t('quality.assurance.selectAll') }}</button>
          <fieldset class="quality-inspection-checks"><legend>{{ t('quality.accountModel') }}</legend><label v-for="row in rows" :key="row.target_id" class="quality-check"><input v-model="batchTargets" type="checkbox" :value="row.target_id" :disabled="!cohort || batchBusy || batchActive || (!batchTargets.includes(row.target_id) && batchTargets.length >= 20)" />{{ row.account_name }}</label></fieldset>
          <fieldset class="quality-inspection-checks"><legend>{{ t('quality.inspector.projects') }}</legend><label v-for="id in batchCheckChoices" :key="id" class="quality-check"><input v-model="batchChecks" type="checkbox" :value="id" :disabled="batchBusy || batchActive" />{{ t('quality.inspector.checks.' + id) }}</label></fieldset>
          <p>{{ t('quality.assurance.batchCost', {accounts:batchTargets.length, requests:batchRequestCount}) }}</p>
          <p class="quality-caption">{{ t('quality.assurance.batchLimit') }}</p>
          <button class="quality-button primary" :disabled="!cohort || !batchTargets.length || !batchChecks.length || batchBusy || batchActive">{{ t('quality.assurance.startBatch') }}</button>
          <p v-if="batchJobs.length" role="status">{{ t('quality.assurance.batchProgress', {done:batchJobs.filter(job => job.status === 'finished').length,total:batchJobs.length}) }}</p>
        </form>
      </details>
      <template v-if="board">
        <p class="quality-caption">{{ t('quality.assurance.method', {samples: board.policy.min_samples, runs: board.policy.min_runs, coverage: board.policy.min_coverage, hours: board.policy.fresh_hours, span:board.policy.min_span_minutes}) }}</p>
        <details class="quality-protocol"><summary>{{ t('quality.assurance.formulaTitle') }}</summary><p class="quality-caption">{{ t('quality.assurance.formula') }}</p></details>
        <div class="quality-score-summary"><span v-for="state in ['strong','usable','weak','blocked','review','insufficient','stale']" :key="state">{{ t('quality.assurance.states.' + state) }} <strong>{{ board.items.filter(row => row.state === state).length }}</strong></span></div>
        <div class="quality-score-list">
          <article v-for="row in rows" :key="row.target_id" class="quality-score-card" :class="'score-' + row.state">
            <div class="quality-card-heading">
              <div><strong>{{ row.rank == null ? '—' : '#' + row.rank }} · {{ row.account_name }}</strong><p class="quality-caption">{{ row.model }} · {{ row.cohort.split(':').at(-1) }}</p></div>
              <div class="quality-score-total"><strong>{{ row.score == null ? '—' : row.score.toFixed(1) }}</strong><span :class="tone(row.state)">{{ t('quality.assurance.states.' + row.state) }}</span></div>
            </div>
            <p v-if="row.comparison_set?.length" class="quality-caption">{{ t('quality.assurance.rankingScope') }}: {{ row.comparison_set.map(id => t('quality.assurance.dimensions.' + id)).join(' / ') }}</p>
            <div class="quality-score-dimensions"><div v-for="dimension in row.dimensions" :key="dimension.id">
              <span>{{ t('quality.assurance.dimensions.' + dimension.id) }}</span><strong>{{ dimension.value == null ? '—' : dimension.value.toFixed(1) }}</strong>
              <progress :value="dimension.value ?? 0" max="100" :aria-label="t('quality.assurance.dimensions.' + dimension.id)" />
              <small>{{ t('quality.assurance.samples', {n: dimension.samples}) }}<template v-if="dimension.p95_ms != null"> · P95 {{ dimension.p95_ms }} ms</template></small>
            </div></div>
            <p v-if="row.trend" class="quality-caption">{{ t('quality.assurance.trend', {recent:row.trend.recent_correct, recentTotal:row.trend.recent_samples, previous:row.trend.previous_correct, previousTotal:row.trend.previous_samples}) }}</p>
            <div class="quality-score-meta quality-caption"><strong>{{ t('quality.assurance.coverage', {n:row.coverage}) }}</strong><span>{{ t('quality.assurance.evidenceCount', {samples:row.samples, runs:row.runs}) }}</span><span>{{ date(row.last_at) }}</span><span>{{ t('quality.assurance.span', {n:row.span_minutes ?? 0}) }}</span></div>
            <ul v-if="row.issues.length" class="quality-score-issues"><li v-for="(issue,index) in uniqueIssues(row)" :key="index" :class="issue.severity === 'block' ? 'bad' : 'warn'">{{ t('quality.assurance.severities.' + issue.severity) }} · {{ t('quality.assurance.issues.' + issue.code) }}<template v-if="issue.check"> · {{ t('quality.inspector.checks.' + issue.check) }}</template></li></ul>
            <details class="quality-protocol"><summary>{{ t('quality.assurance.evidence') }}</summary>
              <p class="quality-caption">{{ t('quality.assurance.excluded', {n:row.excluded}) }}</p>
              <div class="quality-toolbar"><button v-for="id in row.run_ids" :key="id" class="quality-text-button" @click="loadRun(id, row.target_id)">#{{ id.slice(0,8) }}</button></div>
              <div v-for="evidence in row.evidence" :key="evidence.id" class="quality-message"><strong>{{ t('quality.assurance.evidenceKinds.' + evidence.kind) }} · {{ t('quality.assurance.evidenceStates.' + evidence.status) }}</strong><p>{{ evidence.note }}</p><button class="quality-text-button" @click="loadRun(evidence.run_id, row.target_id)">{{ t('quality.view') }} #{{ evidence.run_id.slice(0,8) }}</button><p>{{ evidence.receipt }}<template v-if="evidence.kind === 'billing'"> · {{ evidence.expected }} → {{ evidence.actual }} {{ evidence.currency }}</template></p></div>
            </details>
          </article>
          <p v-if="!rows.length" class="quality-caption">{{ t('quality.noData') }}</p>
        </div>
        <p v-for="account in withoutTargets" :key="account.id" class="quality-caption">{{ account.name }} · {{ t(account.unsupported ? 'quality.unsupportedShort' : 'quality.never') }}</p>
        <p class="quality-caption">{{ t('quality.assurance.boundaries') }}</p>
        <p class="quality-caption">{{ t('quality.assurance.generated', {date:date(board.generated_at)}) }}</p>
      </template>
      <p v-else-if="loading" class="quality-caption">{{ t('quality.loading') }}</p>
    </template>
    <BaseDialog :show="!!selectedRun" :title="t('quality.assurance.evidence')" width="wide" panel-class="quality-theme" @close="selectedRun = null">
      <template v-if="selectedRun">
        <QualityEvidence :run="selectedRun" :target="targets.find(target => target.id === selectedTarget) ?? null" />
        <AuditEvidence v-if="selectedRun.result?.audit" :run="selectedRun" />
        <details class="quality-protocol"><summary>{{ t('quality.assurance.recordEvidence') }}</summary>
          <form class="quality-form" @submit.prevent="saveEvidence">
            <p class="quality-caption">{{ t('quality.assurance.ledgerHint') }}</p>
            <div class="quality-form-grid">
              <label>{{ t('quality.assurance.kind') }}<select v-model="evidenceDraft.kind"><option v-for="kind in ['billing','identity','privacy']" :key="kind" :value="kind">{{ t('quality.assurance.evidenceKinds.' + kind) }}</option></select></label>
              <label>{{ t('quality.assurance.conclusion') }}<select v-model="evidenceDraft.status"><option v-for="status in ['suspected','confirmed','cleared']" :key="status" :value="status">{{ t('quality.assurance.evidenceStates.' + status) }}</option></select></label>
              <label>{{ t('quality.assurance.receipt') }}<input v-model="evidenceDraft.receipt" maxlength="500" required /></label>
              <template v-if="evidenceDraft.kind === 'billing'">
                <label>{{ t('quality.assurance.currency') }}<input v-model="evidenceDraft.currency" maxlength="3" pattern="[A-Z]{3}" required /></label>
                <label>{{ t('quality.assurance.expected') }}<input v-model.number="evidenceDraft.expected" type="number" min="0.000000001" max="1000000000" step="any" required /></label>
                <label>{{ t('quality.assurance.actual') }}<input v-model.number="evidenceDraft.actual" type="number" min="0" max="1000000000" step="any" required /></label>
              </template>
            </div>
            <label>{{ t('quality.assurance.note') }}<textarea v-model="evidenceDraft.note" maxlength="2000" required /></label>
            <label v-if="evidenceDraft.kind === 'billing'" class="quality-check"><input v-model="evidenceDraft.attributable" type="checkbox" required />{{ t('quality.assurance.attributable') }}</label>
            <p v-if="evidenceError" class="quality-message error" role="alert">{{ evidenceError }}</p>
            <button class="quality-button primary" :disabled="saving">{{ t('quality.assurance.appendEvidence') }}</button>
          </form>
        </details>
      </template>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import QualityEvidence from './QualityEvidence.vue'
import AuditEvidence from './AuditEvidence.vue'
import { qualityAPI, scoreDimensions, type ManualTokenReference, type AssuranceSettings, type ScoreBoard, type ScorePolicy, type QualityTarget, type MonitoredAccount, type QualityScore, type InspectionCheckID, type QualityRun, type AssuranceEvidence } from './api'
const props = defineProps<{targets: QualityTarget[]; accounts: MonitoredAccount[]}>()
const { t } = useI18n()
const expanded = ref(true), editing = ref(false), loading = ref(false), saving = ref(false)
const error = ref(''), notice = ref(''), config = ref<AssuranceSettings | null>(null), draft = ref<AssuranceSettings | null>(null), board = ref<ScoreBoard | null>(null)
const days = ref(7), scenario = ref<ScorePolicy['scenario']>('chat'), cohort = ref('')
const batchTargets = ref<string[]>([]), batchChecks = ref<InspectionCheckID[]>(['protocol_stream','reasoning','speed','cache']), batchBusy = ref(false), batchJobs = ref<QualityRun[]>([])
const batchCheckChoices: InspectionCheckID[] = ['protocol_stream','protocol_nonstream','tools','structured','reasoning','speed','cache','context','vision','usage_audit','stability_audit']
const batchActive = computed(() => batchJobs.value.some(job => job.status !== 'finished'))
const batchRequestCount = computed(() => batchTargets.value.length * batchChecks.value.reduce((sum,id) => sum + (id === 'tools' || id === 'reasoning' || id === 'speed' ? 2 : id === 'cache' ? 3 : id === 'usage_audit' ? 6 : id === 'stability_audit' ? 9 : 1),0))
let batchTimer: ReturnType<typeof setTimeout> | undefined
function selectBatch() { batchTargets.value = rows.value.slice(0,20).map(row => row.target_id) }
async function startBatch() {
  if (!cohort.value || batchBusy.value || batchActive.value || !batchTargets.value.length || !batchChecks.value.length) return
  const ids = batchTargets.value.filter(id => rows.value.some(row => row.target_id === id)).slice(0,20), checks = [...batchChecks.value], seed = crypto.randomUUID()
  if (!ids.length) return
  batchBusy.value = true; batchJobs.value = []; let failed = 0
  for (const id of ids) {
    if (disposed) break
    try { const audit = checks.some(check => check === 'usage_audit' || check === 'stability_audit') ? {seed} : undefined; batchJobs.value.push(await qualityAPI.inspect(id, {mode:'quick',checks,...(audit ? {audit} : {})})) }
    catch { failed++ }
  }
  batchBusy.value = false; notice.value = t('quality.assurance.batchSubmitted', {queued:batchJobs.value.length,failed}); pollBatch()
}
function pollBatch() {
  clearTimeout(batchTimer)
  if (disposed || !batchActive.value) return
  batchTimer = setTimeout(async () => {
    if (document.hidden) {pollBatch(); return}
    try { const jobs = await qualityAPI.statuses(batchJobs.value.filter(job => job.status !== 'finished').map(job => job.id)); if (disposed) return; batchJobs.value = batchJobs.value.map(job => jobs.find(current => current.id === job.id) ?? job); if (!batchActive.value) await refresh() }
    catch { if (!disposed) error.value = t('quality.loadError') }
    if (!disposed) pollBatch()
  },3000)
}

const scenarios = ['chat','agent','cost'] as const
const requiredChoices: InspectionCheckID[] = ['protocol_stream','protocol_nonstream','tools','structured','context','vision','usage_audit']
const ruleFields: Array<{key:'min_samples'|'min_runs'|'min_span_minutes'|'min_coverage'|'fresh_hours'|'slow_ms'|'failure_percent'|'usage_tolerance_percent'|'billing_tolerance_percent';min:number;max:number}> = [
  {key:'min_samples',min:5,max:100}, {key:'min_runs',min:2,max:20}, {key:'min_span_minutes',min:0,max:1440}, {key:'min_coverage',min:30,max:100}, {key:'fresh_hours',min:1,max:168}, {key:'slow_ms',min:100,max:120000}, {key:'failure_percent',min:5,max:80}, {key:'usage_tolerance_percent',min:1,max:100}, {key:'billing_tolerance_percent',min:0,max:100},
]
const cohorts = computed(() => [...new Map((board.value?.items ?? []).map(row => [row.cohort, {id:row.cohort,label:row.model + ' · ' + row.platform + ' · ' + row.cohort.split(':').at(-1)}])).values()])
const rows = computed(() => (board.value?.items ?? []).filter(row => !cohort.value || row.cohort === cohort.value).slice().sort((a,b) => a.cohort.localeCompare(b.cohort) || (a.rank ?? 100000) - (b.rank ?? 100000)))
const withoutTargets = computed(() => props.accounts.filter(account => !props.targets.some(target => target.account_id === account.id)))
const weightTotal = computed(() => Object.values(draft.value?.policy.weights ?? {}).reduce((sum,n) => sum + Number(n),0))
const selectedRun = ref<QualityRun | null>(null), selectedTarget = ref(''), evidenceError = ref('')
const evidenceDraft = ref<AssuranceEvidence>(emptyEvidence())
let controller: AbortController | undefined, revision = 0, disposed = false, timer: ReturnType<typeof setTimeout> | undefined
function emptyEvidence(): AssuranceEvidence { return {run_id:'',kind:'billing',status:'suspected',note:'',receipt:'',currency:'USD',expected:0,actual:0,attributable:false} }
function date(value: string | null) { return value ? new Date(value).toLocaleString() : '—' }
function tone(state: string) { return state === 'blocked' || state === 'weak' ? 'bad' : state === 'strong' || state === 'usable' ? 'good' : 'warn' }
function uniqueIssues(row: QualityScore) { return [...new Map(row.issues.map(issue => [issue.code + issue.check, issue])).values()] }
async function refresh() {
  const current = ++revision; clearTimeout(timer); controller?.abort(); controller = new AbortController()
  loading.value = true
  try { const result = await qualityAPI.scores(days.value, scenario.value, controller.signal); if (disposed || current !== revision) return; board.value = result; error.value = '' }
  catch { if (!disposed && current === revision) error.value = t('quality.loadError') }
  finally { if (!disposed && current === revision) { loading.value = false; timer = setTimeout(() => { if (!document.hidden && expanded.value) void refresh(); else schedule() },60000) } }
}
function schedule() { clearTimeout(timer); if (!disposed) timer = setTimeout(() => { if (!document.hidden && expanded.value) void refresh(); else schedule() },60000) }
function setDate(manual: ManualTokenReference, event: Event) { const value = (event.target as HTMLInputElement).value; manual.captured_at = value ? new Date(value + 'T00:00:00Z').toISOString() : '' }
function addManualReference() { if (draft.value) (draft.value.manual_references ??= []).push({id:crypto.randomUUID(),name:'',model:'',protocol:'chat',request_hash:'',input_tokens:1,captured_at:new Date().toISOString()}) }
function addReference() { draft.value?.references.push({id:crypto.randomUUID(),target_id:'',name:'',source:'trusted'}) }
function discardRules() { if (config.value) draft.value = JSON.parse(JSON.stringify(config.value)); error.value = '' }
async function reloadRules() {
  if (saving.value) return
  saving.value = true
  try { config.value = await qualityAPI.assurance(); discardRules() }
  catch { error.value = t('quality.loadError') }
  finally { saving.value = false }
}
async function saveRules() {
  if (!draft.value || saving.value || weightTotal.value !== 100) return
  saving.value = true; error.value = ''
  try { config.value = await qualityAPI.saveAssurance(draft.value); draft.value = JSON.parse(JSON.stringify(config.value)); scenario.value = config.value.policy.scenario; notice.value = t('quality.assurance.saved'); await refresh() }
  catch { error.value = t('quality.assurance.saveError') }
  finally { saving.value = false }
}
async function loadRun(id: string, targetID: string) {
  try { selectedRun.value = await qualityAPI.detail(id); selectedTarget.value = targetID; evidenceDraft.value = {...emptyEvidence(),run_id:id}; evidenceError.value = '' }
  catch { error.value = t('quality.loadError') }
}
async function saveEvidence() {
  if (saving.value) return
  saving.value = true; evidenceError.value = ''
  try { await qualityAPI.saveEvidence(evidenceDraft.value); selectedRun.value = null; notice.value = t('quality.assurance.evidenceSaved'); await refresh() }
  catch { evidenceError.value = t('quality.assurance.evidenceError') }
  finally { saving.value = false }
}
function exportBoard() {
  if (!board.value) return
  const url = URL.createObjectURL(new Blob([JSON.stringify(board.value,null,2)],{type:'application/json'})), link = document.createElement('a')
  link.href = url; link.download = 'upstream-quality-' + days.value + 'd.json'; link.click(); setTimeout(() => URL.revokeObjectURL(url),1000)
}
onMounted(async () => {
  try { const value = await qualityAPI.assurance(); if (disposed) return; config.value = value; draft.value = structuredClone(value); scenario.value = value.policy.scenario }
  catch { if (!disposed) error.value = t('quality.loadError') }
  if (!disposed) await refresh()
})
watch(cohort, () => { batchTargets.value = [] })
onUnmounted(() => { clearTimeout(batchTimer); disposed = true; revision++; controller?.abort(); clearTimeout(timer) })
</script>
