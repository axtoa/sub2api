<template>
  <section class="quality-external-tools">
    <p class="quality-caption">{{ t('quality.external.hint') }}</p>
    <label>{{ t('quality.external.filter') }}<select v-model="family"><option value="all">{{ t('quality.all') }}</option><option value="gpt">ChatGPT / GPT</option><option value="claude">Claude</option></select></label>
    <div class="quality-external-grid">
      <article v-for="tool in visible" :key="tool.id" class="quality-external-tool">
        <div class="quality-card-heading"><h3>{{ tool.name }}</h3><span class="quality-caption">{{ t(`quality.external.${tool.kind}`) }}</span></div>
        <p class="quality-caption">ChatGPT / GPT · Claude</p>
        <p>{{ t(`quality.external.descriptions.${tool.id}`) }}</p>
        <a class="quality-button" :href="tool.url" target="_blank" rel="noopener noreferrer" referrerpolicy="no-referrer">{{ t('quality.external.open') }} ↗</a>
      </article>
    </div>
    <p class="quality-caption">{{ t('quality.external.sourceHint') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const family = ref('all')
const tools = [
  { id: 'cctest', name: 'CCTest', url: 'https://cctest.ai/zh', kind: 'channel', families: ['gpt', 'claude'] },
  { id: 'oktest', name: 'okTest', url: 'https://oktest.ai/zh', kind: 'channel', families: ['gpt', 'claude'] },
  { id: 'ztest', name: '真测 Ztest', url: 'https://ztest.ai/', kind: 'channel', families: ['gpt', 'claude'] },
  { id: 'llmtest', name: 'LLM Test', url: 'https://llmtest.cn/', kind: 'channel', families: ['gpt', 'claude'] },
  { id: 'artificialanalysis', name: 'Artificial Analysis', url: 'https://artificialanalysis.ai/', kind: 'benchmark', families: ['gpt', 'claude'] },
]
const visible = computed(() => tools.filter(tool => family.value === 'all' || tool.families.includes(family.value)))
</script>
