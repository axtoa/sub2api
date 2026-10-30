<template>
  <AppLayout>
    <main class="mx-auto w-full max-w-7xl px-4 py-7 text-gray-900 sm:px-6 lg:px-8 dark:text-white">
      <header class="mb-7 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold">{{ t('userInsights.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('userInsights.subtitle') }}</p>
        </div>
        <button type="button" class="btn btn-secondary px-2.5" :title="t('userInsights.refresh')" :aria-label="t('userInsights.refresh')" :disabled="loading" @click="load">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>
      </header>

      <section v-if="list" class="mb-7 grid grid-cols-2 border-y border-gray-200 py-5 md:grid-cols-4 dark:border-dark-700" :aria-label="t('userInsights.title')">
        <div v-for="(metric, index) in metrics" :key="metric.label" class="min-w-0 px-3 py-2 first:pl-0 md:border-r md:border-gray-200 md:last:border-r-0 dark:md:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ metric.label }}</p>
          <p class="mt-1 truncate text-xl font-semibold tabular-nums" :class="index === 0 ? 'text-emerald-700 dark:text-emerald-400' : ''" :title="metric.value">{{ metric.value }}</p>
        </div>
      </section>
      <p class="mb-6 text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('userInsights.totalHint') }}</p>

      <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center">
        <div class="flex min-w-0 flex-1 items-center gap-2 rounded border border-gray-300 bg-white px-3 dark:border-dark-600 dark:bg-dark-800">
          <Icon name="search" size="sm" class="shrink-0 text-gray-400" />
          <input v-model="searchDraft" type="search" maxlength="100" class="h-10 w-full min-w-0 bg-transparent text-sm outline-none placeholder:text-gray-400" :placeholder="t('userInsights.search')" :aria-label="t('userInsights.search')" @keyup.enter="applySearch" />
        </div>
        <button type="button" class="btn btn-secondary h-10" @click="applySearch">{{ t('userInsights.searchAction') }}</button>
        <select v-model="segment" class="input h-10 w-full sm:w-64" :aria-label="t('userInsights.segment')" @change="applySegment">
          <option value="all">{{ t('userInsights.allUsers') }}</option>
          <option value="redeemed">{{ t('userInsights.redeemed') }}</option>
          <option value="repeat">{{ t('userInsights.repeat') }}</option>
          <option value="inactive30">{{ t('userInsights.inactive30') }}</option>
        </select>
      </div>

      <div class="mb-4 flex items-center gap-2 md:hidden">
        <select v-model="sortBy" class="input h-10 min-w-0 flex-1" :aria-label="t('userInsights.sortBy')" @change="applySort">
          <option v-for="column in columns" :key="column.key" :value="column.key">{{ t(`userInsights.${column.label}`) }}</option>
        </select>
        <button type="button" class="btn btn-secondary h-10 w-10 shrink-0 px-0" :title="t(sortOrder === 'asc' ? 'userInsights.sortDescending' : 'userInsights.sortAscending')" :aria-label="t(sortOrder === 'asc' ? 'userInsights.sortDescending' : 'userInsights.sortAscending')" @click="toggleSort(sortBy)">
          <Icon :name="sortOrder === 'asc' ? 'arrowUp' : 'arrowDown'" size="sm" />
        </button>
      </div>

      <p v-if="error" class="mb-4 rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300" role="alert">{{ error }}</p>
      <div class="overflow-hidden rounded border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800" :aria-busy="loading">
        <div class="hidden overflow-x-auto md:block">
          <table class="w-full min-w-[1200px] text-left text-sm">
            <thead class="bg-gray-50 text-xs font-medium text-gray-500 dark:bg-dark-900 dark:text-dark-400">
              <tr>
                <th v-for="column in columns" :key="column.key" scope="col" class="px-4 py-3" :aria-sort="sortBy === column.key ? (sortOrder === 'asc' ? 'ascending' : 'descending') : 'none'">
                  <button type="button" class="flex w-full items-center gap-1.5 whitespace-nowrap rounded-sm hover:text-emerald-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-emerald-500 dark:hover:text-emerald-400" :class="column.numeric ? 'justify-end' : 'justify-start'" :title="t(sortBy === column.key && sortOrder === 'asc' ? 'userInsights.sortDescending' : 'userInsights.sortAscending')" @click="toggleSort(column.key)">
                    {{ t(`userInsights.${column.label}`) }}
                    <Icon :name="sortBy === column.key ? (sortOrder === 'asc' ? 'arrowUp' : 'arrowDown') : 'arrowsUpDown'" size="sm" class="shrink-0" :class="sortBy === column.key ? 'text-emerald-700 dark:text-emerald-400' : 'text-gray-400 dark:text-dark-500'" />
                  </button>
                </th>
                <th class="w-12 px-4 py-3"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="user in list?.items ?? []" :key="user.id" class="hover:bg-gray-50/70 dark:hover:bg-dark-700/50">
                <td class="px-4 py-3"><button type="button" class="max-w-[230px] text-left hover:text-emerald-700 dark:hover:text-emerald-400" @click="openDetail(user.id)"><span class="block truncate font-medium" :title="user.email">{{ user.email }}</span><span class="text-xs text-gray-500 dark:text-dark-400">{{ user.username || `#${user.id}` }}</span></button></td>
                <td class="min-w-[160px] max-w-[240px] px-4 py-3 text-gray-600 dark:text-dark-300"><span class="block truncate" :title="user.notes || undefined">{{ user.notes || t('userInsights.dash') }}</span></td>
                <td class="px-4 py-3 text-right font-semibold tabular-nums">{{ money(user.redeemed_amount) }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ user.redeem_count }}</td>
                <td class="whitespace-nowrap px-4 py-3 text-gray-600 dark:text-dark-300">{{ date(user.last_redeemed_at) }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ money(user.balance) }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ money(user.usage_30d) }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ user.calls_30d.toLocaleString(locale) }}</td>
                <td class="px-4 py-3 text-right tabular-nums">{{ user.total_calls.toLocaleString(locale) }}</td>
                <td class="whitespace-nowrap px-4 py-3 text-gray-600 dark:text-dark-300">{{ date(user.last_used_at) }}</td>
                <td class="px-4 py-3"><button type="button" class="text-gray-500 hover:text-emerald-700 dark:text-dark-400 dark:hover:text-emerald-400" :title="t('userInsights.viewDetail')" :aria-label="t('userInsights.viewDetail')" @click="openDetail(user.id)"><Icon name="chevronRight" size="sm" /></button></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="divide-y divide-gray-100 md:hidden dark:divide-dark-700">
          <button v-for="user in list?.items ?? []" :key="user.id" type="button" class="w-full px-4 py-4 text-left" @click="openDetail(user.id)">
            <span class="flex items-center justify-between gap-2"><span class="min-w-0 truncate font-medium">{{ user.email }}</span><Icon name="chevronRight" size="sm" class="shrink-0 text-gray-400" /></span>
            <span class="mt-1 flex justify-between gap-2 text-sm"><span class="text-gray-500 dark:text-dark-400">{{ t('userInsights.notes') }}</span><span class="min-w-0 truncate text-right">{{ user.notes || t('userInsights.dash') }}</span></span>
            <span class="mt-2 flex justify-between gap-2 text-sm"><span class="text-gray-500 dark:text-dark-400">{{ t('userInsights.redeemedAmount') }}</span><strong class="tabular-nums">{{ money(user.redeemed_amount) }}</strong></span>
            <span class="mt-1 flex justify-between gap-2 text-sm"><span class="text-gray-500 dark:text-dark-400">{{ t('userInsights.usage30d') }}</span><span class="tabular-nums">{{ money(user.usage_30d) }}</span></span>
            <span class="mt-1 flex justify-between gap-2 text-sm"><span class="text-gray-500 dark:text-dark-400">{{ t('userInsights.calls30d') }}</span><span class="tabular-nums">{{ user.calls_30d.toLocaleString(locale) }}</span></span>
            <span class="mt-1 flex justify-between gap-2 text-sm"><span class="text-gray-500 dark:text-dark-400">{{ t('userInsights.totalCalls') }}</span><span class="tabular-nums">{{ user.total_calls.toLocaleString(locale) }}</span></span>
          </button>
        </div>
        <p v-if="loading && !list" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('userInsights.loading') }}</p>
        <p v-else-if="list && !list.items.length" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('userInsights.noData') }}</p>
      </div>
      <footer v-if="list" class="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500 dark:text-dark-400">
        <span>{{ t('userInsights.pageCount', { count: list.total }) }}</span>
        <div class="ml-auto flex items-center gap-2">
          <button class="btn btn-secondary px-2" type="button" :disabled="loading || page <= 1" :title="t('userInsights.previous')" :aria-label="t('userInsights.previous')" @click="goPage(page - 1)"><Icon name="chevronLeft" size="sm" /></button>
          <span class="min-w-36 text-center">{{ t('userInsights.pageProgress', { page, totalPages }) }}</span>
          <button class="btn btn-secondary px-2" type="button" :disabled="loading || page >= totalPages" :title="t('userInsights.next')" :aria-label="t('userInsights.next')" @click="goPage(page + 1)"><Icon name="chevronRight" size="sm" /></button>
        </div>
      </footer>

      <BaseDialog :show="detailID !== null" :title="t('userInsights.detail')" width="wide" @close="closeDetail">
        <p v-if="detailLoading" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('userInsights.loading') }}</p>
        <p v-else-if="detailError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ detailError }}</p>
        <div v-else-if="detail" class="space-y-7 text-sm text-gray-700 dark:text-dark-200">
          <div><h2 class="break-all text-lg font-semibold text-gray-900 dark:text-white">{{ detail.email }}</h2><p class="text-xs text-gray-500 dark:text-dark-400">{{ detail.username || `#${detail.id}` }} · {{ statusLabel(detail.status) }}</p></div>
          <dl class="grid grid-cols-2 gap-4 border-y border-gray-200 py-4 sm:grid-cols-3 dark:border-dark-700">
            <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('userInsights.redeemedAmount') }}</dt><dd class="mt-1 font-semibold tabular-nums">{{ money(detail.redeemed_amount) }}</dd></div>
            <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('userInsights.adminAdded') }}</dt><dd class="mt-1 font-semibold tabular-nums">{{ money(detail.admin_added_amount) }}</dd></div>
            <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('userInsights.balance') }}</dt><dd class="mt-1 font-semibold tabular-nums">{{ money(detail.balance) }}</dd></div>
            <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('userInsights.usage30d') }}</dt><dd class="mt-1 font-semibold tabular-nums">{{ money(detail.usage_30d) }}</dd></div>
            <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('userInsights.calls30d') }}</dt><dd class="mt-1 font-semibold tabular-nums">{{ detail.calls_30d.toLocaleString(locale) }}</dd></div>
            <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('userInsights.totalCalls') }}</dt><dd class="mt-1 font-semibold tabular-nums">{{ detail.total_calls.toLocaleString(locale) }}</dd></div>
          </dl>
          <div class="grid gap-4 text-xs sm:grid-cols-3"><p><span class="text-gray-500 dark:text-dark-400">{{ t('userInsights.firstRedeemed') }}:</span> {{ date(detail.first_redeemed_at) }}</p><p><span class="text-gray-500 dark:text-dark-400">{{ t('userInsights.lastRedeemed') }}:</span> {{ date(detail.last_redeemed_at) }}</p><p><span class="text-gray-500 dark:text-dark-400">{{ t('userInsights.lastUsed') }}:</span> {{ date(detail.last_used_at) }}</p></div>
          <section><h3 class="mb-3 font-semibold text-gray-900 dark:text-white">{{ t('userInsights.modelPreference') }}</h3><p v-if="!detail.models.length" class="text-gray-500 dark:text-dark-400">{{ t('userInsights.noModels') }}</p><div v-else class="overflow-x-auto"><table class="w-full min-w-[440px] text-left"><thead class="text-xs text-gray-500 dark:text-dark-400"><tr><th class="py-2">{{ t('userInsights.model') }}</th><th class="py-2 text-right">{{ t('userInsights.modelCalls') }}</th><th class="py-2 text-right">{{ t('userInsights.modelCost') }}</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="model in detail.models" :key="model.name"><td class="py-2 pr-3 font-medium">{{ model.name }}</td><td class="py-2 text-right tabular-nums">{{ model.calls }}</td><td class="py-2 text-right tabular-nums">{{ money(model.cost) }}</td></tr></tbody></table></div></section>
          <section><h3 class="mb-3 font-semibold text-gray-900 dark:text-white">{{ t('userInsights.recentRedemptions') }}</h3><p v-if="!detail.redemptions.length" class="text-gray-500 dark:text-dark-400">{{ t('userInsights.noRedemptions') }}</p><div v-else class="divide-y divide-gray-100 dark:divide-dark-700"><div v-for="(item, index) in detail.redemptions" :key="`${item.used_at}-${index}`" class="flex justify-between gap-4 py-2"><span>{{ date(item.used_at) }}</span><strong class="tabular-nums">{{ money(item.value) }}</strong></div></div></section>
          <router-link class="inline-flex items-center gap-1.5 text-emerald-700 hover:underline dark:text-emerald-400" :to="`/admin/usage?user_id=${detail.id}`"><Icon name="externalLink" size="sm" />{{ t('userInsights.allUsage') }}</router-link>
        </div>
      </BaseDialog>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { userInsightsAPI, type InsightDetail, type InsightList, type Segment, type SortKey, type SortOrder } from './api'

