<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <div ref="scroller" class="min-h-0 flex-1 overflow-y-auto bg-gray-50/60 px-4 py-6 dark:bg-dark-950/30 sm:px-8">
      <div v-if="!messages.length" class="mx-auto flex h-full max-w-xl flex-col items-center justify-center gap-4 text-center">
        <div class="rounded-2xl bg-primary-50 p-4 text-primary-600 dark:bg-primary-900/20 dark:text-primary-400"><Icon name="document" size="xl" /></div>
        <div><h2 class="text-lg font-semibold text-gray-900 dark:text-white">从一句话开始</h2><p class="mt-2 text-sm text-gray-500">写作、润色，或整理一个想法</p></div>
        <div class="flex flex-wrap justify-center gap-2"><button v-for="item in starters" :key="item.label" type="button" class="btn btn-secondary btn-sm" @click="prompt = item.prompt; composer?.focus()">{{ item.label }}</button></div>
      </div>
      <div v-else class="mx-auto max-w-3xl space-y-6">
        <article v-for="(message, index) in messages" :key="index" :class="message.role === 'user' ? 'ml-auto max-w-[85%]' : ''">
          <div class="mb-2 flex items-center gap-2 text-xs text-gray-400"><span>{{ message.role === 'user' ? '你' : message.model || 'AI' }}</span><button v-if="message.content" type="button" class="hover:text-primary-600" title="复制" @click="copy(message.content)"><Icon name="copy" size="xs" /></button></div>
          <div class="whitespace-pre-wrap break-words rounded-xl px-4 py-3 text-sm leading-7" :class="message.role === 'user' ? 'bg-primary-50 text-gray-900 dark:bg-primary-900/20 dark:text-gray-100' : 'border border-gray-200 bg-white text-gray-800 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-200'">{{ message.content || (busy && index === messages.length - 1 ? '思考中…' : '未生成内容') }}</div>
        </article>
      </div>
    </div>
    <form class="border-t border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900" @submit.prevent="send">
      <div class="mx-auto max-w-4xl">
        <div class="mb-3 grid grid-cols-2 items-center gap-2 sm:flex sm:flex-wrap">
          <select v-model.number="keyId" aria-label="文本渠道" :disabled="busy" class="input w-full min-w-0 sm:flex-1 sm:max-w-[230px]"><option :value="0" disabled>选择渠道</option><option v-for="key in activeKeys" :key="key.id" :value="key.id">{{ key.name }} · {{ key.group?.name || key.group?.platform }}</option></select>
          <Select
            v-model="model"
            :options="modelOptions"
            aria-label="文本模型"
            :disabled="busy || loadingModels || !models.length"
            :loading="loadingModels"
            :searchable="true"
            :placeholder="loadingModels ? '加载模型…' : modelError ? '模型加载失败' : '暂无可用文本模型'"
            search-placeholder="搜索模型"
            empty-text="没有匹配的模型"
            class="w-full min-w-0 sm:flex-1 sm:max-w-[320px]"
          />
          <button type="button" class="btn btn-secondary btn-sm" :aria-expanded="showSystem" @click="showSystem = !showSystem">角色</button>
          <button type="button" class="btn btn-secondary btn-sm sm:ml-auto" :disabled="busy || !messages.length" @click="messages = []; error = ''">新对话</button>
        </div>
        <textarea v-if="showSystem" v-model="system" :disabled="busy" rows="2" aria-label="角色设定" placeholder="角色设定（选填）" class="input mb-3 resize-y text-sm" />
        <div class="rounded-xl border border-gray-200 bg-gray-50 p-3 focus-within:border-primary-400 dark:border-dark-700 dark:bg-dark-800/60">
          <textarea ref="composer" v-model="prompt" rows="3" aria-label="文本提示词" placeholder="写下你的问题或创作要求…" class="w-full resize-none border-0 bg-transparent text-sm leading-6 text-gray-900 outline-none dark:text-gray-100" @keydown="handleComposerKeydown" />
          <div class="mt-2 flex items-center justify-between gap-3"><span class="text-xs text-gray-400">Enter 发送 · Shift + Enter 换行</span><button v-if="busy" type="button" class="btn btn-secondary btn-sm" @click="controller?.abort()">停止</button><button v-else type="submit" class="btn btn-primary min-w-[100px] justify-center" :disabled="!selectedKey?.key || !model.trim() || !prompt.trim()">发送</button></div>
        </div>
        <div v-if="modelError" role="alert" class="mt-2 flex items-center gap-2 text-xs text-red-600 dark:text-red-400">
          <span>{{ modelError }}</span>
          <button type="button" class="shrink-0 underline underline-offset-2" :disabled="busy || loadingModels" @click="loadModels">重新加载</button>
        </div>
        <p v-else-if="selectedKey && !loadingModels && !models.length" class="mt-2 text-xs text-gray-500">当前渠道暂无可用文本模型，请切换渠道。</p>
        <p v-if="error" role="alert" class="mt-2 text-xs text-red-600 dark:text-red-400">{{ error }}</p>
        <p v-if="!activeKeys.length" class="mt-2 text-xs text-gray-500">暂无可用渠道，请先创建 API Key。</p>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import { useClipboard } from '@/composables/useClipboard'
