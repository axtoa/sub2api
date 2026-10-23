<template>
  <AppLayout>
    <section class="balance-theme balance-page">
      <header class="balance-head">
        <div><h1>{{ t('upstreamBalance.title') }}</h1><p class="balance-muted">{{ t('upstreamBalance.subtitle') }}</p></div>
        <div class="balance-tools">
          <button class="balance-button" :disabled="loading" @click="refresh">{{ t('upstreamBalance.refresh') }}</button>
          <button class="balance-button" @click="openSettings">{{ t('upstreamBalance.settings') }}</button>
          <button class="balance-button" :disabled="batch || querying.size > 0 || !automaticWallets.length" @click="queryAll">{{ t('upstreamBalance.queryAll') }}</button>
          <button class="balance-button primary" @click="openWallet()">+ {{ t('upstreamBalance.add') }}</button>
        </div>
      </header>
      <p v-if="error" class="balance-message error" role="alert">{{ error }}</p>
      <p v-if="notice || batch" class="balance-message" role="status">{{ batch ? t('upstreamBalance.queryingAll', { done: batchDone, total: batchTotal }) : notice }}</p>
      <div class="balance-overview">
        <article><p class="balance-muted">{{ t('upstreamBalance.walletCount') }}</p><strong>{{ wallets.length }}</strong></article>
        <article><p class="balance-muted">{{ t('upstreamBalance.lowCount') }}</p><strong :class="lowCount ? 'warn' : ''">{{ lowCount }}</strong></article>
        <article><p class="balance-muted">{{ t('upstreamBalance.actualTotal') }}</p><div class="balance-totals"><strong v-for="total in totals" :key="total.currency">{{ money(total.amount) }} <small>{{ total.currency }}</small></strong><strong v-if="!totals.length">—</strong></div></article>
      </div>
      <p class="balance-muted">{{ t('upstreamBalance.totalHint') }}</p>
      <div class="balance-filters">
        <input v-model="search" type="search" :aria-label="t('upstreamBalance.search')" :placeholder="t('upstreamBalance.search')" />
        <select v-model="filter" :aria-label="t('upstreamBalance.filter')"><option v-for="item in filters" :key="item" :value="item">{{ t(`upstreamBalance.${item}`) }}</option></select>
      </div>
      <p v-if="!ready && loading" class="balance-empty">{{ t('upstreamBalance.loading') }}</p>
      <div v-else-if="!visibleWallets.length" class="balance-empty">{{ t(wallets.length ? 'upstreamBalance.noMatches' : 'upstreamBalance.empty') }}</div>
      <div v-else class="balance-grid">
        <article v-for="wallet in visibleWallets" :key="wallet.id" class="balance-card" :class="{ 'is-low': wallet.low && !wallet.stale }" :aria-label="wallet.name">
          <div class="balance-card-head"><div class="balance-card-name"><h2>{{ wallet.name }}</h2><p class="balance-muted">{{ wallet.provider || '—' }}</p></div><span class="balance-badge">{{ t(`upstreamBalance.sources.${wallet.mode}`) }}</span></div>
          <div>
            <p class="balance-muted">{{ wallet.estimated_balance != null ? t('upstreamBalance.estimated') : wallet.observation ? t(`upstreamBalance.scopes.${wallet.observation.scope}`) : t('upstreamBalance.statuses.never') }}</p>
            <p class="balance-amount" :class="wallet.low && !wallet.stale ? 'warn' : ''">{{ money(wallet.estimated_balance ?? wallet.observation?.amount) }}<small>{{ wallet.observation?.currency ?? wallet.currency }}</small></p>
            <p v-if="wallet.estimated_balance != null && wallet.observation" class="balance-muted">{{ t('upstreamBalance.baseline') }} {{ money(wallet.observation.amount) }} {{ wallet.observation.currency }}</p>
            <p v-if="wallet.stale" class="balance-muted warn">{{ t('upstreamBalance.lastKnown') }}</p>
          </div>
          <div><p class="balance-muted">{{ t('upstreamBalance.costLabel') }}</p><dl class="balance-metrics"><div v-for="metric in metrics" :key="metric"><dt>{{ t(`upstreamBalance.${metric}`) }}</dt><dd>{{ money(wallet[`${metric}_cost`]) }}</dd></div></dl></div>
          <div>
            <p v-if="wallet.days_remaining != null" class="good">{{ t('upstreamBalance.estimateDays', { days: wallet.days_remaining }) }}</p>
            <p v-if="wallet.days_remaining != null" class="balance-muted">{{ t('upstreamBalance.forecastHint') }}</p>
            <p class="balance-muted">{{ wallet.accounts.length ? `${t('upstreamBalance.linkedAccounts')} · ${wallet.accounts.map(a => a.name).join('、')}` : t('upstreamBalance.notLinked') }}</p>
            <details v-if="wallet.accounts.length" class="balance-breakdown"><summary class="balance-link">{{ t('upstreamBalance.accountBreakdown') }}</summary><div class="balance-table-wrap"><table><thead><tr><th>{{ t('upstreamBalance.accounts') }}</th><th v-for="metric in metrics" :key="metric">{{ t(`upstreamBalance.${metric}`) }}</th></tr></thead><tbody><tr v-for="account in wallet.accounts" :key="account.id"><th>{{ account.name }}</th><td v-for="metric in metrics" :key="metric">{{ money(account[`${metric}_cost`] ?? '0') }}</td></tr></tbody></table></div></details>
            <p v-if="wallet.usage_incomplete" class="balance-muted warn">{{ t('upstreamBalance.incomplete') }}</p>
          </div>
          <div class="balance-muted">
            <p :class="needsAttention(wallet) ? 'warn' : 'good'">{{ t(`upstreamBalance.statuses.${wallet.stale && wallet.status === 'ok' ? 'stale' : wallet.status}`) }}</p>
            <p v-if="wallet.last_error" class="warn">{{ reason(wallet.last_error) }}</p>
            <p v-if="wallet.observation">{{ t(wallet.mode === 'manual' ? 'upstreamBalance.baselineTime' : 'upstreamBalance.queriedAt') }} · {{ datetime(wallet.observation.at) }}</p>
            <p v-if="wallet.mode === 'sub2api'">{{ wallet.auto_enabled ? `${t('upstreamBalance.nextQuery')} · ${datetime(wallet.next_query_at)}` : t('upstreamBalance.autoOff') }}</p>
          </div>
          <footer class="balance-footer">
            <button v-if="wallet.mode === 'sub2api'" class="balance-button" :disabled="querying.has(wallet.id) || batch" @click="queryOne(wallet)">{{ t(querying.has(wallet.id) ? 'upstreamBalance.querying' : 'upstreamBalance.query') }}</button>
            <button v-else class="balance-button" @click="openCalibration(wallet)">{{ t('upstreamBalance.calibrate') }}</button>
            <div class="balance-actions"><button class="balance-link" @click="openWallet(wallet)">{{ t('upstreamBalance.edit') }}</button><button class="balance-link" @click="openHistory(wallet)">{{ t('upstreamBalance.history') }}</button><a v-if="safeURL(wallet.management_url)" class="balance-link" :href="wallet.management_url" target="_blank" rel="noopener noreferrer">{{ t('upstreamBalance.manage') }}</a></div>
          </footer>
        </article>
      </div>
    </section>

    <BaseDialog :show="dialog === 'wallet'" :title="t(editID ? 'upstreamBalance.edit' : 'upstreamBalance.add')" width="wide" panel-class="balance-theme" @close="closeDialog">
      <form id="upstream-wallet-form" class="balance-form" @submit.prevent="saveWallet">
        <div class="balance-form-grid">
          <label>{{ t('upstreamBalance.name') }}<input v-model="draft.name" maxlength="160" :placeholder="t('upstreamBalance.namePlaceholder')" required /></label>
          <label>{{ t('upstreamBalance.provider') }}<input v-model="draft.provider" maxlength="160" :placeholder="t('upstreamBalance.providerPlaceholder')" /></label>
          <label>{{ t('upstreamBalance.mode') }}<select v-model="draft.mode"><option value="sub2api">{{ t('upstreamBalance.sub2api') }}</option><option value="manual">{{ t('upstreamBalance.manual') }}</option></select></label>
          <label>{{ t('upstreamBalance.currency') }}<select v-model="draft.currency"><option v-for="currency in currencies" :key="currency">{{ currency }}</option></select></label>
          <label>{{ t('upstreamBalance.threshold') }}<input v-model="thresholdDraft" inputmode="decimal" pattern="(0|[1-9][0-9]{0,8})(\.[0-9]{1,6})?" /></label>
          <label v-if="draft.mode === 'sub2api'">{{ t('upstreamBalance.interval') }}<input v-model.number="draft.interval_minutes" type="number" min="5" max="1440" required /></label>
        </div>
        <fieldset class="balance-form">
          <legend>{{ t('upstreamBalance.accounts') }} · {{ t('upstreamBalance.selectedCount', { count: draft.account_ids.length }) }}</legend>
          <p class="balance-muted">{{ t('upstreamBalance.accountsHint') }}</p>
          <p v-if="accountsLoading" class="balance-muted">{{ t('upstreamBalance.loading') }}</p>
          <input v-model="accountSearch" type="search" :aria-label="t('upstreamBalance.accountSearch')" :placeholder="t('upstreamBalance.accountSearch')" />
          <div class="balance-accounts"><label v-for="account in availableAccounts" :key="account.id" class="balance-check" :class="{ unavailable: taken(account) }"><input v-model="draft.account_ids" type="checkbox" :value="account.id" :disabled="taken(account) || accountsLoading" /><span>{{ account.name }} <span class="balance-muted">· {{ account.platform }} / {{ account.type }}{{ taken(account) ? ` · ${t('upstreamBalance.usedElsewhere')}` : '' }}</span></span></label><p v-if="!accountsLoading && !availableAccounts.length" class="balance-muted">{{ t('upstreamBalance.noAccounts') }}</p></div>
        </fieldset>
        <template v-if="draft.mode === 'sub2api'">
          <label>{{ t('upstreamBalance.queryAccount') }}<select v-model="draft.query_account_id" required><option :value="null" disabled>{{ t('upstreamBalance.chooseAccount') }}</option><option v-for="account in queryAccounts" :key="account.id" :value="account.id">{{ account.name }}</option></select></label>
          <p class="balance-muted">{{ t('upstreamBalance.sourceHint') }}</p>
          <label class="balance-check"><input v-model="draft.auto_enabled" type="checkbox" />{{ t('upstreamBalance.autoEnabled') }}</label><p class="balance-muted">{{ t('upstreamBalance.intervalHint') }}</p>
        </template>
        <template v-else>
          <label>{{ t(requiresBaseline ? 'upstreamBalance.resetBalance' : 'upstreamBalance.recalibrateBalance') }}<input v-model="initialDraft" inputmode="decimal" pattern="-?(0|[1-9][0-9]{0,8})(\.[0-9]{1,6})?" :required="requiresBaseline" /></label>
          <p v-if="requiresBaseline" class="balance-muted">{{ t('upstreamBalance.resetHint') }}</p>
          <label class="balance-check"><input v-model="draft.estimate_enabled" type="checkbox" :disabled="draft.currency !== 'USD' || !draft.account_ids.length" />{{ t('upstreamBalance.estimateEnabled') }}</label><p class="balance-muted">{{ t('upstreamBalance.estimateHint') }}</p>
        </template>
        <label>{{ t('upstreamBalance.managementURL') }}<input v-model="draft.management_url" type="url" maxlength="2000" placeholder="https://" /></label>
        <label>{{ t('upstreamBalance.note') }}<textarea v-model="draft.note" rows="2" maxlength="2000" /></label>
        <p v-if="dialogError" class="balance-message error" role="alert">{{ dialogError }}</p>
      </form>
      <template #footer><button class="balance-button" :disabled="busy" @click="closeDialog">{{ t('upstreamBalance.cancel') }}</button><button class="balance-button primary" type="submit" form="upstream-wallet-form" :disabled="busy || accountsLoading || accountsFailed">{{ t(busy ? 'upstreamBalance.saving' : 'upstreamBalance.save') }}</button></template>
    </BaseDialog>
    <BaseDialog :show="dialog === 'calibrate'" :title="t('upstreamBalance.calibrate')" panel-class="balance-theme" @close="closeDialog">
      <form id="upstream-calibrate-form" class="balance-form" @submit.prevent="calibrate"><h2>{{ selectedWallet?.name }}</h2><label>{{ t('upstreamBalance.initialBalance') }} · {{ selectedWallet?.currency }}<input v-model="initialDraft" inputmode="decimal" pattern="-?(0|[1-9][0-9]{0,8})(\.[0-9]{1,6})?" required /></label><p class="balance-muted">{{ t('upstreamBalance.calibrationHint') }}</p><p v-if="dialogError" class="balance-message error" role="alert">{{ dialogError }}</p></form>
      <template #footer><button class="balance-button" :disabled="busy" @click="closeDialog">{{ t('upstreamBalance.cancel') }}</button><button class="balance-button primary" type="submit" form="upstream-calibrate-form" :disabled="busy">{{ t(busy ? 'upstreamBalance.saving' : 'upstreamBalance.save') }}</button></template>
    </BaseDialog>
    <BaseDialog :show="dialog === 'settings'" :title="t('upstreamBalance.settings')" panel-class="balance-theme" @close="closeDialog">
      <form v-if="settingsDraft" id="upstream-settings-form" class="balance-form" @submit.prevent="saveSettings"><label class="balance-check"><input v-model="settingsDraft.email_enabled" type="checkbox" />{{ t('upstreamBalance.emailEnabled') }}</label><label>{{ t('upstreamBalance.recipients') }}<textarea v-model="recipientsDraft" rows="3" /></label><p class="balance-muted">{{ t('upstreamBalance.emailHint', { timezone }) }}</p><p class="balance-muted">{{ t('upstreamBalance.reminderHint') }}</p><div v-if="deliveries.length"><h2>{{ t('upstreamBalance.deliveries') }}</h2><p v-for="delivery in deliveries" :key="delivery.day + delivery.recipient" class="balance-muted">{{ delivery.day }} · {{ delivery.recipient }} · {{ t(`upstreamBalance.delivery.${delivery.status}`) }}</p></div></form>
      <p v-else-if="!dialogError" class="balance-muted">{{ t('upstreamBalance.loading') }}</p><p v-if="dialogError" class="balance-message error" role="alert">{{ dialogError }}</p>
      <template #footer><button class="balance-button" :disabled="busy" @click="closeDialog">{{ t('upstreamBalance.cancel') }}</button><button class="balance-button primary" form="upstream-settings-form" type="submit" :disabled="busy || !settingsDraft">{{ t(busy ? 'upstreamBalance.saving' : 'upstreamBalance.save') }}</button></template>
    </BaseDialog>
    <BaseDialog :show="dialog === 'history'" :title="t('upstreamBalance.history')" width="wide" panel-class="balance-theme" @close="closeDialog"><div class="balance-history"><p class="balance-muted">{{ t('upstreamBalance.historyHint') }}</p><p v-if="historyLoading" class="balance-muted">{{ t('upstreamBalance.loading') }}</p><p v-if="dialogError" class="balance-message error" role="alert">{{ dialogError }}</p><article v-for="(event, index) in history" :key="index"><h2>{{ t(event.action === 'calibrate' ? 'upstreamBalance.eventCalibrate' : 'upstreamBalance.eventSave') }}</h2><p class="balance-muted">{{ datetime(event.at) }} · {{ t('upstreamBalance.operator') }} #{{ event.actor_id }}</p><details><summary>{{ t('upstreamBalance.changes') }}</summary><pre>{{ JSON.stringify(event.detail, null, 2) }}</pre></details></article></div></BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { balanceAPI, type AccountOption, type Delivery, type Event, type Settings, type Wallet, type WalletInput } from './api'