const { t, locale } = useI18n()
const list = ref<InsightList | null>(null)
const detail = ref<InsightDetail | null>(null)
const searchDraft = ref('')
const search = ref('')
const segment = ref<Segment>('all')
const page = ref(1)
const sortBy = ref<SortKey>('redeemed_amount')
const sortOrder = ref<SortOrder>('desc')
const columns: Array<{ key: SortKey; label: string; numeric?: boolean }> = [
  { key: 'email', label: 'user' },
  { key: 'notes', label: 'notes' },
  { key: 'redeemed_amount', label: 'redeemedAmount', numeric: true },
  { key: 'redeem_count', label: 'redeemCount', numeric: true },
  { key: 'last_redeemed_at', label: 'lastRedeemed' },
  { key: 'balance', label: 'balance', numeric: true },
  { key: 'usage_30d', label: 'usage30d', numeric: true },
  { key: 'calls_30d', label: 'calls30d', numeric: true },
  { key: 'total_calls', label: 'totalCalls', numeric: true },
  { key: 'last_used_at', label: 'lastUsed' }
]
const loading = ref(false)
const error = ref('')
const detailID = ref<number | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
let listController: AbortController | undefined
let detailController: AbortController | undefined
let listRevision = 0
let detailRevision = 0

const money = (value: string) => `$${Number(value).toLocaleString(locale.value, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
const date = (value: string | null) => value ? new Date(value).toLocaleString(locale.value, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) : t('userInsights.dash')
const statusLabel = (value: string) => value === 'active' || value === 'disabled' ? t(`userInsights.${value}`) : value
const metrics = computed(() => list.value ? [
  { label: t('userInsights.totalAmount'), value: money(list.value.summary.amount) },
  { label: t('userInsights.totalUsers'), value: list.value.summary.users.toLocaleString(locale.value) },
  { label: t('userInsights.totalCodes'), value: list.value.summary.codes.toLocaleString(locale.value) },
  { label: t('userInsights.filteredAmount'), value: money(list.value.filtered_amount) }
] : [])
const totalPages = computed(() => list.value ? Math.max(1, Math.ceil(list.value.total / list.value.page_size)) : 1)

async function load() {
  const revision = ++listRevision
  listController?.abort()
  listController = new AbortController()
  loading.value = true
  error.value = ''
  try {
    const result = await userInsightsAPI.list(page.value, search.value, segment.value, sortBy.value, sortOrder.value, listController.signal)
    if (revision !== listRevision) return
    list.value = result
  } catch (cause) {
    if (revision !== listRevision || (cause && typeof cause === 'object' && 'code' in cause && cause.code === 'ERR_CANCELED')) return
    error.value = t('userInsights.loadError')
  } finally {
    if (revision === listRevision) loading.value = false
  }
}
function applySearch() { search.value = searchDraft.value.trim(); page.value = 1; void load() }
function applySegment() { page.value = 1; void load() }
function goPage(value: number) { page.value = value; void load() }
function applySort() { page.value = 1; void load() }
function toggleSort(key: SortKey) {
  sortOrder.value = sortBy.value === key && sortOrder.value === 'asc' ? 'desc' : 'asc'
  sortBy.value = key
  applySort()
}
async function openDetail(id: number) {
  const revision = ++detailRevision
  detailController?.abort()
  detailController = new AbortController()
  detailID.value = id
  detail.value = null
  detailError.value = ''
  detailLoading.value = true
  try {
    const result = await userInsightsAPI.detail(id, detailController.signal)
    if (revision === detailRevision) detail.value = result
  } catch (cause) {
    if (revision !== detailRevision || (cause && typeof cause === 'object' && 'code' in cause && cause.code === 'ERR_CANCELED')) return
    detailError.value = t('userInsights.detailError')
  } finally {
    if (revision === detailRevision) detailLoading.value = false
  }
}
function closeDetail() { ++detailRevision; detailController?.abort(); detailID.value = null; detail.value = null }
onMounted(() => { void load() })
onUnmounted(() => { ++listRevision; ++detailRevision; listController?.abort(); detailController?.abort() })
</script>
