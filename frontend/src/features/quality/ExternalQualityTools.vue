<template>
  <section class="quality-external-tools">
    <p class="quality-caption">{{ t('quality.external.hint') }}</p>
    <div class="quality-resource-filters">
      <label class="quality-resource-search">{{ t('quality.external.search') }}<input v-model="query" type="search" :placeholder="t('quality.external.searchHint')" /></label>
      <label>{{ t('quality.external.filter') }}<select v-model="family"><option value="all">{{ t('quality.all') }}</option><option value="gpt">{{ t('quality.external.gptGeneral') }}</option><option value="claude">{{ t('quality.external.claudeGeneral') }}</option><option value="general">{{ t('quality.external.general') }}</option></select></label>
      <label>{{ t('quality.external.usageLabel') }}<select v-model="usage"><option value="all">{{ t('quality.all') }}</option><option v-for="value in usageOptions" :key="value" :value="value">{{ t(`quality.external.usages.${value}`) }}</option></select></label>
    </div>
    <nav class="quality-resource-groups quality-external-categories" :aria-label="t('quality.external.groupLabel')">
      <button type="button" class="quality-button" :aria-pressed="group === 'all'" :disabled="saving || !!draft" @click="selectGroup('all')">{{ t('quality.external.allResources') }}<span class="quality-external-count">{{ matches.length }}</span></button>
      <button v-for="item in resourceGroups" :key="item.id" type="button" class="quality-button" :aria-pressed="group === item.id" :disabled="saving || !!draft" @click="selectGroup(item.id)">{{ t(`quality.external.groups.${item.id}`) }}<span class="quality-external-count">{{ groupCount(item.id) }}</span></button>
    </nav>
    <nav v-if="group !== 'all'" class="quality-resource-kinds quality-external-categories" :aria-label="t('quality.external.kind')">
      <button type="button" class="quality-button" :aria-pressed="category === 'all'" :disabled="saving || !!draft" @click="category = 'all'">{{ t('quality.all') }}</button>
      <button v-for="kind in categories" :key="kind" type="button" class="quality-button" :aria-pressed="category === kind" :disabled="saving || !!draft" @click="category = kind">{{ t(`quality.external.${kind}`) }}<span class="quality-external-count">{{ kindCount(kind) }}</span></button>
    </nav>
    <p class="quality-caption">{{ category === 'all' ? t('quality.external.discoveryHint') : t(`quality.external.categoryHints.${category}`) }}</p>
    <div class="quality-external-toolbar">
      <button type="button" class="quality-button primary" :disabled="!list || loading || saving || !!draft || list.items.length >= 100" @click="edit()">＋ {{ t('quality.external.add') }}</button>
      <button type="button" class="quality-text-button" :disabled="loading || saving" @click="load">{{ t('quality.external.reload') }}</button>
      <button type="button" class="quality-text-button" :disabled="!list || loading || saving" @click="exportResources">{{ t('quality.external.export') }}</button>
      <span v-if="list" class="quality-caption" role="status">{{ t('quality.external.count', { count: visible.length, total: filtered.length, saved: list.items.length }) }}</span>
    </div>
    <p v-if="loading" class="quality-caption" role="status">{{ t('quality.loading') }}</p>
    <p v-if="error" class="quality-message error" role="alert">{{ error }}</p>
    <p v-if="notice" class="quality-caption" role="status">{{ notice }}</p>
    <form v-if="draft" class="quality-form quality-external-editor" @submit.prevent="save">
      <h3>{{ t(draft.id ? 'quality.external.edit' : 'quality.external.add') }}</h3>
      <div class="quality-form-grid">
        <label>{{ t('quality.external.name') }}<input v-model="draft.name" maxlength="100" required :disabled="saving || loading" /></label>
        <label>{{ t('quality.external.kind') }}<select v-model="draft.kind" :disabled="saving || loading"><optgroup v-for="item in resourceGroups" :key="item.id" :label="t(`quality.external.groups.${item.id}`)"><option v-for="kind in item.kinds" :key="kind" :value="kind">{{ t(`quality.external.${kind}`) }}</option></optgroup></select></label>
      </div>
      <label>{{ t('quality.external.url') }}<input v-model="draft.url" type="url" placeholder="https://" maxlength="2048" required :disabled="saving || loading" /></label>
      <label>{{ t('quality.external.description') }}<textarea v-model="draft.description" rows="3" maxlength="2000" required :disabled="saving || loading" /></label>
      <label>{{ t('quality.external.tagsLabel') }}<input v-model="tagText" maxlength="247" :placeholder="t('quality.external.tagsHint')" :disabled="saving || loading" /></label>
      <div class="quality-form-grid">
        <label>{{ t('quality.external.usageLabel') }}<select v-model="draft.usage" :disabled="saving || loading"><option v-for="value in usageOptions" :key="value" :value="value">{{ t(`quality.external.usages.${value}`) }}</option></select></label>
        <label>{{ t('quality.external.originLabel') }}<select v-model="draft.origin" :disabled="saving || loading"><option v-for="value in originOptions" :key="value" :value="value">{{ t(`quality.external.origins.${value}`) }}</option></select></label>
      </div>
      <label>{{ t('quality.external.notesLabel') }}<textarea v-model="draft.notes" rows="2" maxlength="2000" :disabled="saving || loading" /></label>
      <fieldset><legend>{{ t('quality.external.filter') }}</legend><div class="quality-library-toolbar"><label class="quality-check"><input v-model="draft.families" type="checkbox" value="gpt" :disabled="saving || loading" />ChatGPT / GPT</label><label class="quality-check"><input v-model="draft.families" type="checkbox" value="claude" :disabled="saving || loading" />Claude</label></div><p class="quality-caption">{{ t('quality.external.familyHint') }}</p></fieldset>
      <details class="quality-resource-review-editor">
        <summary>{{ t('quality.external.reviewLabel') }}</summary>
        <div class="quality-form-grid">
          <label>{{ t('quality.external.reviewLabel') }}<select v-model="draft.review" :disabled="saving || loading"><option value="">{{ t('quality.external.reviews.unknown') }}</option><option value="public">{{ t('quality.external.reviews.public') }}</option><option value="unconfirmed">{{ t('quality.external.reviews.unconfirmed') }}</option></select></label>
          <label>{{ t('quality.external.reviewedAt') }}<input v-model="draft.reviewed_at" type="date" :disabled="saving || loading" /></label>
        </div>
        <label>{{ t('quality.external.referenceLabel') }}<input v-model="draft.reference_url" type="url" maxlength="2048" :disabled="saving || loading" /></label>
        <p class="quality-caption">{{ t('quality.external.reviewHint') }}</p>
      </details>
      <div class="quality-library-toolbar"><button class="quality-button primary" :disabled="saving || loading">{{ t('quality.external.save') }}</button><button type="button" class="quality-text-button" :disabled="saving || loading" @click="draft = null">{{ t('quality.cancel') }}</button></div>
    </form>
    <div class="quality-external-grid">
      <article v-for="tool in visible" :key="tool.id" class="quality-external-tool">
        <div class="quality-card-heading"><h3>{{ tool.name }}</h3><span class="quality-caption">{{ t(`quality.external.${tool.kind}`) }}</span></div>
        <p class="quality-caption quality-resource-domain">{{ hostname(tool.url) }} · {{ tool.families?.length ? tool.families.map(f => f === 'gpt' ? 'ChatGPT / GPT' : 'Claude').join(' · ') : t('quality.external.general') }}</p>
        <div class="quality-resource-meta"><span>{{ tool.origin ? t(`quality.external.origins.${tool.origin}`) : t('quality.external.originUnknown') }}</span><span>{{ t(`quality.external.usages.${tool.usage || 'saved'}`) }}</span><span v-if="tool.review" :class="{ 'quality-resource-unconfirmed': tool.review === 'unconfirmed' }">{{ t(`quality.external.reviews.${tool.review}`) }}<template v-if="tool.reviewed_at"> · {{ tool.reviewed_at }}</template></span></div>
        <p class="quality-external-description">{{ description(tool) }}</p>
        <div v-if="tool.tags?.length" class="quality-resource-tags"><span v-for="tag in tool.tags" :key="tag">{{ tag }}</span></div>
        <details v-if="tool.notes" class="quality-resource-notes"><summary>{{ t('quality.external.notesLabel') }}</summary><p>{{ tool.notes }}</p></details>
        <a v-if="tool.reference_url && safeResourceURL(tool.reference_url)" class="quality-text-button quality-resource-reference" :href="tool.reference_url" target="_blank" rel="noopener noreferrer" referrerpolicy="no-referrer">{{ t('quality.external.referenceLabel') }} ↗</a>
        <div class="quality-library-toolbar quality-resource-actions"><a v-if="safeResourceURL(tool.url)" class="quality-button" :href="tool.url" target="_blank" rel="noopener noreferrer" referrerpolicy="no-referrer">{{ t(`quality.external.openByKind.${tool.kind}`) }} ↗</a><button type="button" class="quality-text-button" :disabled="loading || saving || !!draft" @click="edit(tool)">{{ t('quality.external.edit') }}</button><button type="button" class="quality-text-button" :disabled="loading || saving || !!draft" @click="remove(tool.id)">{{ t('quality.external.delete') }}</button></div>
      </article>
    </div>
    <button v-if="visible.length < filtered.length" type="button" class="quality-button quality-resource-more" @click="limit += 12">{{ t('quality.external.more') }}</button>
    <p v-if="list && !loading && !visible.length" class="quality-caption">{{ t('quality.external.empty') }}</p>
    <p class="quality-caption">{{ t('quality.external.sourceHint') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { qualityAPI, type ExternalTool, type ExternalTools, type ExternalToolKind } from './api'
import { resourceGroups, resourceGroup, safeResourceURL, type ResourceGroup } from './externalResources'
const { t, te } = useI18n()
const family = ref('all'), usage = ref('all'), query = ref(''), limit = ref(12)
const group = ref<ResourceGroup>('all'), category = ref<ExternalToolKind | 'all'>('all')
const list = ref<ExternalTools | null>(null), draft = ref<ExternalTool | null>(null), tagText = ref('')
const usageOptions = ['saved', 'trial', 'using'] as const
const originOptions = ['user', 'curated', 'custom'] as const
const categories = computed(() => resourceGroups.find(item => item.id === group.value)?.kinds ?? [])
const loading = ref(false), saving = ref(false), error = ref(''), notice = ref('')
const matches = computed(() => {
  const term = query.value.trim().toLocaleLowerCase()
  return (list.value?.items ?? []).filter(tool => {
    const families = tool.families ?? []
    const familyMatch = family.value === 'all' || (family.value === 'general' ? !families.length : !families.length || families.includes(family.value as 'gpt' | 'claude'))
    const searchText = [tool.name, tool.url, description(tool), tool.notes, ...(tool.tags ?? []), t(`quality.external.${tool.kind}`)].join(' ').toLocaleLowerCase()
    return familyMatch && (usage.value === 'all' || (tool.usage || 'saved') === usage.value) && (!term || searchText.includes(term))
  })
})
const filtered = computed(() => matches.value.filter(tool => (group.value === 'all' || resourceGroup(tool.kind) === group.value) && (category.value === 'all' || tool.kind === category.value)))
const visible = computed(() => filtered.value.slice(0, limit.value))
function groupCount(value: ResourceGroup) { return matches.value.filter(tool => resourceGroup(tool.kind) === value).length }
function kindCount(value: ExternalToolKind) { return matches.value.filter(tool => tool.kind === value).length }
function selectGroup(value: ResourceGroup) { group.value = value; category.value = 'all' }
watch(query, () => { if (!draft.value) selectGroup('all') }, { flush: 'sync' })
watch([query, family, usage, group, category], () => { limit.value = 12 }, { flush: 'sync' })
const controller = new AbortController()
let disposed = false
function description(tool: ExternalTool) { return tool.description || (te(`quality.external.descriptions.${tool.id}`) ? t(`quality.external.descriptions.${tool.id}`) : '') }
function hostname(url: string) { try { return new URL(url).hostname } catch { return '' } }
async function load() {
  if (loading.value || saving.value) return
  loading.value = true; error.value = ''; notice.value = ''
  try { const result = await qualityAPI.externalTools(controller.signal); if (!disposed) { list.value = result; draft.value = null } }
  catch { if (!disposed) error.value = t('quality.loadError') }
  finally { loading.value = false }
}
function edit(tool?: ExternalTool) {
  if (loading.value || saving.value) return
  const kind = category.value === 'all' ? categories.value[0] ?? 'channel' : category.value
  draft.value = tool ? { ...tool, usage: tool.usage || 'saved', origin: tool.origin || 'custom', review: tool.review || undefined, reviewed_at: tool.reviewed_at || '', reference_url: tool.reference_url || '', families: [...(tool.families ?? [])], tags: [...(tool.tags ?? [])], description: description(tool) } : { id: '', name: '', url: '', kind, families: group.value === 'quality' ? ['gpt', 'claude'] : [], description: '', usage: 'saved', origin: 'custom', notes: '', reviewed_at: '', reference_url: '' }
  tagText.value = draft.value.tags?.join(', ') ?? ''
  error.value = ''; notice.value = ''
}
async function persist(items: ExternalTool[]) {
  if (!list.value || loading.value || saving.value) return
  if (new TextEncoder().encode(JSON.stringify(items)).length > 256 * 1024) { error.value = t('quality.external.tooLarge'); return }
  saving.value = true; error.value = ''; notice.value = ''
  try {
    const result = await qualityAPI.saveExternalTools({ items, version: list.value.version })
    if (!disposed) { list.value = result; draft.value = null; notice.value = t('quality.external.saved'); return true }
  } catch { if (!disposed) error.value = t('quality.external.saveError') }
  finally { saving.value = false }
}
async function save() {
  if (!list.value || !draft.value) return
  const tool = { ...draft.value, name: draft.value.name.trim(), url: draft.value.url.trim(), description: draft.value.description.trim(), notes: draft.value.notes?.trim(), reference_url: draft.value.reference_url?.trim(), review: draft.value.review || undefined, families: [...draft.value.families], tags: [...new Set(tagText.value.split(/[,，]/).map(value => value.trim()).filter(Boolean))] }
  if (!tool.name || !tool.description || !safeResourceURL(tool.url) || (tool.reference_url && !safeResourceURL(tool.reference_url)) || tool.tags.length > 8 || tool.tags.some(tag => [...tag].length > 30)) { error.value = t('quality.external.invalid'); return }
  const editing = !!tool.id
  const normalized = (value: string) => new URL(value).href.replace(/\/+$/, '')
  if (list.value.items.some(item => item.id !== tool.id && item.kind === tool.kind && safeResourceURL(item.url) && normalized(item.url) === normalized(tool.url))) { error.value = t('quality.external.duplicate'); return }
  tool.id ||= crypto.randomUUID()
  if (await persist(editing ? list.value.items.map(item => item.id === tool.id ? tool : item) : [...list.value.items, tool])) {
    query.value = ''; selectGroup(resourceGroup(tool.kind)); category.value = tool.kind
    if (family.value !== 'all' && (family.value === 'general' ? tool.families.length > 0 : tool.families.length && !tool.families.includes(family.value as 'gpt' | 'claude'))) family.value = 'all'
    if (usage.value !== 'all' && usage.value !== tool.usage) usage.value = 'all'
    // A large category must still show the resource just saved.
    const index = filtered.value.findIndex(item => item.id === tool.id)
    limit.value = Math.max(12, index + 1)
  }
}
async function remove(id: string) { if (list.value) await persist(list.value.items.filter(tool => tool.id !== id)) }
function exportResources() {
  if (!list.value) return
  const url = URL.createObjectURL(new Blob([JSON.stringify({ exported_at: new Date().toISOString(), ...list.value, items: list.value.items.map(tool => ({ ...tool, description: description(tool) })) }, null, 2)], { type: 'application/json' }))
  const link = document.createElement('a'); link.href = url; link.download = `external-resources-${new Date().toISOString().slice(0, 10)}.json`; link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
onMounted(load)
onUnmounted(() => { disposed = true; controller.abort() })
</script>
