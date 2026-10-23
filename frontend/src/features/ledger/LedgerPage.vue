<template>
  <AppLayout>
    <main class="ledger-theme ledger-page">
      <header class="ledger-head"><div><h1>{{ t('ledger.title') }}</h1><p class="ledger-muted">{{ t('ledger.subtitle') }}</p></div><div class="ledger-tools"><button class="ledger-button" :disabled="loading" :aria-label="t('ledger.refresh')" :title="t('ledger.refresh')" @click="refresh"><Icon name="refresh" size="sm" /></button><button class="ledger-button" :aria-label="t('ledger.settings')" :title="t('ledger.settings')" @click="openSettings"><Icon name="cog" size="sm" /></button><button class="ledger-button primary" :disabled="!ready" @click="tab === 'entries' ? openEntry() : openResource()"><Icon name="plus" size="sm" />{{ t(tab === 'entries' ? 'ledger.addEntry' : 'ledger.addResource') }}</button></div></header>
      <p v-if="error" class="ledger-message error" role="alert">{{ error }}</p><p v-if="notice" class="ledger-muted" role="status">{{ notice }}</p>
      <nav class="ledger-tabs" role="tablist" :aria-label="t('ledger.title')"><button v-for="item in tabs" :id="`ledger-tab-${item}`" :key="item" role="tab" :aria-selected="tab === item" :aria-controls="`ledger-panel-${item}`" @click="tab = item">{{ t(`ledger.${item}`) }}<span v-if="item === 'resources' && dueCount" class="ledger-badge warn">{{ dueCount }}</span></button></nav>
      <section v-if="tab === 'entries'" id="ledger-panel-entries" role="tabpanel" aria-labelledby="ledger-tab-entries" class="ledger-page">
        <div class="ledger-filters"><input v-model="filters.month" type="month" min="1900-01" max="2200-12" :aria-label="t('ledger.month')" /><button class="ledger-button" :aria-pressed="!filters.month" @click="filters.month = ''">{{ t('ledger.allTime') }}</button><select v-model="filters.kind" :aria-label="t('ledger.kind')"><option value="">{{ t('ledger.allKinds') }}</option><option v-for="kind in kinds" :key="kind" :value="kind">{{ t(`ledger.${kind}`) }}</option></select><input v-model="filters.q" maxlength="100" :placeholder="t('ledger.search')" :aria-label="t('ledger.search')" /><button class="ledger-button" :aria-expanded="moreFilters" @click="moreFilters = !moreFilters">{{ t('ledger.filters') }}</button><button class="ledger-button" :disabled="exporting || loading || !list?.total" @click="exportCSV">{{ t(exporting ? 'ledger.exporting' : 'ledger.export') }}</button></div>
        <div v-if="moreFilters" class="ledger-filters"><select v-model="filters.currency" :aria-label="t('ledger.currency')"><option value="">{{ t('ledger.allCurrencies') }}</option><option v-for="currency in [...currencies, 'CREDIT']" :key="currency">{{ currency }}</option></select><select v-model="filters.resource_id" :aria-label="t('ledger.linkResource')"><option value="">{{ t('ledger.allResources') }}</option><option v-for="resource in resources" :key="resource.id" :value="resource.id">{{ resource.name }}</option></select><select v-model="filters.status" :aria-label="t('ledger.status')"><option value="active">{{ t('ledger.validOnly') }}</option><option value="all">{{ t('ledger.includeVoided') }}</option><option value="voided">{{ t('ledger.voidedOnly') }}</option></select><button class="ledger-button" @click="resetFilters">{{ t('ledger.reset') }}</button></div>
        <div v-if="list" class="ledger-stats"><article v-for="metric in metrics" :key="metric" class="ledger-stat"><h2>{{ t(`ledger.${metric === 'net' ? 'cashDifference' : metric}`) }}</h2><dl v-if="totalsFor(metric).length"><div v-for="total in totalsFor(metric)" :key="total.currency"><dt>{{ total.currency === 'CREDIT' ? t('ledger.creditUnit') : total.currency }}</dt><dd :class="metric === 'income' ? 'good' : ''">{{ money(total[metric]) }}</dd></div></dl><p v-else>—</p><small v-if="metric === 'gift'">{{ t('ledger.giftHint') }}</small></article></div>
        <p class="ledger-muted">{{ t('ledger.totalsHint') }}</p>
        <div class="ledger-table-wrap" :aria-busy="loading"><table class="ledger-table"><thead><tr><th>{{ t('ledger.date') }}</th><th>{{ t('ledger.kind') }}</th><th>{{ t('ledger.amount') }}</th><th>{{ t('ledger.category') }}</th><th>{{ t('ledger.counterparty') }} / {{ t('ledger.note') }}</th><th>{{ t('ledger.actions') }}</th></tr></thead><tbody><tr v-for="entry in list?.items ?? []" :key="entry.id" :class="{ 'ledger-voided': entry.voided }"><td>{{ entry.occurred_on }}</td><td>{{ t(`ledger.${entry.kind}`) }}<small v-if="entry.voided" class="ledger-badge">{{ t('ledger.voided') }}</small></td><td class="money" :class="{ good: entry.kind === 'income' }">{{ entry.kind === 'expense' ? '−' : entry.kind === 'income' ? '+' : '' }}{{ money(entry.amount) }} <small>{{ entry.currency === 'CREDIT' ? t('ledger.creditUnit') : entry.currency }}</small></td><td>{{ entry.category }}<p v-if="entry.resource_id" class="ledger-muted">{{ resourceName(entry.resource_id) }}</p></td><td class="ledger-note"><p>{{ entry.counterparty || '—' }}</p><p class="ledger-muted">{{ entry.note }}</p></td><td><div class="ledger-actions"><button v-if="!entry.voided" class="ledger-link" @click="openEntry(entry)">{{ t('ledger.edit') }}</button><button class="ledger-link" @click="openEntry(entry, true)">{{ t('ledger.clone') }}</button><button class="ledger-link" @click="openHistory(entry.id)">{{ t('ledger.history') }}</button><button v-if="!entry.voided" class="ledger-link bad" @click="openVoid(entry)">{{ t('ledger.void') }}</button></div></td></tr></tbody></table><p v-if="!list?.items.length" class="ledger-empty">{{ t(loading ? 'ledger.loading' : 'ledger.emptyEntries') }}</p></div>
        <footer v-if="list" class="ledger-footer"><span class="ledger-muted">{{ t('ledger.totalRecords', { count: list.total }) }}</span><div class="ledger-tools"><button class="ledger-button" :disabled="filters.page <= 1 || loading" @click="filters.page--">{{ t('ledger.previous') }}</button><span>{{ filters.page }} / {{ Math.max(1, Math.ceil(list.total / list.page_size)) }}</span><button class="ledger-button" :disabled="filters.page * list.page_size >= list.total || loading" @click="filters.page++">{{ t('ledger.next') }}</button></div></footer>
      </section>
      <section v-else id="ledger-panel-resources" role="tabpanel" aria-labelledby="ledger-tab-resources" class="ledger-page">
        <div class="ledger-filters"><input v-model="resourceSearch" :placeholder="t('ledger.searchResource')" :aria-label="t('ledger.searchResource')" /><select v-model="resourceFilter" :aria-label="t('ledger.status')"><option value="active">{{ t('ledger.renewing') }}</option><option value="due">{{ t('ledger.dueSoon') }} ({{ dueCount }})</option><option value="expired">{{ t('ledger.expired') }}</option><option value="archived">{{ t('ledger.notRenewing') }}</option><option value="all">{{ t('ledger.allResources') }}</option></select></div>
        <p class="ledger-muted">{{ t('ledger.reminderSummary', { days: settings?.settings.reminder_days.join(' / ') || '30 / 7 / 1', timezone }) }} {{ t(settings?.settings.email_enabled ? 'ledger.mailOn' : 'ledger.mailOff') }}</p>
        <div class="ledger-resource-grid"><article v-for="resource in visibleResources" :key="resource.id" class="ledger-resource"><header class="ledger-head"><h2>{{ resource.name }}</h2><span class="ledger-badge" :class="resourceTone(resource)">{{ resourceStatus(resource) }}</span></header><p class="ledger-muted">{{ t(`ledger.${resource.kind}`) }} · {{ resource.provider || '—' }}</p><div><p>{{ t('ledger.expiresAt') }} · {{ datetime(resource.expires_at) }}</p><p class="ledger-muted">{{ t('ledger.estimate') }} · {{ resource.renewal_amount ? `${money(resource.renewal_amount)} ${resource.currency}` : t('ledger.notSet') }}</p><p v-if="resource.note" class="ledger-muted">{{ resource.note }}</p></div><div class="ledger-actions"><a v-if="safeURL(resource.management_url)" :href="resource.management_url" class="ledger-link" target="_blank" rel="noopener noreferrer">{{ t('ledger.managementURL') }} ↗</a><button class="ledger-link" @click="viewResourceEntries(resource)">{{ t('ledger.linkedEntries') }}</button><button class="ledger-link" @click="openHistory(resource.id)">{{ t('ledger.history') }}</button></div><footer class="ledger-footer"><button class="ledger-link" @click="openResource(resource)">{{ t('ledger.edit') }}</button><button class="ledger-button" @click="openRenew(resource)">{{ t('ledger.renew') }}</button></footer></article></div><p v-if="!visibleResources.length" class="ledger-empty">{{ t(loading ? 'ledger.loading' : 'ledger.emptyResources') }}</p>
      </section>
    </main>

    <BaseDialog :show="dialog === 'entry'" :title="t(editID ? 'ledger.editEntry' : 'ledger.addEntry')" width="wide" panel-class="ledger-theme" @close="closeDialog">
      <form id="ledger-entry" class="ledger-form" @submit.prevent="saveEntry"><div class="ledger-form-grid"><label>{{ t('ledger.kind') }}<select v-model="entryDraft.kind"><option v-for="kind in kinds" :key="kind" :value="kind">{{ t(`ledger.${kind}`) }}</option></select></label><label>{{ t('ledger.date') }}<input v-model="entryDraft.occurred_on" type="date" min="1900-01-01" max="2200-12-31" required /></label><label>{{ t('ledger.amount') }}<input v-model="entryDraft.amount" inputmode="decimal" pattern="(0|[1-9][0-9]{0,8})(\.[0-9]{1,6})?" required /></label><label>{{ t('ledger.currency') }}<select v-model="entryDraft.currency"><option v-for="currency in entryDraft.kind === 'gift' ? [...currencies, 'CREDIT'] : currencies" :key="currency">{{ currency }}</option></select></label><label>{{ t('ledger.category') }}<input v-model="entryDraft.category" list="ledger-categories" maxlength="25" required /><datalist id="ledger-categories"><option v-for="category in categories" :key="category" :value="category" /></datalist></label><label>{{ t('ledger.counterparty') }}<input v-model="entryDraft.counterparty" maxlength="50" /></label><label class="ledger-span">{{ t('ledger.note') }}<textarea v-model="entryDraft.note" rows="2" maxlength="2000" /></label><label class="ledger-span">{{ t('ledger.linkResource') }}<select v-model="entryDraft.resource_id" :disabled="createLinkedResource"><option :value="null">{{ t('ledger.noResource') }}</option><option v-for="resource in resources" :key="resource.id" :value="resource.id">{{ resource.name }}</option></select></label></div><p v-if="entryDraft.kind === 'gift'" class="ledger-muted">{{ t('ledger.giftExplain') }}</p><label v-if="entryDraft.kind === 'expense' && !editID" class="ledger-check"><input v-model="createLinkedResource" type="checkbox" />{{ t('ledger.createLinkedResource') }}</label><fieldset v-if="createLinkedResource && entryDraft.kind === 'expense'"><legend>{{ t('ledger.resourceInfo') }}</legend><ResourceFields :draft="resourceDraft" /></fieldset><p v-if="dialogError" class="ledger-message error" role="alert">{{ dialogError }}</p></form>
      <template #footer><button class="ledger-button" :disabled="busy" @click="closeDialog">{{ t('ledger.cancel') }}</button><button class="ledger-button primary" type="submit" form="ledger-entry" :disabled="busy">{{ t(busy ? 'ledger.saving' : 'ledger.save') }}</button></template>
    </BaseDialog>
    <BaseDialog :show="dialog === 'resource'" :title="t(editID ? 'ledger.editResource' : 'ledger.addResource')" width="wide" panel-class="ledger-theme" @close="closeDialog"><form id="ledger-resource" class="ledger-form" @submit.prevent="saveResource"><ResourceFields :draft="resourceDraft" /><p class="ledger-muted">{{ t('ledger.resourceOnlyHint') }}</p><p v-if="dialogError" class="ledger-message error" role="alert">{{ dialogError }}</p></form><template #footer><button class="ledger-button" :disabled="busy" @click="closeDialog">{{ t('ledger.cancel') }}</button><button class="ledger-button primary" type="submit" form="ledger-resource" :disabled="busy">{{ t(busy ? 'ledger.saving' : 'ledger.save') }}</button></template></BaseDialog>
    <BaseDialog :show="dialog === 'renew'" :title="t('ledger.renew')" panel-class="ledger-theme" @close="closeDialog"><form v-if="selectedResource" id="ledger-renew" class="ledger-form" @submit.prevent="saveRenew"><h2>{{ selectedResource.name }}</h2><p class="ledger-muted">{{ t('ledger.previousExpiry') }} · {{ datetime(selectedResource.expires_at) }}</p><label>{{ t('ledger.newExpiry') }}<input v-model="renewDraft.expiry" type="datetime-local" min="1900-01-01T00:00" max="2200-12-31T23:59" required /></label><p class="ledger-muted">{{ t('ledger.localTimeHint', { timezone: browserTimezone }) }}</p><label class="ledger-check"><input v-model="renewDraft.record_expense" type="checkbox" />{{ t('ledger.recordExpense') }}</label><p class="ledger-muted">{{ t(renewDraft.record_expense ? 'ledger.renewHint' : 'ledger.dateOnlyHint') }}</p><div v-if="renewDraft.record_expense" class="ledger-form-grid"><label>{{ t('ledger.amount') }}<input v-model="renewDraft.amount" inputmode="decimal" pattern="(0|[1-9][0-9]{0,8})(\.[0-9]{1,6})?" required /></label><label>{{ t('ledger.currency') }}<select v-model="renewDraft.currency"><option v-for="currency in currencies" :key="currency">{{ currency }}</option></select></label><label>{{ t('ledger.date') }}<input v-model="renewDraft.occurred_on" type="date" min="1900-01-01" max="2200-12-31" required /></label></div><label>{{ t('ledger.note') }}<textarea v-model="renewDraft.note" maxlength="2000" rows="2" /></label><p v-if="dialogError" class="ledger-message error" role="alert">{{ dialogError }}</p></form><template #footer><button class="ledger-button" :disabled="busy" @click="closeDialog">{{ t('ledger.cancel') }}</button><button class="ledger-button primary" type="submit" form="ledger-renew" :disabled="busy">{{ t(busy ? 'ledger.saving' : 'ledger.confirm') }}</button></template></BaseDialog>
    <BaseDialog :show="dialog === 'void'" :title="t('ledger.void')" panel-class="ledger-theme" @close="closeDialog"><p>{{ t('ledger.voidHint') }}</p><p v-if="selectedEntry" class="ledger-muted">{{ selectedEntry.occurred_on }} · {{ selectedEntry.category }} · {{ money(selectedEntry.amount) }} {{ selectedEntry.currency }}</p><p v-if="dialogError" class="ledger-message error" role="alert">{{ dialogError }}</p><template #footer><button class="ledger-button" :disabled="busy" @click="closeDialog">{{ t('ledger.cancel') }}</button><button class="ledger-button danger" :disabled="busy" @click="voidEntry">{{ t('ledger.confirmVoid') }}</button></template></BaseDialog>
    <BaseDialog :show="dialog === 'history'" :title="t('ledger.history')" width="wide" panel-class="ledger-theme" @close="closeDialog"><p v-if="historyLoading" class="ledger-muted">{{ t('ledger.loading') }}</p><p v-if="dialogError" class="ledger-message error" role="alert">{{ dialogError }}</p><div class="ledger-history"><p class="ledger-muted">{{ t('ledger.historyHint') }}</p><article v-for="(event, index) in history" :key="index"><h3>{{ t(`ledger.events.${event.action}`) }}</h3><p class="ledger-muted">{{ datetime(event.at) }} · {{ t('ledger.operator') }} #{{ event.actor_id }}</p><details><summary>{{ t('ledger.changes') }}</summary><p>{{ t('ledger.before') }}</p><pre>{{ event.before ? JSON.stringify(event.before, null, 2) : '—' }}</pre><p>{{ t('ledger.after') }}</p><pre>{{ JSON.stringify(event.after, null, 2) }}</pre></details></article></div></BaseDialog>
    <BaseDialog :show="dialog === 'settings'" :title="t('ledger.settings')" panel-class="ledger-theme" @close="closeDialog">
      <form v-if="settingsDraft" id="ledger-settings" class="ledger-form" @submit.prevent="saveSettings">
        <label>{{ t('ledger.reminderDays') }}<input v-model="daysDraft" required placeholder="30, 7, 1" /></label><p class="ledger-muted">{{ t('ledger.daysHint') }}</p>
        <label class="ledger-check"><input v-model="settingsDraft.email_enabled" type="checkbox" />{{ t('ledger.emailEnabled') }}</label><label>{{ t('ledger.recipients') }}<textarea v-model="recipientsDraft" rows="2" /></label><p class="ledger-muted">{{ t('ledger.emailHint', { timezone }) }}</p>
        <div v-if="settings?.deliveries.length"><h2>{{ t('ledger.deliveries') }}</h2><p v-for="delivery in settings.deliveries" :key="delivery.day + delivery.recipient" class="ledger-muted">{{ delivery.day }} · {{ delivery.recipient }} · <span :class="delivery.status === 'failed' ? 'bad' : ''">{{ t(`ledger.delivery.${delivery.status}`) }}</span></p></div>
      </form>
      <p v-else-if="!dialogError" class="ledger-muted">{{ t('ledger.loading') }}</p><p v-if="dialogError" class="ledger-message error" role="alert">{{ dialogError }}</p>
      <template #footer><button class="ledger-button" :disabled="busy" @click="closeDialog">{{ t('ledger.cancel') }}</button><button class="ledger-button primary" type="submit" form="ledger-settings" :disabled="busy || !settingsDraft">{{ t(busy ? 'ledger.saving' : 'ledger.save') }}</button></template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import ResourceFields, { type ResourceDraft } from './ResourceFields.vue'
