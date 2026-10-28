<template>
  <AppLayout>
    <main class="quality-theme quality-page" data-density="compact">
      <header class="quality-header"><div><h1>{{ t('quality.title') }}<span class="quality-live-dot" /></h1><p class="quality-caption quality-subtitle">{{ t('quality.subtitle') }}</p></div><div class="quality-toolbar">
        <div class="quality-batch-controls"><select v-model="batchKind" :aria-label="t('quality.batchKind')"><option value="availability">{{ t('quality.runAvailability') }}</option><option value="quality">{{ t('quality.runQuality') }}</option><option value="cache">{{ t('quality.runCache') }}</option></select><button class="quality-button primary" :disabled="!batchTargets.length" @click="runBatch"><Icon name="play" size="sm" />{{ t('quality.batchRun') }} <span>{{ batchTargets.length }}</span></button></div>
        <button class="quality-button" :disabled="syncing" @click="syncAccounts(false)"><Icon name="refresh" size="sm" />{{ t(syncing ? 'quality.syncing' : 'quality.sync') }}</button><button class="quality-button quality-icon-button" :disabled="!snapshot" :title="t('quality.settings')" :aria-label="t('quality.settings')" @click="openSettings"><Icon name="cog" size="sm" /></button>
      </div></header>
      <p v-if="error" class="quality-message error" role="alert">{{ error }}</p>
      <p v-if="notice" class="quality-caption" role="status" aria-live="polite">{{ notice }}</p>
      <template v-if="snapshot">
        <div class="quality-overview"><span>{{ t('quality.accountTotal') }} <strong>{{ accounts.length }}</strong></span><span>{{ t('quality.autoEnabled') }} <strong>{{ snapshot.settings.auto_enabled ? accounts.filter(a => a.enabled && !a.unsupported && accountTargets(a.id).some(t=>t.enabled)).length : 0 }}</strong></span><span>{{ t('quality.attention') }} <strong>{{ accounts.filter(accountAttention).length }}</strong></span><span>{{ t('quality.paused') }} <strong>{{ accounts.filter(a => !a.enabled).length }}</strong></span></div>
        <p v-if="!snapshot.settings.auto_enabled" class="quality-caption">{{ t('quality.automaticOff') }}</p>
        <p v-if="snapshot.budget.limited" class="quality-message warning">{{ t('quality.budgetLimited') }}</p>
        <div v-if="batchJobs.length" class="quality-batch-progress" role="status" aria-live="polite"><div><strong>{{ t('quality.batchProgress', {done:batchFinished,total:batchJobs.length}) }}</strong><span class="quality-caption">{{ t('quality.batchWaiting',{count:batchJobs.filter(j=>j.status!=='finished').length}) }}</span><button class="quality-text-button" @click="batchJobs=[]">{{ t('quality.hideProgress') }}</button></div><progress :value="batchFinished" :max="batchJobs.length" /><p v-if="batchFailed" class="quality-caption">{{ t('quality.batchFailures',{count:batchFailed}) }}</p></div>
        <div class="quality-filters">
          <details ref="groupPicker" class="quality-group-picker"><summary>{{ selectedGroupName }}<Icon name="chevronDown" size="sm" /></summary><div class="quality-group-popover"><input v-model="groupSearch" :placeholder="t('quality.searchGroups')" :aria-label="t('quality.searchGroups')" /><div class="quality-group-options"><button v-for="group in visibleGroups" :key="group.id" :aria-pressed="groupID === group.id" @click="chooseGroup(group.id)"><span>{{ group.name }}</span><span>{{ groupCount(group.id) }}</span></button><p v-if="!visibleGroups.length" class="quality-caption">{{ t('quality.noMatches') }}</p></div></div></details>
          <input v-model="search" :placeholder="t('quality.search')" :aria-label="t('quality.search')" />
          <select v-model="filter" :aria-label="t('quality.allStatuses')"><option value="all">{{ t('quality.allStatuses') }}</option><option value="attention">{{ t('quality.onlyAttention') }}</option><option value="paused">{{ t('quality.paused') }}</option><option value="new">{{ t('quality.never') }}</option><option value="unsupported">{{ t('quality.unsupportedShort') }}</option></select>
        </div>
        <div class="quality-filter-result quality-caption"><span>{{ t('quality.showingAccounts', { count: visible.length, total: accounts.length }) }}</span><button class="quality-text-button" :disabled="loading" :title="t('quality.refreshResults')" :aria-label="t('quality.refreshResults')" @click="refresh"><Icon name="refresh" size="sm" /></button></div>
        <div class="quality-account-grid"><QualityAccountCard v-for="account in visible" :key="account.id" :account="account" :targets="accountTargets(account.id)" :target="selectedTarget(account.id)" :saving="savingAccounts.has(account.id)" :settings="snapshot.settings" :now="serverNow" @toggle="toggleAccount(account, $event)" @model="selectedModels[account.id] = $event" @view="(mode, id) => openAccount(account, mode, id)" @run="runAccount(account, $event)" /><p v-if="!visible.length" class="quality-empty">{{ t(accounts.length ? 'quality.noMatches' : 'quality.emptyDescription') }}</p></div>
        <div class="quality-legend quality-caption"><span><i class="good" />{{ t('quality.passedShort') }}</span><span><i class="bad" />{{ t('quality.unavailable') }}</span><span><i class="warn" />{{ t('quality.incompleteShort') }}</span><span><i />{{ t('quality.noData') }}</span></div>
      </template>
      <p v-else-if="loading || syncing" class="quality-caption" role="status">{{ t('quality.loading') }}</p>
    </main>

    <BaseDialog :show="dialog === 'settings'" :title="t('quality.settings')" width="wide" panel-class="quality-theme" @close="closeDialog">
      <form v-if="settingsDraft && snapshot" id="quality-settings" class="quality-form" @submit.prevent="saveSettings"><label class="quality-check"><input v-model="settingsDraft.auto_enabled" type="checkbox" />{{ t('quality.autoEnabled') }}</label><p class="quality-caption">{{ t('quality.autoHint') }}</p><QualityProfileForm :model-value="settingsDraft" @update:model-value="Object.assign(settingsDraft!, $event)" /><QualityQuestionBank :model-value="settingsDraft.question_bank ?? []" @update:model-value="settingsDraft.question_bank=$event" /><h2>{{ t('quality.budget') }}</h2><div class="quality-form-grid"><label>{{ t('quality.requestLimit') }}<input v-model.number="settingsDraft.daily_requests" type="number" min="1" max="100000" step="1" required /></label><label>{{ t('quality.tokenLimit') }}<input v-model.number="settingsDraft.daily_tokens" type="number" min="1000" max="1000000000" step="1" required /></label></div><p class="quality-caption">{{ t('quality.budgetHint', { timezone: snapshot.budget.timezone }) }}</p><p>{{ t('quality.today') }} {{ number(snapshot.budget.requests) }} / {{ number(snapshot.settings.daily_requests) }} · {{ t('quality.tokens') }} {{ number(snapshot.budget.tokens) }} / {{ number(snapshot.settings.daily_tokens) }}</p><p class="quality-caption">{{ t('quality.manualUsage', { count: number(snapshot.budget.manual_requests), tokens: number(snapshot.budget.manual_tokens) }) }}</p><p v-if="snapshot.budget.unconfirmed_requests + snapshot.budget.manual_unconfirmed" class="quality-caption">{{ t('quality.reserved', { count: snapshot.budget.unconfirmed_requests + snapshot.budget.manual_unconfirmed }) }}</p><p class="quality-caption">{{ t('quality.forecast', { count: draftForecast }) }}</p><p v-if="draftForecast>settingsDraft.daily_requests" class="quality-message warning">{{ t('quality.forecastExceeds') }}</p><p class="quality-caption">{{ t('quality.referenceHint') }}</p><p v-if="dialogError" class="quality-message error" role="alert">{{ dialogError }}</p></form>
      <template #footer><button class="quality-button" :disabled="busy" @click="restoreDefaults">{{ t('quality.defaults') }}</button><button class="quality-button primary" type="submit" form="quality-settings" :disabled="busy">{{ t('quality.save') }}</button></template>
    </BaseDialog>

    <BaseDialog :show="dialog === 'account'" :title="drawerAccount?.name || t('quality.details')" panel-class="quality-theme quality-detail-drawer" :close-on-click-outside="true" @close="closeDialog">
      <template v-if="drawerAccount"><p class="quality-caption">{{ drawerAccount.platform }} · {{ drawerTarget?.model || t('quality.reasons.unsupported') }}</p><select v-if="accountTargets(drawerAccount.id).length > 1" :value="drawerTarget?.id" :aria-label="t('quality.model')" class="quality-drawer-model" @change="changeDrawerModel(($event.target as HTMLSelectElement).value)"><option v-for="target in accountTargets(drawerAccount.id)" :key="target.id" :value="target.id">{{ target.model }}</option></select>
        <nav class="quality-detail-tabs" :aria-label="t('quality.details')"><button v-for="tab in detailTabs" :key="tab" class="quality-button" :aria-pressed="detailMode === tab" @click="setDetailMode(tab)">{{ t(`quality.${tab === 'settings' ? 'edit' : tab}`) }}</button></nav>
        <p v-if="dialogError" class="quality-message error" role="alert">{{ dialogError }}</p>
        <template v-if="detailMode === 'settings'">
          <form v-if="drawerTarget && targetProfile" id="quality-target" class="quality-form" @submit.prevent="saveTarget"><label class="quality-check"><input v-model="targetEnabled" type="checkbox" />{{ t('quality.enabled') }}</label><label class="quality-check"><input v-model="inherit" type="checkbox" />{{ t('quality.inheritToggle') }}</label><p class="quality-caption">{{ t('quality.overridesHint') }}</p><QualityProfileForm v-if="!inherit" v-model="targetProfile" :advanced-open="true" /><button class="quality-button primary" :disabled="busy">{{ t('quality.save') }}</button></form>
          <form class="quality-add-model" @submit.prevent="addModel"><label>{{ t('quality.addModel') }}<input v-model="newModel" maxlength="100" :placeholder="t('quality.modelPlaceholder')" required /></label><button class="quality-button" :disabled="busy">{{ t('quality.add') }}</button></form>
        </template>
        <template v-else>
          <div v-if="detailMode === 'availability' && drawerTarget" class="quality-probe-summary"><div><span>{{ t('quality.probeRate') }}</span><strong>{{ probeRate(drawerTarget) == null ? '—' : `${probeRate(drawerTarget)!.toFixed(2)}%` }}</strong></div><div><span>{{ t('quality.ttft') }}</span><strong>{{ firstToken(drawerTarget) == null ? '—' : `${firstToken(drawerTarget)} ms` }}</strong></div><p class="quality-caption">{{ t('quality.trafficHint') }}</p></div>
          <div v-if="detailMode === 'cache' && drawerTarget" class="quality-probe-summary"><div><span>{{ t('quality.billedCache') }}</span><strong>{{ number(drawerTarget.metrics.billed_cache_tokens) }} token</strong></div><p class="quality-caption">{{ t('quality.cacheHint') }}</p></div><template v-if="detailMode === 'history'"><p v-if="!history.length && !detailLoading" class="quality-caption">{{ t('quality.noHistory') }}</p><div class="quality-history-list"><button v-for="item in history" :key="item.id" class="quality-history-row" :aria-pressed="detail?.id === item.id" @click="fetchDetail(item.id)"><span>{{ date(item.created_at) }} · {{ t(`quality.${item.kind}`) }}<small>{{ t(item.manual ? 'quality.manual' : 'quality.automatic') }} · #{{ item.id.slice(0,8) }}</small></span><span :class="runTone(item)">{{ t(`quality.${verdictKey(item)}`) }}</span></button></div></template>
          <p v-if="detailLoading" class="quality-caption" role="status">{{ t('quality.loading') }}</p><QualityEvidence v-else-if="detail" :run="detail" :target="drawerTarget" /><p v-else-if="detailMode !== 'history'" class="quality-caption">{{ t('quality.never') }}</p>
          <div class="quality-drawer-actions"><button v-if="drawerTarget" class="quality-button primary" :disabled="drawerAccount.unsupported" @click="runAccount(drawerAccount, detailMode === 'history' ? 'quality' : detailMode)"><Icon name="play" size="sm" />{{ t('quality.run') }} · {{ t(`quality.${detailMode === 'history' ? 'quality' : detailMode}`) }}</button><button v-if="detailMode !== 'history'" class="quality-text-button" @click="setDetailMode('history')">{{ t('quality.history') }}</button></div>
        </template>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import QualityProfileForm from './QualityProfileForm.vue'
