<template>
  <article class="quality-account-card" tabindex="0" :aria-label="`${account.name} · ${t('quality.details')}`" @click="openCard" @keydown.enter.self.prevent="emit('view', 'availability')" @keydown.space.self.prevent="emit('view', 'availability')">
    <header class="quality-account-head">
      <button class="quality-account-name" @click.stop="emit('view', 'availability')"><span class="quality-platform-dot" :class="account.platform" /><h3 :title="account.name">{{ account.name }}</h3></button>
      <label class="quality-auto" @click.stop><input type="checkbox" :checked="account.enabled" :disabled="saving || account.unsupported" :aria-label="`${account.name} ${t('quality.autoEnabled')}`" @change="emit('toggle', ($event.target as HTMLInputElement).checked)" /><span>{{ t('quality.autoShort') }}</span></label>
    </header>
    <div class="quality-model"><select v-if="targets.length > 1" :value="target?.id" :aria-label="`${account.name} ${t('quality.model')}`" @click.stop @change="emit('model', ($event.target as HTMLSelectElement).value)"><option v-for="item in targets" :key="item.id" :value="item.id">{{ item.model }}</option></select><span v-else :title="target?.model">{{ target?.model || t('quality.unsupportedShort') }}</span><button class="quality-text-button quality-card-detail" :aria-label="`${account.name} ${t('quality.edit')}`" @click.stop="emit('view', 'settings')"><Icon name="cog" size="sm" /></button></div>
    <div class="quality-rate"><span class="quality-health" :class="stale ? 'warn' : runTone(latest)" :title="status"><i />{{ status }}</span><span :title="t('quality.probeRate')"><strong>{{ rate == null ? '—' : rate.toFixed(1) }}</strong><small v-if="rate != null">%</small></span></div>
    <div class="quality-chart-heading"><span>{{ t('quality.recentShort', { count: Math.min(target?.recent.length || 0, 48) }) }}</span><span>{{ t('quality.last24Short') }}</span></div>
    <div class="quality-chart" :aria-label="`${account.name} ${t('quality.recent')}`"><button v-for="(item, index) in slots" :key="index" class="quality-mark" :class="runTone(item) || 'empty'" :tabindex="item ? 0 : -1" :aria-label="item ? `${date(item.finished_at)} · ${t(`quality.${verdictKey(item)}`)}` : t('quality.noProbes')" :title="item ? `${date(item.finished_at)} · ${t(`quality.${verdictKey(item)}`)}` : t('quality.noProbes')" @click.stop="emit('view', 'availability', item?.id)" /></div>
    <div class="quality-compact-facts">
      <button class="quality-latency-chip quality-latency-first" :class="latencyTone" :title="t('quality.ttft')" @click.stop="emit('view', 'availability')"><span>{{ t('quality.firstShort') }}</span><strong>{{ latency }}</strong></button>
      <button class="quality-ability-chip" :class="abilityState" :title="t(`quality.abilityHints.${abilityState}`)" @click.stop="emit('view', 'quality')">{{ t(`quality.abilityStates.${abilityState}`) }}</button>
      <button v-if="target" class="quality-assessment-chip" :class="assessment?.state || 'insufficient'" :title="`${assessmentContext ? `${assessmentContext} · ` : ''}${t('quality.assurance.rowHint')}`" @click.stop="emit('assessment', target.id)"><span>{{ t('quality.assurance.scoreShort') }}</span><strong>{{ assessment?.score != null && assessment.state !== 'stale' ? assessment.score.toFixed(0) : '—' }}</strong><small>{{ t(`quality.assurance.statesShort.${assessment?.state || 'insufficient'}`) }}</small></button>
      <span v-if="target?.pending" class="quality-pending" :title="t(`quality.${verdictKey(target.pending)}`)"><span class="quality-pulse" />{{ t(`quality.${verdictKey(target.pending)}`) }}</span>
    </div>
    <footer class="quality-card-actions">
      <button class="quality-text-button" :aria-label="`${account.name} ${t('quality.records')}`" @click.stop="emit('view', 'history')">{{ t('quality.records') }}<Icon name="chevronRight" size="xs" /></button>
      <div><button class="quality-button" :disabled="!target || account.unsupported" @click.stop="emit('run', 'availability')"><Icon name="bolt" size="xs" />{{ t('quality.testShort') }}</button><button class="quality-button primary" :disabled="!target || account.unsupported" @click.stop="emit('run', 'quality')"><Icon name="beaker" size="xs" />{{ t('quality.qualityShort') }}</button></div>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { CheckKind, MonitoredAccount, QualityScore, QualitySettings, QualityTarget } from './api'
import { probeRate, probeSlots, runTone, verdictKey, reasonKey, isStale } from './display'
const props = defineProps<{ account: MonitoredAccount; targets: QualityTarget[]; target: QualityTarget | null; assessment?: QualityScore | null; assessmentContext?: string; saving: boolean; settings: QualitySettings; now: number }>()
const emit = defineEmits<{ toggle: [value: boolean]; model: [id: string]; run: [kind: CheckKind]; view: [kind: CheckKind | 'history' | 'settings', id?: string]; assessment: [targetID: string] }>()
const { t } = useI18n()
const latest = computed(() => props.target?.recent[0])
const rate = computed(() => probeRate(props.target)), slots = computed(() => probeSlots(props.target))
function formatLatency(ms: number | null | undefined) { return ms == null || ms <= 0 ? '—' : ms < 1000 ? `${Math.round(ms)}ms` : `${(ms / 1000).toFixed(1)}s` }
const timedRun = computed(() => props.target?.recent.find(run => run.status === 'finished' && (run.result?.duration_ms ?? 0) > 0))
const latency = computed(() => formatLatency(timedRun.value?.result?.first_token_ms ?? (!timedRun.value ? props.target?.metrics.first_token_ms : null)))
const latencyTone = computed(() => { const ms = timedRun.value?.result?.first_token_ms ?? (!timedRun.value ? props.target?.metrics.first_token_ms : null); return ms == null ? 'unknown' : ms >= 5000 ? 'slow' : ms >= 2000 ? 'medium' : 'fast' })
const abilityState = computed(() => {
  if (props.assessment?.issues.some(issue => issue.code === 'capability_decline') || ['degraded', 'suspected'].includes(props.target?.latest_quality?.verdict ?? '')) return 'suspected'
  return props.target?.latest_quality?.verdict === 'passed' ? 'normal' : 'unknown'
})
const stale = computed(() => isStale(props.target,props.settings,props.now))
const status = computed(() => props.account.unsupported ? t('quality.unsupportedShort') : stale.value ? t('quality.stale') : latest.value ? t(latest.value.verdict==='passed' ? 'quality.available' : reasonKey(latest.value)) : t('quality.never'))
function date(value?: string | null) { return value ? new Date(value).toLocaleString([], {month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit'}) : '—' }
function openCard(event: MouseEvent) { if (!(event.target as HTMLElement).closest('button,input,select,label')) emit('view','availability') }
</script>