import { ledgerAPI, type Entry, type EntryInput, type EntryList, type Event, type Filters, type Kind, type Resource, type ResourceInput, type Settings, type SettingsResponse } from './api'
import { calendarDays, currencies, isoDatetime, localDatetime, money } from './display'
import './ledger.css'

const { t } = useI18n()
const tabs = ['entries', 'resources'] as const, kinds: Kind[] = ['expense', 'income', 'gift'], metrics = ['income', 'expense', 'net', 'gift'] as const
const tab = ref<typeof tabs[number]>('entries'), list = ref<EntryList | null>(null), resources = ref<Resource[]>([]), settings = ref<SettingsResponse | null>(null)
const ready = ref(false), loading = ref(false), busy = ref(false), exporting = ref(false), moreFilters = ref(false), error = ref(''), notice = ref(''), dialogError = ref('')
const filters = reactive<Filters>({ month: localDatetime(new Date().toISOString()).slice(0,7), q: '', kind: '', currency: '', resource_id: '', status: 'active', page: 1 })
const timezone = ref('Asia/Shanghai'), browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone, today = ref(localDatetime(new Date().toISOString()).slice(0,10)), now = ref(new Date())
const dialog = ref<'' | 'entry' | 'resource' | 'renew' | 'void' | 'history' | 'settings'>(''), editID = ref(''), selectedResource = ref<Resource | null>(null), selectedEntry = ref<Entry | null>(null)
function emptyEntry(): EntryInput { return { kind: 'expense', amount: '', currency: 'CNY', category: '', occurred_on: today.value, counterparty: '', note: '', resource_id: null, version: 0 } }
function emptyResource(): ResourceDraft { return { name: '', kind: 'server', provider: '', expiry: '', renewal_amount: '', currency: 'CNY', management_url: '', note: '', active: true, version: 0 } }
const entryDraft = ref(emptyEntry()), resourceDraft = ref(emptyResource()), createLinkedResource = ref(false)
const renewDraft = ref({ expiry: '', amount: '', currency: 'CNY', occurred_on: '', note: '', record_expense: true })
const settingsDraft = ref<Settings | null>(null), recipientsDraft = ref(''), daysDraft = ref(''), history = ref<Event[]>([]), historyLoading = ref(false)
const resourceSearch = ref(''), resourceFilter = ref('active')
const categories = computed(() => [...new Set([t('ledger.server'), t('ledger.domain'), t('ledger.resourcePurchase'), t('ledger.other'), ...(list.value?.categories ?? [])])])
function totalsFor(metric: typeof metrics[number]) { return (list.value?.totals ?? []).filter(row => metric === 'gift' ? !/^0(?:\.0+)?$/.test(row.gift) : row.currency !== 'CREDIT') }
const days = (r: Resource) => calendarDays(r.expires_at, now.value, timezone.value)
const expired = (r: Resource) => new Date(r.expires_at) <= now.value
const largestThreshold = computed(() => Math.max(...(settings.value?.settings.reminder_days ?? [30,7,1])))
const due = (r: Resource) => r.active && (expired(r) || days(r) <= largestThreshold.value)
const dueCount = computed(() => resources.value.filter(due).length)
const visibleResources = computed(() => resources.value.filter(r => {
  const matches = `${r.name} ${r.provider} ${r.note}`.toLowerCase().includes(resourceSearch.value.trim().toLowerCase())
  return matches && (resourceFilter.value === 'all' || (resourceFilter.value === 'active' && r.active) || (resourceFilter.value === 'archived' && !r.active) || (resourceFilter.value === 'due' && due(r)) || (resourceFilter.value === 'expired' && r.active && expired(r)))
}))
function resourceTone(r: Resource) { return !r.active ? '' : expired(r) ? 'bad' : due(r) ? 'warn' : 'good' }
function resourceStatus(r: Resource) { return !r.active ? t('ledger.notRenewing') : expired(r) ? t('ledger.expired') : days(r) === 0 ? t('ledger.dueToday') : t('ledger.daysLeft', { days: days(r) }) }
function resourceName(id: string) { return resources.value.find(r => r.id === id)?.name ?? id }
function datetime(value: string) { return new Date(value).toLocaleString(undefined, { timeZone: timezone.value, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) }
function safeURL(value: string) { try { const url = new URL(value); return url.protocol === 'https:' && !url.username && !url.password } catch { return false } }

let disposed = false, loadRevision = 0, dialogRevision = 0, filterTimer: ReturnType<typeof setTimeout> | undefined, clockTimer: ReturnType<typeof setInterval> | undefined, controller: AbortController | undefined
let pending: { action: string; body: string; key: string } | undefined
function requestKey(action: string, body: unknown) {
  const serialized = JSON.stringify(body)
  if (!pending || pending.action !== action || pending.body !== serialized) pending = { action, body: serialized, key: crypto.randomUUID() }
  return pending.key
}
function requestError(err: unknown) { const e = err as { status?: number; response?: { status?: number } }; return (e?.status ?? e?.response?.status) === 409 ? t('ledger.conflict') : t('ledger.requestError') }
async function refresh() {
  const revision = ++loadRevision
  controller?.abort(); controller = new AbortController(); loading.value = true; error.value = ''
  try {
    const [entries, inventory] = await Promise.all([ledgerAPI.entries({ ...filters }, controller.signal), ledgerAPI.resources(controller.signal)])
    if (disposed || revision !== loadRevision) return
    list.value = entries; resources.value = inventory.items; timezone.value = inventory.timezone; today.value = entries.today; now.value = new Date(inventory.server_time); ready.value = true
    if (entries.total && (filters.page - 1) * entries.page_size >= entries.total) filters.page = Math.ceil(entries.total / entries.page_size)
  } catch { if (!disposed && revision === loadRevision) error.value = t('ledger.loadError') }
  finally { if (!disposed && revision === loadRevision) loading.value = false }
}
watch(() => [filters.month, filters.q, filters.kind, filters.currency, filters.resource_id, filters.status], () => { filters.page = 1; scheduleRefresh() })
watch(() => filters.page, scheduleRefresh)
function scheduleRefresh() { clearTimeout(filterTimer); filterTimer = setTimeout(refresh, 250) }
function resetFilters() { Object.assign(filters, { month: today.value.slice(0,7), q: '', kind: '', currency: '', resource_id: '', status: 'active', page: 1 }) }
watch(() => entryDraft.value.kind, kind => { if (kind !== 'gift' && entryDraft.value.currency === 'CREDIT') entryDraft.value.currency = 'CNY'; if (kind !== 'expense') createLinkedResource.value = false })
function startDialog(type: typeof dialog.value) { ++dialogRevision; dialogError.value = ''; notice.value = ''; pending = undefined; dialog.value = type }
function closeDialog() { if (busy.value) return; ++dialogRevision; dialog.value = ''; history.value = []; dialogError.value = '' }
function openEntry(entry?: Entry, clone = false) {
  startDialog('entry'); editID.value = entry && !clone ? entry.id : ''; createLinkedResource.value = false; resourceDraft.value = emptyResource()
  entryDraft.value = entry ? { kind: entry.kind, amount: entry.amount, currency: entry.currency, category: entry.category, occurred_on: clone ? today.value : entry.occurred_on, counterparty: entry.counterparty, note: entry.note, resource_id: entry.resource_id, version: clone ? 0 : entry.version } : emptyEntry()
}
function openResource(resource?: Resource) {
  startDialog('resource'); editID.value = resource?.id ?? ''; resourceDraft.value = resource ? { ...resource, expiry: localDatetime(resource.expires_at), originalExpiry: resource.expires_at, renewal_amount: resource.renewal_amount ?? '' } : emptyResource()
}
function openRenew(resource: Resource) {
  startDialog('renew'); selectedResource.value = resource
  renewDraft.value = { expiry: '', amount: resource.renewal_amount ?? '', currency: resource.currency, occurred_on: today.value, note: '', record_expense: true }
}
function openVoid(entry: Entry) { startDialog('void'); selectedEntry.value = entry }
function viewResourceEntries(resource: Resource) { resetFilters(); filters.month = ''; filters.resource_id = resource.id; moreFilters.value = true; tab.value = 'entries' }
function resourceInput(): ResourceInput { const r = resourceDraft.value; return { name: r.name, kind: r.kind, provider: r.provider, expires_at: r.originalExpiry && localDatetime(r.originalExpiry) === r.expiry ? r.originalExpiry : isoDatetime(r.expiry), renewal_amount: r.renewal_amount || null, currency: r.currency, management_url: r.management_url, note: r.note, active: r.active, version: r.version } }
async function perform(action: string, body: unknown, operation: (key: string) => Promise<unknown>) {
  if (busy.value) return
  busy.value = true; dialogError.value = ''
  try { await operation(requestKey(action, body)); if (disposed) return; busy.value = false; closeDialog(); notice.value = t('ledger.saved'); await refresh() }
  catch (err) { if (!disposed) dialogError.value = requestError(err) }
  finally { busy.value = false }
}
async function saveEntry() {
  try { const body: EntryInput = { ...entryDraft.value }; if (createLinkedResource.value && body.kind === 'expense') { body.resource_id = null; body.new_resource = resourceInput() }; await perform(`entry:${editID.value}`, body, key => ledgerAPI.saveEntry(editID.value, body, key)) }
  catch { dialogError.value = t('ledger.invalidDate') }
}
async function saveResource() { try { const body = resourceInput(); await perform(`resource:${editID.value}`, body, key => ledgerAPI.saveResource(editID.value, body, key)) } catch { dialogError.value = t('ledger.invalidDate') } }
async function saveRenew() {
  const resource = selectedResource.value; if (!resource) return
  try {
    const body = { version: resource.version, expires_at: isoDatetime(renewDraft.value.expiry), record_expense: renewDraft.value.record_expense, amount: renewDraft.value.amount, currency: renewDraft.value.currency, occurred_on: renewDraft.value.occurred_on, note: renewDraft.value.note }
    if (new Date(body.expires_at) <= new Date(resource.expires_at)) { dialogError.value = t('ledger.expiryMustAdvance'); return }
    await perform(`renew:${resource.id}`, body, key => ledgerAPI.renew(resource.id, body, key))
  } catch { dialogError.value = t('ledger.invalidDate') }
}
async function voidEntry() { const entry = selectedEntry.value; if (entry) await perform(`void:${entry.id}`, { version: entry.version }, key => ledgerAPI.voidEntry(entry, key)) }
async function openHistory(id: string) {
  startDialog('history'); history.value = []; historyLoading.value = true; const revision = dialogRevision
  try { const result = await ledgerAPI.history(id); if (!disposed && revision === dialogRevision) history.value = result }
  catch { if (!disposed && revision === dialogRevision) dialogError.value = t('ledger.loadError') }
  finally { if (!disposed && revision === dialogRevision) historyLoading.value = false }
}
async function openSettings() {
  startDialog('settings'); settingsDraft.value = null; const revision = dialogRevision
  try { const result = await ledgerAPI.settings(); if (disposed || revision !== dialogRevision) return; settings.value = result; settingsDraft.value = structuredClone(result.settings); daysDraft.value = result.settings.reminder_days.join(', '); recipientsDraft.value = result.settings.recipients.join('\n') }
  catch { if (!disposed && revision === dialogRevision) dialogError.value = t('ledger.loadError') }
}
async function saveSettings() {
  if (!settingsDraft.value || busy.value) return
  const days = daysDraft.value.split(/[,，\s]+/).filter(Boolean).map(Number), recipients = recipientsDraft.value.split(/[,，;；\s]+/).filter(Boolean)
  if (!days.length || days.length > 10 || days.some(d => !Number.isInteger(d) || d < 1 || d > 365) || new Set(days).size !== days.length || recipients.length > 5) { dialogError.value = t('ledger.invalidSettings'); return }
  busy.value = true; dialogError.value = ''
  try { const result = await ledgerAPI.saveSettings({ ...settingsDraft.value, reminder_days: days, recipients }); if (disposed) return; settings.value = { settings: result, deliveries: settings.value?.deliveries ?? [], timezone: timezone.value }; busy.value = false; closeDialog(); notice.value = t('ledger.saved') }
  catch (err) { if (!disposed) dialogError.value = requestError(err) } finally { busy.value = false }
}
async function exportCSV() {
  if (exporting.value) return; exporting.value = true; error.value = ''
  try { const blob = await ledgerAPI.export({ ...filters }); if (disposed) return; const url = URL.createObjectURL(blob); const link = document.createElement('a'); link.href = url; link.download = `ledger-${filters.month || 'all'}.csv`; document.body.appendChild(link); link.click(); link.remove(); setTimeout(() => URL.revokeObjectURL(url), 1000) }
  catch { if (!disposed) error.value = t('ledger.exportError') } finally { exporting.value = false }
}
onMounted(() => { void refresh(); void ledgerAPI.settings().then(result => { if (!disposed) settings.value = result }).catch(() => {}); clockTimer = setInterval(() => { now.value = new Date() }, 60000) })
onUnmounted(() => { disposed = true; ++loadRevision; ++dialogRevision; controller?.abort(); clearTimeout(filterTimer); clearInterval(clockTimer) })
</script>