import { actualTotals, currencies, money, safeURL } from './display'
import './balance.css'

const { t, te } = useI18n()
const wallets = ref<Wallet[]>([]), loading = ref(false), ready = ref(false), error = ref(''), notice = ref(''), timezone = ref('Asia/Shanghai')
const search = ref(''), filter = ref('all'), filters = ['all', 'low', 'attention', 'manualOnly', 'autoOnly'], metrics = ['today', 'week', 'month'] as const
const lowCount = computed(() => wallets.value.filter(w => w.low && !w.stale).length), totals = computed(() => actualTotals(wallets.value))
const automaticWallets = computed(() => wallets.value.filter(w => w.mode === 'sub2api'))
function needsAttention(w: Wallet) { return w.stale || w.usage_incomplete || ['failed', 'unsupported', 'source_changed'].includes(w.status) }
const visibleWallets = computed(() => wallets.value.filter(w => {
  const matches = `${w.name} ${w.provider} ${w.accounts.map(a => a.name).join(' ')}`.toLowerCase().includes(search.value.trim().toLowerCase())
  return matches && (filter.value === 'all' || (filter.value === 'low' && w.low && !w.stale) || (filter.value === 'attention' && needsAttention(w)) || (filter.value === 'manualOnly' && w.mode === 'manual') || (filter.value === 'autoOnly' && w.mode === 'sub2api'))
}))
const querying = ref(new Set<string>()), batch = ref(false), batchDone = ref(0), batchTotal = ref(0)
const dialog = ref<'' | 'wallet' | 'calibrate' | 'settings' | 'history'>(''), dialogError = ref(''), busy = ref(false), editID = ref(''), selectedWallet = ref<Wallet | null>(null)
const accounts = ref<AccountOption[]>([]), accountsLoading = ref(false), accountsFailed = ref(false), accountSearch = ref('')
function emptyDraft(): WalletInput { return { name: '', provider: '', mode: 'sub2api', query_account_id: null, account_ids: [], currency: 'USD', auto_enabled: true, interval_minutes: 30, estimate_enabled: false, low_threshold: null, management_url: '', note: '', version: 0, initial_balance: null } }
const draft = ref(emptyDraft()), thresholdDraft = ref(''), initialDraft = ref(''), settingsDraft = ref<Settings | null>(null), recipientsDraft = ref(''), deliveries = ref<Delivery[]>([]), history = ref<Event[]>([]), historyLoading = ref(false)
const requiresBaseline = computed(() => !selectedWallet.value || selectedWallet.value.mode !== 'manual' || selectedWallet.value.currency !== draft.value.currency || !selectedWallet.value.observation || [...selectedWallet.value.account_ids].sort().join(',') !== [...draft.value.account_ids].sort().join(','))
function taken(a: AccountOption) { return !!a.wallet_id && a.wallet_id !== editID.value }
const availableAccounts = computed(() => {
  const missing = draft.value.account_ids.filter(id => !accounts.value.some(a => a.id === id)).map(id => ({ id, name: `#${id} · ${t('upstreamBalance.missingAccount')}`, platform: '—', type: 'deleted', status: 'deleted', wallet_id: editID.value }))
  return [...accounts.value, ...missing].filter(a => `${a.name} ${a.platform}`.toLowerCase().includes(accountSearch.value.trim().toLowerCase()))
})
const queryAccounts = computed(() => accounts.value.filter(a => draft.value.account_ids.includes(a.id) && a.type === 'apikey'))
watch(() => [draft.value.mode, draft.value.currency, draft.value.account_ids.join(',')], () => {
  if (draft.value.currency !== 'USD' || !draft.value.account_ids.length) draft.value.estimate_enabled = false
  if (!accountsLoading.value && draft.value.query_account_id && !queryAccounts.value.some(a => a.id === draft.value.query_account_id)) draft.value.query_account_id = null
})
function datetime(s: string) { return new Date(s).toLocaleString(undefined, { timeZone: timezone.value, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) }
function reason(code: string) { return te(`upstreamBalance.reasons.${code}`) ? t(`upstreamBalance.reasons.${code}`) : t('upstreamBalance.queryFailed') }
function requestError(e: unknown) { const err = e as { status?: number; response?: { status?: number } }; return (err?.status ?? err?.response?.status) === 409 ? t('upstreamBalance.conflict') : t('upstreamBalance.requestError') }
let disposed = false, revision = 0, dialogRevision = 0, controller: AbortController | undefined, timer: ReturnType<typeof setInterval> | undefined
let pending: { action: string; body: string; key: string } | undefined
function requestKey(action: string, value: unknown) { const body = JSON.stringify(value); if (!pending || pending.action !== action || pending.body !== body) pending = { action, body, key: crypto.randomUUID() }; return pending.key }
async function refresh() {
  const current = ++revision; controller?.abort(); controller = new AbortController(); loading.value = true; error.value = ''
  try { const data = await balanceAPI.list(controller.signal); if (disposed || current !== revision) return; wallets.value = data.items; timezone.value = data.timezone; ready.value = true }
  catch { if (!disposed && current === revision) error.value = t('upstreamBalance.loadError') }
  finally { if (!disposed && current === revision) loading.value = false }
}
function startDialog(kind: typeof dialog.value) { ++dialogRevision; dialog.value = kind; dialogError.value = ''; notice.value = ''; pending = undefined }
function closeDialog() { if (busy.value) return; ++dialogRevision; dialog.value = ''; dialogError.value = '' }
async function openWallet(wallet?: Wallet) {
  startDialog('wallet'); const current = dialogRevision; editID.value = wallet?.id ?? ''; selectedWallet.value = wallet ?? null; accountSearch.value = ''; initialDraft.value = ''; thresholdDraft.value = wallet?.low_threshold ?? ''
  draft.value = wallet ? { name: wallet.name, provider: wallet.provider, mode: wallet.mode, query_account_id: wallet.query_account_id, account_ids: [...wallet.account_ids], currency: wallet.currency, auto_enabled: wallet.auto_enabled, interval_minutes: wallet.interval_minutes, estimate_enabled: wallet.estimate_enabled, low_threshold: wallet.low_threshold, management_url: wallet.management_url, note: wallet.note, version: wallet.version, initial_balance: null } : emptyDraft()
  accountsLoading.value = true; accountsFailed.value = false
  try { const data = await balanceAPI.accounts(); if (!disposed && current === dialogRevision) accounts.value = data }
  catch { if (!disposed && current === dialogRevision) { accountsFailed.value = true; dialogError.value = t('upstreamBalance.loadError') } }
  finally { if (!disposed && current === dialogRevision) accountsLoading.value = false }
}
function openCalibration(wallet: Wallet) { startDialog('calibrate'); selectedWallet.value = wallet; initialDraft.value = '' }
async function perform(action: string, body: unknown, fn: (key: string) => Promise<unknown>) {
  if (busy.value) return; busy.value = true; dialogError.value = ''
  try { await fn(requestKey(action, body)); if (disposed) return; busy.value = false; closeDialog(); notice.value = t('upstreamBalance.saved'); await refresh() }
  catch (err) { if (!disposed) dialogError.value = requestError(err) }
  finally { busy.value = false }
}
async function saveWallet() {
  if (accountsLoading.value || accountsFailed.value) return
  const body: WalletInput = { ...draft.value, account_ids: [...draft.value.account_ids].sort((a, b) => a - b), low_threshold: thresholdDraft.value.trim() || null, initial_balance: draft.value.mode === 'manual' ? initialDraft.value.trim() || null : null }
  if (body.mode === 'manual') { body.query_account_id = null; body.auto_enabled = false }
  if (!body.name.trim() || (body.mode === 'sub2api' && !body.query_account_id) || (body.mode === 'manual' && requiresBaseline.value && !body.initial_balance) || (body.management_url && !safeURL(body.management_url))) { dialogError.value = t('upstreamBalance.invalid'); return }
  await perform(`save:${editID.value}`, body, key => balanceAPI.save(editID.value, body, key))
}
async function calibrate() { const w = selectedWallet.value; if (w) { const body = { amount: initialDraft.value.trim(), version: w.version }; await perform(`calibrate:${w.id}`, body, key => balanceAPI.calibrate(w.id, body.amount, body.version, key)) } }
async function queryOne(wallet: Wallet, quiet = false) {
  if (querying.value.has(wallet.id) || disposed) return false
  querying.value = new Set(querying.value).add(wallet.id)
  if (!quiet) { notice.value = ''; error.value = '' }
  try { const result = await balanceAPI.query(wallet.id); if (!disposed && !quiet) notice.value = t(result.status === 'ok' ? 'upstreamBalance.queryDone' : 'upstreamBalance.queryFailed'); return result.status === 'ok' }
  catch (err) { if (!disposed && !quiet) error.value = requestError(err); return false }
  finally { const next = new Set(querying.value); next.delete(wallet.id); querying.value = next; if (!quiet && !disposed) await refresh() }
}
async function queryAll() {
  if (batch.value || querying.value.size) return
  const queue = [...automaticWallets.value]; if (!queue.length) { notice.value = t('upstreamBalance.noAutomatic'); return }
  batch.value = true; batchDone.value = 0; batchTotal.value = queue.length; let failed = 0; notice.value = ''
  async function worker() { while (queue.length && !disposed) { const wallet = queue.shift()!; if (!await queryOne(wallet, true)) failed++; batchDone.value++ } }
  try { await Promise.all([worker(), worker(), worker()]); if (!disposed) notice.value = t('upstreamBalance.batchDone', { total: batchTotal.value, failed }) }
  finally { batch.value = false; if (!disposed) await refresh() }
}
async function openSettings() {
  startDialog('settings'); const current = dialogRevision; settingsDraft.value = null; deliveries.value = []
  try { const data = await balanceAPI.settings(); if (disposed || current !== dialogRevision) return; settingsDraft.value = structuredClone(data.settings); recipientsDraft.value = data.settings.recipients.join('\n'); deliveries.value = data.deliveries; timezone.value = data.timezone }
  catch { if (!disposed && current === dialogRevision) dialogError.value = t('upstreamBalance.loadError') }
}
async function saveSettings() {
  if (!settingsDraft.value || busy.value) return
  const recipients = recipientsDraft.value.split(/[,，;；\s]+/).filter(Boolean)
  if (recipients.length > 5 || (settingsDraft.value.email_enabled && !recipients.length)) { dialogError.value = t('upstreamBalance.invalidSettings'); return }
  busy.value = true; dialogError.value = ''
  try { await balanceAPI.saveSettings({ ...settingsDraft.value, recipients }); if (!disposed) { busy.value = false; closeDialog(); notice.value = t('upstreamBalance.saved') } }
  catch (err) { if (!disposed) dialogError.value = requestError(err) }
  finally { busy.value = false }
}
async function openHistory(wallet: Wallet) {
  startDialog('history'); const current = dialogRevision; history.value = []; historyLoading.value = true
  try { const data = await balanceAPI.history(wallet.id); if (!disposed && current === dialogRevision) history.value = data }
  catch { if (!disposed && current === dialogRevision) dialogError.value = t('upstreamBalance.loadError') }
  finally { if (!disposed && current === dialogRevision) historyLoading.value = false }
}
onMounted(() => { void refresh(); timer = setInterval(() => { if (!batch.value && !loading.value && document.visibilityState !== 'hidden') void refresh() }, 60000) })
onUnmounted(() => { disposed = true; ++revision; ++dialogRevision; controller?.abort(); clearInterval(timer) })
</script>
