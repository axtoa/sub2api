<template>
  <section class="quality-form quality-library">
    <p class="quality-caption">{{ t('quality.library.hint') }}</p>
    <div class="quality-library-toolbar">
      <input v-model="search" :placeholder="t('quality.library.search')" :aria-label="t('quality.library.search')" />
      <button type="button" class="quality-button primary" :disabled="loading || saving || !!draft || !bank || bank.items.length >= 100" @click="edit()">{{ t('quality.addQuestion') }}</button>
      <button type="button" class="quality-text-button" :disabled="loading || saving" @click="load">{{ t('quality.library.reload') }}</button>
    </div>
    <p v-if="loading" class="quality-caption" role="status">{{ t('quality.loading') }}</p>
    <p v-if="error" class="quality-message error" role="alert">{{ error }}</p>
    <p v-if="notice" class="quality-caption" role="status">{{ notice }}</p>
    <form v-if="draft" class="quality-form quality-library-editor" @submit.prevent="save">
      <h3>{{ t(editingID ? 'quality.library.edit' : 'quality.addQuestion') }}</h3>
      <InspectionQuestionEditor v-model="draft" single :disabled="loading || saving" />
      <div class="quality-library-toolbar"><button class="quality-button primary" :disabled="loading || saving">{{ t('quality.library.save') }}</button><button type="button" class="quality-text-button" :disabled="loading || saving" @click="draft = null">{{ t('quality.cancel') }}</button></div>
    </form>
    <p v-if="bank" class="quality-caption">{{ t('quality.library.count', { count: bank.items.length }) }}</p>
    <div class="quality-library-list">
      <article v-for="item in visible" :key="item.id" class="quality-library-item">
        <div class="quality-card-heading"><strong>{{ item.name }}</strong><span class="quality-caption">{{ t(`quality.custom.${item.match}`) }}</span></div>
        <p class="quality-library-preview">{{ item.prompt }}</p>
        <div class="quality-library-toolbar"><button type="button" class="quality-text-button" :disabled="loading || saving || !!draft" @click="edit(item)">{{ t('quality.library.edit') }}</button><button type="button" class="quality-text-button" :disabled="loading || saving || !!draft" @click="remove(item.id)">{{ t('quality.library.delete') }}</button></div>
      </article>
    </div>
    <p v-if="bank && !visible.length" class="quality-caption">{{ t(bank.items.length ? 'quality.noMatches' : 'quality.library.empty') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import InspectionQuestionEditor from './InspectionQuestionEditor.vue'
import { qualityAPI, type InspectionQuestion, type InspectionQuestionBank, type InspectionQuestionTemplate } from './api'
import { questionSnapshot, validInspectionQuestions } from './inspectionQuestions'

const { t } = useI18n()
const bank = ref<InspectionQuestionBank | null>(null), draft = ref<InspectionQuestion[] | null>(null)
const editingID = ref(''), search = ref(''), error = ref(''), notice = ref(''), loading = ref(false), saving = ref(false)
const visible = computed(() => bank.value?.items.filter(q => `${q.name} ${q.prompt}`.toLowerCase().includes(search.value.trim().toLowerCase())) ?? [])
let disposed = false
const controller = new AbortController()
async function load() {
  if (loading.value || saving.value) return
  loading.value = true; error.value = ''; notice.value = ''
  try { const result = await qualityAPI.inspectionQuestions(controller.signal); if (!disposed) { bank.value = result; draft.value = null } }
  catch { if (!disposed) error.value = t('quality.loadError') }
  finally { loading.value = false }
}
function edit(item?: InspectionQuestionTemplate) {
  if (loading.value || saving.value) return
  editingID.value = item?.id ?? ''; error.value = ''; notice.value = ''
  draft.value = [item ? questionSnapshot(item) : { name: '', prompt: '', match: 'manual', expected: '' }]
}
async function persist(items: InspectionQuestionTemplate[]) {
  if (!bank.value || loading.value || saving.value) return
  if (new TextEncoder().encode(JSON.stringify(items)).length > 256 * 1024) { error.value = t('quality.library.tooLarge'); return }
  saving.value = true; error.value = ''; notice.value = ''
  try {
    const result = await qualityAPI.saveInspectionQuestions({ items, version: bank.value.version })
    if (!disposed) { bank.value = result; draft.value = null; notice.value = t('quality.library.saved') }
  } catch { if (!disposed) error.value = t('quality.library.saveError') }
  finally { saving.value = false }
}
async function save() {
  if (!bank.value || !draft.value || saving.value) return
  if (!validInspectionQuestions(draft.value, 1)) { error.value = t('quality.custom.invalid'); return }
  const item = { ...questionSnapshot(draft.value[0]!), id: editingID.value || crypto.randomUUID() }
  await persist(editingID.value ? bank.value.items.map(q => q.id === editingID.value ? item : q) : [...bank.value.items, item])
}
async function remove(id: string) { if (bank.value) await persist(bank.value.items.filter(q => q.id !== id)) }
onMounted(load)
onUnmounted(() => { disposed = true; controller.abort() })
</script>