import QualityQuestionBank from './QualityQuestionBank.vue'
import QualityAccountCard from './QualityAccountCard.vue'
import QualityEvidence from './QualityEvidence.vue'
import { attention, profileOf, qualityAPI } from './api'
import type { CheckKind, MonitoredAccount, QualityProfile, QualityRun, QualitySettings, QualitySnapshot } from './api'
import { firstToken, probeRate, runTone, verdictKey, isStale } from './display'
import './quality.css'

type DetailMode = CheckKind | 'history' | 'settings'
const { t } = useI18n()
const snapshot = ref<QualitySnapshot | null>(null)
const loading = ref(false), syncing = ref(false), busy = ref(false), error = ref(''), notice = ref(''), dialogError = ref('')
const search = ref(''), filter = ref('all'), groupID = ref('all'), groupSearch = ref(''), groupPicker = ref<HTMLDetailsElement | null>(null)
const serverNow = computed(() => Date.parse(snapshot.value?.server_time || '') || Date.now())
const batchKind = ref<CheckKind>('availability'), batchJobs = ref<QualityRun[]>([])
const batchTargets = computed(() => visible.value.filter(a=>!a.unsupported).flatMap(a=>{const target=selectedTarget(a.id);return target?[target.id]:[]}))
const batchFinished = computed(() => batchJobs.value.filter(j=>j.status==='finished').length)
const batchFailed = computed(() => batchJobs.value.filter(j=>j.status==='finished' && !['passed','cache_hit'].includes(j.verdict)).length)
const draftForecast = computed(() => { const cfg=settingsDraft.value; if(!cfg?.auto_enabled)return 0; return targets.value.filter(t=>t.enabled && t.account_enabled).reduce((sum,target)=>{ const p=target.overrides??cfg; return sum+(p.availability_enabled?Math.ceil(1440/Math.max(1,p.availability_interval_minutes)):0)+(p.quality_enabled?Math.ceil(1440/Math.max(5,p.quality_interval_minutes))*p.questions:0)+(p.cache_enabled?Math.ceil(1440/Math.max(5,p.cache_interval_minutes))*3:0) },0) })
const selectedModels = ref<Record<number,string>>({}), savingAccounts = ref(new Set<number>())
const targets = computed(() => snapshot.value?.targets ?? []), accounts = computed(() => snapshot.value?.accounts ?? [])
const byAccount = computed(() => { const map = new Map<number, typeof targets.value>(); for (const target of targets.value) { const items = map.get(target.account_id) ?? []; items.push(target); map.set(target.account_id, items) } return map })
const groups = computed(() => [{ id:'all', name:t('quality.allGroups') }, ...[...new Map(accounts.value.flatMap(a=>a.groups).map(g=>[g.id,g])).values()].sort((a,b)=>a.name.localeCompare(b.name)).map(g=>({id:String(g.id),name:g.name})), { id:'none',name:t('quality.ungrouped') }])
const selectedGroupName = computed(() => groups.value.find(g=>g.id===groupID.value)?.name ?? t('quality.allGroups'))
const visibleGroups = computed(() => groups.value.filter(g=>g.name.toLowerCase().includes(groupSearch.value.trim().toLowerCase())))
const visible = computed(() => accounts.value.filter(a => inGroup(a,groupID.value) && `${a.name} ${accountTargets(a.id).map(x=>x.model).join(' ')}`.toLowerCase().includes(search.value.trim().toLowerCase()) && (filter.value==='all' || (filter.value==='attention' ? accountAttention(a) : filter.value==='paused' ? !a.enabled : filter.value==='unsupported' ? a.unsupported : !accountTargets(a.id).some(x=>x.recent.length || x.latest_quality || x.latest_cache)))))
const dialog = ref<'settings' | 'account' | ''>(''), settingsDraft = ref<QualitySettings | null>(null)
const drawerID = ref<number | null>(null), detailMode = ref<DetailMode>('quality'), detailTabs: DetailMode[] = ['availability','cache','quality','history','settings']
const drawerAccount = computed(() => accounts.value.find(a=>a.id===drawerID.value) ?? null)
const drawerTarget = computed(() => drawerID.value == null ? null : selectedTarget(drawerID.value))
const history = ref<QualityRun[]>([]), detail = ref<QualityRun | null>(null), detailLoading = ref(false)
const targetProfile = ref<QualityProfile | null>(null), inherit = ref(true), targetEnabled = ref(true), newModel = ref('')
let timer: ReturnType<typeof setTimeout> | undefined, disposed = false, loadController: AbortController | undefined, revision = 0, refreshRequested = false

