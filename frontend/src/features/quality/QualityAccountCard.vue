<template>
  <article class="quality-account-card" tabindex="0" :aria-label="`${account.name} · ${t('quality.details')}`" @click="openCard" @keydown.enter.self.prevent="emit('view', 'availability')" @keydown.space.self.prevent="emit('view', 'availability')">
    <header class="quality-account-head">
      <button class="quality-account-name" @click.stop="emit('view', 'availability')"><span class="quality-platform-dot" :class="account.platform" /><h3 :title="account.name">{{ account.name }}</h3></button>
      <label class="quality-auto" @click.stop><input type="checkbox" :checked="account.enabled" :disabled="saving || account.unsupported" :aria-label="`${account.name} ${t('quality.autoEnabled')}`" @change="emit('toggle', ($event.target as HTMLInputElement).checked)" /><span>{{ t('quality.autoShort') }}</span></label>
    </header>
    <div class="quality-model"><select v-if="targets.length > 1" :value="target?.id" :aria-label="`${account.name} ${t('quality.model')}`" @click.stop @change="emit('model', ($event.target as HTMLSelectElement).value)"><option v-for="item in targets" :key="item.id" :value="item.id">{{ item.model }}</option></select><span v-else :title="target?.model">{{ target?.model || t('quality.unsupportedShort') }}</span><button class="quality-text-button quality-card-detail" :aria-label="`${account.name} ${t('quality.edit')}`" @click.stop="emit('view', 'settings')"><Icon name="cog" size="sm" /></button></div>
    <div class="quality-rate"><span class="quality-health" :class="stale ? 'warn' : runTone(latest)" :title="status"><i />{{ status }}</span><span :title="t('quality.probeRate')"><strong>{{ rate == null ? '—' : rate.toFixed(1) }}</strong><small v-if="rate != null">%</small></span></div>
    <div class="quality-chart-heading"><span>{{ t('quality.recentCount', { count: Math.min(target?.recent.length || 0, 48) }) }}</span><span>{{ t('quality.last24Hours') }}</span></div>
    <div class="quality-chart" :aria-label="`${account.name} ${t('quality.recent')}`"><button v-for="(item, index) in slots" :key="index" class="quality-mark" :class="runTone(item) || 'empty'" :tabindex="item ? 0 : -1" :aria-label="item ? `${date(item.finished_at)} · ${t(`quality.${verdictKey(item)}`)}` : t('quality.noProbes')" :title="item ? `${date(item.finished_at)} · ${t(`quality.${verdictKey(item)}`)}` : t('quality.noProbes')" @click.stop="emit('view', 'availability', item?.id)" /></div>
    <div class="quality-compact-facts">
      <button :title="t('quality.ttft')" @click.stop="emit('view', 'availability')"><Icon name="clock" size="xs" />{{ latency }}</button>
      <button :title="cacheTitle" @click.stop="emit('view', 'cache')"><Icon name="database" size="xs" />{{ cacheBrief }}</button>
      <button :class="runTone(capabilityRun)" :title="`${capability} · ${date(capabilityRun?.finished_at)}`" @click.stop="emit('view', 'quality')"><Icon name="beaker" size="xs" /><span>{{ capability }}</span></button>
    </div>
    <div class="quality-card-note" :class="target?.pending ? 'quality-pending' : ''" :title="note"><span v-if="target?.pending" class="quality-pulse" />{{ note }}</div>
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
import type { CheckKind, MonitoredAccount, QualitySettings, QualityTarget } from './api'
import { cachePercent, firstToken, probeRate, probeSlots, runTone, verdictKey, reasonKey, isStale } from './display'
const props = defineProps<{ account: MonitoredAccount; targets: QualityTarget[]; target: QualityTarget | null; saving: boolean; settings: QualitySettings; now: number }>()
const emit = defineEmits<{ toggle: [value: boolean]; model: [id: string]; run: [kind: CheckKind]; view: [kind: CheckKind | 'history' | 'settings', id?: string] }>()
const { t } = useI18n()
const latest = computed(() => props.target?.recent[0])
const rate = computed(() => probeRate(props.target)), slots = computed(() => probeSlots(props.target))
const profile = computed(() => props.target?.overrides ?? props.settings)
const latency = computed(() => { const ms = firstToken(props.target); return ms == null ? '—' : ms < 1000 ? `${Math.round(ms)}ms` : `${(ms / 1000).toFixed(1)}s` })
const cacheBrief = computed(() => { const run=props.target?.latest_cache, percent=cachePercent(run); return percent != null ? `${percent.toFixed(0)}%` : !run && !profile.value.cache_enabled ? t('quality.manualValidation') : t(`quality.${run ? 'unknownShort' : 'cachePending'}`) })
const cacheTitle = computed(() => props.target?.latest_cache ? t(reasonKey(props.target.latest_cache)) : t('quality.cacheDefault'))
const capabilityRun = computed(() => props.target?.last_valid_quality ?? props.target?.latest_quality)
const capability = computed(() => { const run=capabilityRun.value, result=run?.result; return run && ['passed','suspected','degraded'].includes(run.verdict) && result && result.graded === result.planned && result.planned > 0 ? `${t(run.verdict==='passed' ? 'quality.samplePassed' : 'quality.anomalyShort')} ${result.correct}/${result.planned}` : t(run ? reasonKey(run) : 'quality.qualityPending') })
const stale = computed(() => isStale(props.target,props.settings,props.now))
const status = computed(() => props.account.unsupported ? t('quality.unsupportedShort') : stale.value ? t('quality.stale') : latest.value ? t(latest.value.verdict==='passed' ? 'quality.available' : reasonKey(latest.value)) : t('quality.never'))
const note = computed(() => {
  const target=props.target
  if (target?.pending) return `${t(`quality.${verdictKey(target.pending)}`)} · ${t(`quality.${target.pending.kind}`)}`
  const failed=target?.latest_quality
  if (failed && !['passed','suspected','degraded'].includes(failed.verdict)) return `${t('quality.qualityShort')} · ${t(reasonKey(failed))}`
  if (!props.settings.auto_enabled || !props.account.enabled || !target?.enabled) return t('quality.automaticOff')
  return latest.value ? `${t('quality.lastChecked')} ${date(latest.value.finished_at)}` : t('quality.awaitingFirst')
})
function date(value?: string | null) { return value ? new Date(value).toLocaleString([], {month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit'}) : '—' }
function openCard(event: MouseEvent) { if (!(event.target as HTMLElement).closest('button,input,select,label')) emit('view','availability') }
</script>