import { listCreativeTextModels, streamCreativeText, type CreativeTextMessage } from '@/api/creativeText'
import type { ApiKey } from '@/types'

const props = defineProps<{ apiKeys: ApiKey[] }>()
const { copyToClipboard: copy } = useClipboard()
const activeKeys = computed(() => props.apiKeys.filter(key => key.status === 'active' && key.group))
const keyId = ref(0)
const selectedKey = computed(() => activeKeys.value.find(key => key.id === keyId.value))
const model = ref('')
const models = ref<string[]>([])
const modelOptions = computed(() => models.value.map(id => ({ value: id, label: id })))
const modelError = ref('')
const prompt = ref('')
const system = ref('')
const showSystem = ref(false)
const busy = ref(false)
const loadingModels = ref(false)
const error = ref('')
const messages = ref<(CreativeTextMessage & { model?: string })[]>([])
const scroller = ref<HTMLElement>()
const composer = ref<HTMLTextAreaElement>()
let controller: AbortController | undefined
let modelController: AbortController | undefined
const starters = [
  { label: '写文案', prompt: '帮我写一段产品介绍，产品是：' },
  { label: '润色', prompt: '保留原意，把下面的文字改得自然简洁：\n' },
  { label: '整理思路', prompt: '帮我梳理这个想法，列出可执行的步骤：' },
]
watch(activeKeys, keys => { if (!keys.some(key => key.id === keyId.value)) keyId.value = keys[0]?.id || 0 }, { immediate: true })
async function loadModels() {
  const key = selectedKey.value?.key
  modelController?.abort()
  models.value = []; model.value = ''; modelError.value = ''
  if (!key) { loadingModels.value = false; return }
  const request = new AbortController()
  modelController = request; loadingModels.value = true
  try {
    const found = await listCreativeTextModels(key, request.signal)
    if (request.signal.aborted) return
    models.value = found.filter(id => !/(?:image|video|sora|hailuo|tts|embedding|whisper)/i.test(id))
    model.value = models.value[0] || ''
  } catch {
    if (!request.signal.aborted) modelError.value = '模型列表加载失败，请重试或切换渠道。'
  } finally { if (modelController === request) loadingModels.value = false }
}
watch(() => selectedKey.value?.key, (key, previousKey) => {
  if (previousKey !== undefined && key !== previousKey) {
    messages.value = []
    error.value = ''
  }
  void loadModels()
}, { immediate: true })

function handleComposerKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing || event.keyCode === 229) return
  event.preventDefault()
  if (!event.repeat) void send()
}

async function send() {
  const key = selectedKey.value?.key
  if (busy.value || !key || !model.value.trim() || !prompt.value.trim()) return
  const question = prompt.value.trim()
  const selectedModel = model.value.trim()
  messages.value.push({ role: 'user', content: question })
  const history: CreativeTextMessage[] = messages.value.filter(message => message.content).map(({ role, content }) => ({ role, content }))
  if (system.value.trim()) history.unshift({ role: 'system', content: system.value.trim() })
  messages.value.push({ role: 'assistant', content: '', model: selectedModel })
  const index = messages.value.length - 1
  prompt.value = ''; busy.value = true; error.value = ''; controller = new AbortController()
  try {
    await streamCreativeText(key, selectedModel, history, controller.signal, text => {
      messages.value[index].content += text
      void nextTick(() => { if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight })
    })
    if (!messages.value[index].content) {
      messages.value.splice(index - 1, 2)
      if (!prompt.value.trim()) prompt.value = question
      error.value = '未收到文本，请重试。'
    }
  } catch (err) {
    if (!controller.signal.aborted) error.value = err instanceof Error ? err.message : '生成失败'
    if (!messages.value[index].content) {
      messages.value.splice(index - 1, 2)
      if (!prompt.value.trim()) prompt.value = question
    }
  } finally { busy.value = false }
}
onBeforeUnmount(() => { controller?.abort(); modelController?.abort() })
</script>