function number(n: number) { return new Intl.NumberFormat().format(n) }
function date(value?: string | null) { return value ? new Date(value).toLocaleString() : '—' }
function accountTargets(id: number) { return byAccount.value.get(id) ?? [] }
function selectedTarget(id: number) { const items=accountTargets(id); return items.find(x=>x.id===selectedModels.value[id]) ?? items.find(x=>x.enabled) ?? items[0] ?? null }
function accountAttention(account: MonitoredAccount) { return accountTargets(account.id).some(target=>attention(target) || !!snapshot.value && isStale(target,snapshot.value.settings,serverNow.value)) }
function inGroup(account: MonitoredAccount, id: string) { return id==='all' || (id==='none' ? !account.groups.length : account.groups.some(g=>String(g.id)===id)) }
function groupCount(id: string) { return accounts.value.filter(a=>inGroup(a,id)).length }
function chooseGroup(id: string) { groupID.value=id; if(groupPicker.value) groupPicker.value.open=false }
function scheduleRefresh() { clearTimeout(timer); if(!disposed && !document.hidden) timer=setTimeout(refresh, refreshRequested ? 0 : targets.value.some(x=>x.pending) || batchJobs.value.some(j=>j.status!=='finished') ? 3000 : 60000) }
async function refresh() {
  if(disposed) return
  if(loading.value){refreshRequested=true;return}
  refreshRequested=false;loading.value=true;clearTimeout(timer);loadController=new AbortController()
  try { const data=await qualityAPI.snapshot(loadController.signal);if(disposed)return;snapshot.value=data;error.value='';if(!groups.value.some(g=>g.id===groupID.value))groupID.value='all'
    await refreshBatch()
    if(dialog.value==='account' && detailMode.value!=='settings') await loadDrawer(detailMode.value==='history' && detail.value?.target_id===drawerTarget.value?.id ? detail.value?.id : undefined)
  } catch { if(!disposed && !loadController.signal.aborted) error.value=t('quality.loadError') }
  finally {loading.value=false;scheduleRefresh()}
}
async function syncAccounts(initial: boolean) {
  if(syncing.value)return
  syncing.value=true;error.value=''
  try { const result=await qualityAPI.sync(initial);if(disposed)return;if(!initial)notice.value=t('quality.synced',{count:result.created});await refresh() }
  catch {if(!disposed)error.value=t('quality.loadError')}
  finally {syncing.value=false}
}
async function toggleAccount(account: MonitoredAccount, enabled: boolean) {
  if(savingAccounts.value.has(account.id))return
  savingAccounts.value.add(account.id)
  try {await qualityAPI.saveAccount(account,enabled);await refresh()}catch{if(!disposed)error.value=t('quality.requestError')}finally{savingAccounts.value.delete(account.id)}
}
async function runAccount(account: MonitoredAccount, kind: CheckKind) {
  const target=selectedTarget(account.id)
  if(!target || account.unsupported)return
  const kinds: CheckKind[]=[kind]
  // Each click is submitted independently, even while earlier jobs are pending.
  const results=await Promise.allSettled(kinds.map(k=>qualityAPI.run([target.id],k)))
  if(disposed)return
  const accepted=results.flatMap(r=>r.status==='fulfilled'?r.value.jobs:[])
  if(accepted.length){target.pending=accepted[0]!;notice.value=t('quality.tasksQueued',{count:accepted.length,account:account.name});await refresh()}
  if(results.some(r=>r.status==='rejected' || r.value.skipped>0))error.value=t('quality.partialQueue')
}
async function runBatch() {
  const ids=[...batchTargets.value],kind=batchKind.value
  let skipped=0
  // Each click is an independent batch; failures never silently replay paid requests.
  for(let offset=0;offset<ids.length;offset+=100) {
    try { const result=await qualityAPI.run(ids.slice(offset,offset+100),kind); if(disposed)return;batchJobs.value.push(...result.jobs);skipped+=result.skipped }
    catch { if(disposed)return;skipped+=ids.length-offset;break }
  }
  await refresh()
  if(disposed)return
  if(skipped)error.value=t('quality.partialQueue')
  else notice.value=t('quality.batchQueued',{count:ids.length})
}
async function refreshBatch() {
  const ids=batchJobs.value.filter(j=>j.status!=='finished').map(j=>j.id)
  for(let offset=0;offset<ids.length;offset+=100) {
    try { const updates=await qualityAPI.statuses(ids.slice(offset,offset+100)); if(disposed)return;const byID=new Map(updates.map(j=>[j.id,j]));const requested=new Set(ids.slice(offset,offset+100));batchJobs.value=batchJobs.value.map(j=>byID.get(j.id)??(requested.has(j.id)?{...j,status:'finished',verdict:'unknown',result:{reason:'record_missing',correct:0,graded:0,planned:0,requests:0,budget_tokens:0,first_token_ms:null,duration_ms:0,items:null,cache_reported:false,cache_input_tokens:0,cache_read_tokens:0}}:j)) }
    catch { if(!disposed)notice.value=t('quality.progressUnavailable');return }
  }
}
function closeDialog(){if(!busy.value){dialog.value='';dialogError.value='';revision++;detailLoading.value=false}}
function openSettings(){if(!snapshot.value)return;settingsDraft.value=JSON.parse(JSON.stringify(snapshot.value.settings));dialogError.value='';dialog.value='settings';revision++}
function restoreDefaults(){if(snapshot.value && settingsDraft.value)settingsDraft.value={...snapshot.value.defaults,version:settingsDraft.value.version}}
async function save(work:()=>Promise<void>){if(busy.value)return;busy.value=true;dialogError.value='';try{await work();notice.value=t('quality.saved');await refresh();if(dialog.value==='account' && detailMode.value==='settings')prepareTarget()}catch{if(!disposed)dialogError.value=t('quality.requestError')}finally{busy.value=false}}
async function saveSettings(){await save(async()=>{if(settingsDraft.value){await qualityAPI.saveSettings(settingsDraft.value);dialog.value=''}})}
function prepareTarget(){const target=drawerTarget.value;if(!target || !snapshot.value){targetProfile.value=null;return}inherit.value=target.overrides==null;targetEnabled.value=target.enabled;targetProfile.value={...(target.overrides ?? profileOf(snapshot.value.settings))}}
async function saveTarget(){const target=drawerTarget.value;await save(async()=>{if(target && targetProfile.value)await qualityAPI.saveTarget(target,inherit.value?null:targetProfile.value,targetEnabled.value)})}
async function addModel(){const account=drawerAccount.value,model=newModel.value.trim();if(!account || !model)return;await save(async()=>{const result=await qualityAPI.add([account.id],[model]);if(!result.created)throw new Error('not-added');newModel.value=''})}
async function openAccount(account:MonitoredAccount,mode:DetailMode,id?:string){drawerID.value=account.id;dialog.value='account';detailMode.value=mode;dialogError.value='';newModel.value='';prepareTarget();await loadDrawer(id)}
async function setDetailMode(mode:DetailMode){detailMode.value=mode;dialogError.value='';prepareTarget();await loadDrawer()}
async function changeDrawerModel(id:string){if(drawerID.value!=null)selectedModels.value[drawerID.value]=id;prepareTarget();await loadDrawer()}
async function loadDrawer(id?:string){
  const current=++revision,target=drawerTarget.value,mode=detailMode.value
  if(!target || mode==='settings'){history.value=[];detail.value=null;detailLoading.value=false;return}
  detailLoading.value=true
  try {const items=await qualityAPI.history(target.id);if(current!==revision || disposed)return;history.value=items
    const latestID=id ?? items.find(x=>(mode==='history'||x.kind===mode))?.id
    if(latestID){const run=await qualityAPI.detail(latestID);if(current===revision && !disposed)detail.value=run}else detail.value=null
  }catch{if(current===revision && !disposed)dialogError.value=t('quality.loadError')}
  finally{if(current===revision)detailLoading.value=false}
}
async function fetchDetail(id:string){const current=++revision;detailLoading.value=true;try{const run=await qualityAPI.detail(id);if(current===revision&&!disposed)detail.value=run}catch{if(current===revision&&!disposed)dialogError.value=t('quality.loadError')}finally{if(current===revision)detailLoading.value=false}}
function visibilityChanged(){if(document.hidden)clearTimeout(timer);else void refresh()}
onMounted(()=>{
  void syncAccounts(true).then(()=>{
    if(disposed)return
    const id=Number(new URLSearchParams(window.location.search).get('account'))
    const account=accounts.value.find(a=>a.id===id)
    if(account)void openAccount(account,'quality')
  })
  document.addEventListener('visibilitychange',visibilityChanged)
})
onUnmounted(()=>{disposed=true;revision++;clearTimeout(timer);loadController?.abort();document.removeEventListener('visibilitychange',visibilityChanged)})
</script>
