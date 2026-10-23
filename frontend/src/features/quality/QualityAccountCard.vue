<template>
  <article class="quality-account-card" :aria-label="account.name" :title="account.unsupported ? t('quality.reasons.unsupported') : undefined">
    <header class="quality-account-head">
      <div class="quality-account-name"><h3>{{ account.name }}</h3><span class="quality-caption quality-standard-only">{{ account.platform }} · {{ t('quality.modelCount', { count: targets.length }) }}</span></div>
      <label class="quality-auto"><input type="checkbox" :checked="account.enabled" :disabled="saving || account.unsupported" :aria-label="`${account.name} ${t('quality.autoEnabled')}`" @change="emit('toggle', ($event.target as HTMLInputElement).checked)" />{{ t('quality.autoShort') }}</label>
    </header>
    <p class="quality-account-groups quality-standard-only">{{ account.groups.map(g => g.name).join(' / ') || t('quality.ungrouped') }}</p>
    <div class="quality-model quality-standard-only">
      <select v-if="targets.length > 1" :value="target?.id" :aria-label="`${account.name} ${t('quality.model')}`" @change="emit('model', ($event.target as HTMLSelectElement).value)"><option v-for="item in targets" :key="item.id" :value="item.id">{{ item.model }}</option></select>
      <span v-else>{{ target?.model || t('quality.reasons.unsupported') }}</span>
    </div>
    <div class="quality-rate"><span class="quality-caption">{{ t('quality.probeRate') }}</span><strong>{{ rate == null ? '—' : rate.toFixed(2) }}<small v-if="rate != null">%</small></strong></div>
    <div class="quality-chart-heading"><span>{{ target?.recent.length ? t('quality.last180') : t('quality.noProbes') }}</span><span v-if="target?.probe_failures">{{ t('quality.probeFailures', { count: target.probe_failures }) }}</span></div>
    <button class="quality-chart" :aria-label="`${account.name} ${t('quality.recent')}`" @click="emit('view', 'availability')"><span v-for="(item, index) in slots" :key="index" class="quality-mark" :class="runTone(item) || 'empty'" :title="item ? `${date(item.finished_at || item.created_at)} · ${t(`quality.${verdictKey(item)}`)}` : t('quality.noProbes')" @click.stop="emit('view', 'availability', item?.id)" /></button>
    <p class="quality-last-check quality-caption quality-standard-only">{{ date(target?.recent[0]?.finished_at) }}</p>
    <dl class="quality-facts quality-standard-only"><div><dt>{{ t('quality.ttft') }}</dt><dd>{{ latency }}</dd></div><div><dt>{{ t('quality.cache') }}</dt><dd><button class="quality-text-button" @click="emit('view', 'cache')">{{ cache }}</button></dd></div><div><dt>{{ t('quality.quality') }}</dt><dd><button class="quality-text-button" :class="runTone(target?.latest_quality)" @click="emit('view', 'quality')">{{ capability }}</button></dd></div></dl>
    <div class="quality-compact-facts">
      <button :aria-label="`${t('quality.ttft')}: ${latency}`" :title="`${t('quality.ttft')}: ${latency}`" @click="emit('view', 'availability')"><Icon name="clock" size="xs" />{{ latency }}</button>
      <button :aria-label="`${t('quality.cache')}: ${cache}`" :title="cache" @click="emit('view', 'cache')"><Icon name="database" size="xs" />{{ cacheBrief }}</button>
      <button :class="runTone(target?.latest_quality)" :aria-label="`${t('quality.quality')}: ${capability}`" :title="capability" @click="emit('view', 'quality')"><Icon name="beaker" size="xs" />{{ capabilityBrief }}</button>
    </div>
    <div class="quality-card-actions"><button class="quality-button" :disabled="!target || account.unsupported" :title="t('quality.runAvailability')" @click="emit('run', 'availability')"><Icon name="bolt" size="sm" />{{ t('quality.testShort') }}</button><button class="quality-button" :disabled="!target || account.unsupported" :title="t('quality.runCache')" @click="emit('run', 'cache')"><Icon name="database" size="sm" />{{ t('quality.cacheShort') }}</button><button class="quality-button primary" :disabled="!target || account.unsupported" :title="t('quality.runQuality')" @click="emit('run', 'quality')"><Icon name="beaker" size="sm" />{{ t('quality.qualityShort') }}</button></div>
    <footer class="quality-account-footer quality-standard-only"><button class="quality-text-button" :disabled="!target || account.unsupported" @click="emit('run', 'all')">{{ t('quality.runAll') }}</button><div><button class="quality-text-button" :title="t('quality.edit')" :aria-label="t('quality.edit')" @click="emit('view', 'settings')"><Icon name="cog" size="sm" /></button><button class="quality-text-button" @click="emit('view', 'history')"><Icon name="clock" size="sm" />{{ t('quality.records') }}</button></div></footer>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { CheckKind, MonitoredAccount, QualityTarget } from './api'
import { cachePercent, firstToken, probeRate, probeSlots, runTone, verdictKey } from './display'

const props = defineProps<{ account: MonitoredAccount; targets: QualityTarget[]; target: QualityTarget | null; saving: boolean }>()
const emit = defineEmits<{ toggle: [value: boolean]; model: [id: string]; run: [kind: CheckKind | 'all']; view: [kind: CheckKind | 'history' | 'settings', id?: string] }>()
const { t } = useI18n()
const rate = computed(() => probeRate(props.target)), slots = computed(() => probeSlots(props.target))
const latency = computed(() => { const ms = firstToken(props.target); return ms == null ? '—' : `${Math.round(ms).toLocaleString()}ms` })
const cacheBrief = computed(() => { const percent = cachePercent(props.target?.latest_cache); return percent == null ? t(`quality.${props.target?.latest_cache?.verdict === 'not_observed' ? 'cacheMissShort' : props.target?.latest_cache ? 'unknownShort' : 'cachePending'}`) : `${percent.toFixed(2)}%` })
const cache = computed(() => { const text = t(`quality.${verdictKey(props.target?.latest_cache)}`); const percent = cachePercent(props.target?.latest_cache); return percent == null ? text : `${text} · ${percent.toFixed(2)}%` })
const capability = computed(() => { const run = props.target?.latest_quality; const result = run?.result; return t(`quality.${verdictKey(run)}`) + (result && result.graded === result.planned && result.planned > 0 ? ` · ${result.correct}/${result.planned}` : '') })
const capabilityBrief = computed(() => { const run = props.target?.latest_quality; const result = run?.result; return run?.status === 'finished' && ['passed','suspected','degraded'].includes(run.verdict) && result && result.graded === result.planned && result.planned > 0 ? `${t(`quality.${['suspected','degraded'].includes(run.verdict) ? 'anomalyShort' : 'passedShort'}`)} ${result.correct}/${result.planned}` : t(`quality.${run?.result?.reason === 'timeout' ? 'timeoutShort' : run ? 'incompleteShort' : 'qualityPending'}`) })
function date(value?: string | null) { return value ? new Date(value).toLocaleString() : t('quality.noProbes') }
</script>
