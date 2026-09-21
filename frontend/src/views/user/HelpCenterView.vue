<template>
  <AppLayout>
    <div class="mx-auto grid max-w-[1500px] gap-5 xl:grid-cols-[minmax(0,1fr)_220px] 2xl:grid-cols-[minmax(0,1fr)_240px]">
      <main class="min-w-0 space-y-5">
        <section class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
          <div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div class="max-w-3xl">
              <span class="inline-flex items-center gap-1.5 rounded-full bg-primary-50 px-3 py-1 text-xs font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                <Icon name="book" size="xs" />
                接入文档
              </span>
              <h2 class="mt-4 text-2xl font-semibold text-gray-950 dark:text-white sm:text-3xl">
                选择客户端，一步一步完成接入
              </h2>
              <p class="mt-3 text-sm leading-6 text-gray-600 dark:text-dark-300">
                选择客户端后，页面会显示对应的安装、配置和测试步骤。可以使用 CC Switch 图形配置；支持的客户端也可以手动配置。
              </p>
            </div>
            <div class="grid min-w-0 gap-3 sm:grid-cols-2 lg:w-80 lg:grid-cols-1">
              <div class="rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/60">
                <div class="text-xs font-medium text-gray-500 dark:text-dark-400">API Endpoint</div>
                <div class="mt-2 flex min-w-0 items-center gap-2">
                  <code class="min-w-0 flex-1 truncate rounded bg-white px-2 py-1 font-mono text-xs text-gray-700 dark:bg-dark-800 dark:text-dark-200">{{ endpointBase }}</code>
                  <button type="button" class="btn btn-secondary btn-sm shrink-0 px-2" :title="copyLabel" @click="copy(endpointBase)">
                    <Icon name="copy" size="xs" />
                  </button>
                </div>
              </div>
              <div class="rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/60">
                <div class="text-xs font-medium text-gray-500 dark:text-dark-400">当前 API Key</div>
                <div class="mt-2 flex min-w-0 items-center gap-2">
                  <code class="min-w-0 flex-1 truncate rounded bg-white px-2 py-1 font-mono text-xs text-gray-700 dark:bg-dark-800 dark:text-dark-200">{{ selectedApiKeyPreview }}</code>
                  <button type="button" class="btn btn-secondary btn-sm shrink-0 px-2" :disabled="!apiKeyValue" :title="copyLabel" @click="copy(apiKeyValue)">
                    <Icon name="copy" size="xs" />
                  </button>
                </div>
              </div>
            </div>
          </div>
          <div class="mt-5 flex flex-wrap gap-2">
            <a
              v-for="item in quickLinks"
              :key="item.href"
              :href="item.href"
              class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-600 transition-colors hover:border-primary-200 hover:text-primary-600 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300 dark:hover:border-primary-700 dark:hover:text-primary-300"
            >
              <Icon name="chevronRight" size="xs" />
              {{ item.label }}
            </a>
          </div>
        </section>

        <section id="client" class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
          <SectionHeader index="01" title="选择客户端" description="桌面应用适合新手，CLI 适合熟悉命令行的用户。选择后，下方只显示对应的接入步骤。" />
          <div class="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <button
              v-for="client in clients"
              :key="client.id"
              type="button"
              class="group min-h-36 rounded-xl border p-4 text-left transition-all"
              :class="selectedClientId === client.id
                ? 'border-primary-400 bg-primary-50 shadow-sm ring-2 ring-primary-500/20 dark:border-primary-600 dark:bg-primary-900/20'
                : 'border-gray-200 bg-white hover:border-primary-200 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-700 dark:hover:bg-dark-700/60'"
              @click="selectedClientId = client.id"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-100">
                  <Icon :name="client.icon" size="md" />
                </div>
                <span
                  class="rounded-full px-2 py-0.5 text-[11px] font-semibold"
                  :class="client.recommended
                    ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
                    : 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'"
                >
                  {{ client.badge }}
                </span>
              </div>
              <div class="mt-4 font-semibold text-gray-950 dark:text-white">{{ client.name }}</div>
              <p class="mt-2 text-xs leading-5 text-gray-600 dark:text-dark-300">{{ client.description }}</p>
            </button>
          </div>
        </section>

        <section id="install" class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
          <SectionHeader index="02" :title="`安装 ${selectedClient.name}`" description="先安装客户端本体；如果使用图形配置，再安装 CC Switch。" />
          <div class="mt-5 grid gap-4 lg:grid-cols-2">
            <article class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
              <div class="flex items-center gap-2 font-semibold text-gray-950 dark:text-white">
                <Icon name="download" size="sm" />
                {{ selectedClient.installTitle }}
              </div>
              <p class="mt-3 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ selectedClient.installDescription }}</p>
              <div v-if="selectedClient.commands.length" class="mt-4 space-y-3">
                <CommandBlock
                  v-for="command in selectedClient.commands"
                  :key="command.label"
                  :label="command.label"
                  :code="command.code"
                  @copy="copy"
                />
              </div>
              <a
                :href="selectedClient.installLink"
                target="_blank"
                rel="noopener noreferrer"
                class="btn btn-secondary btn-sm mt-4"
              >
                <Icon name="externalLink" size="xs" />
                官方说明
              </a>
            </article>

            <article class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
              <div class="flex items-center gap-2 font-semibold text-gray-950 dark:text-white">
                <Icon name="cog" size="sm" />
                图形配置：安装 CC Switch
              </div>
              <p class="mt-3 text-sm leading-6 text-gray-600 dark:text-dark-300">
                选择图形配置时需要安装 CC Switch。Codex、Claude Code CLI 和 Grok Build CLI 也可以改用手动配置；Claude Desktop 请使用图形配置。
              </p>
              <div class="mt-4 flex flex-wrap gap-2">
                <a href="https://github.com/farion1231/cc-switch/releases" target="_blank" rel="noopener noreferrer" class="btn btn-primary btn-sm">
                  <Icon name="download" size="xs" />
                  下载 CC Switch
                </a>
                <a href="https://github.com/farion1231/cc-switch/blob/main/docs/user-manual/zh/1-getting-started/1.2-installation.md" target="_blank" rel="noopener noreferrer" class="btn btn-secondary btn-sm">
                  <Icon name="book" size="xs" />
                  安装说明
                </a>
              </div>
            </article>
          </div>
        </section>

        <section id="config" class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
          <SectionHeader index="03" title="配置" description="复制 Endpoint 和 API Key 后，按客户端选择图形配置或手动配置。" />
          <div class="mt-5 grid gap-4 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
            <div class="space-y-4">
              <label class="block">
                <span class="input-label">API Key</span>
                <select v-if="apiKeys.length" v-model="selectedApiKeyId" class="input">
                  <option v-for="item in apiKeys" :key="item.id" :value="item.id">
                    {{ item.name }} · {{ maskApiKey(item.key) }}
                  </option>
                </select>
                <input
                  v-else
                  v-model="manualApiKey"
                  class="input font-mono"
                  placeholder="sk-..."
                  autocomplete="off"
                  spellcheck="false"
                >
                <span class="input-hint">{{ apiKeysHint }}</span>
              </label>

              <label class="block">
                <span class="input-label">Endpoint</span>
                <input v-model="endpointDraft" class="input font-mono" spellcheck="false">
                <span class="input-hint">默认取站点配置中的 API Base URL，没有配置时使用当前域名。</span>
              </label>

              <div>
                <span class="input-label">配置方式</span>
                <div class="grid grid-cols-2 gap-2 rounded-xl bg-gray-100 p-1 dark:bg-dark-900">
                  <button
                    type="button"
                    class="rounded-lg px-3 py-2 text-sm font-medium transition-colors"
                    :class="configMethod === 'cc-switch' ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-800 dark:text-primary-300' : 'text-gray-600 dark:text-dark-300'"
                    @click="configMethod = 'cc-switch'"
                  >
                    CC Switch 图形配置
                  </button>
                  <button
                    type="button"
                    class="rounded-lg px-3 py-2 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                    :class="configMethod === 'manual' ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-800 dark:text-primary-300' : 'text-gray-600 dark:text-dark-300'"
                    :disabled="!selectedClient.manualSupported"
                    @click="configMethod = 'manual'"
                  >
                    手动配置
                  </button>
                </div>
                <p v-if="!selectedClient.manualSupported" class="mt-2 text-xs text-amber-600 dark:text-amber-300">
                  {{ selectedClient.name }} 请使用 CC Switch 图形配置。
                </p>
              </div>
            </div>

            <article class="min-w-0 rounded-xl border border-gray-200 p-4 dark:border-dark-700">
              <template v-if="configMethod === 'cc-switch'">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="font-semibold text-gray-950 dark:text-white">在 CC Switch 新增 Provider</div>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">把以下字段复制到对应输入框。</p>
                  </div>
                  <Icon name="cog" size="lg" class="text-primary-500" />
                </div>
                <dl class="mt-4 grid gap-3 sm:grid-cols-2">
                  <CopyField label="Provider Name" :value="providerName" @copy="copy" />
                  <CopyField label="Base URL" :value="endpointBase" @copy="copy" />
                  <CopyField label="API Key" :value="apiKeyValue || '请先选择或填写 API Key'" @copy="copy" />
                  <CopyField label="Default Model" :value="selectedClient.defaultModel" @copy="copy" />
                </dl>
                <div class="mt-4 rounded-xl bg-gray-50 p-4 text-sm leading-6 text-gray-600 dark:bg-dark-900/60 dark:text-dark-300">
                  选择当前客户端对应的 Provider 类型，粘贴 Base URL 与 API Key，保存后把该 Provider 设为当前使用项。
                </div>
              </template>
              <template v-else>
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="font-semibold text-gray-950 dark:text-white">{{ selectedClient.manualTitle }}</div>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ selectedClient.manualHint }}</p>
                  </div>
                  <button type="button" class="btn btn-secondary btn-sm" @click="copy(manualConfig)">
                    <Icon name="copy" size="xs" />
                    复制
                  </button>
                </div>
                <pre class="code-block mt-4 max-h-[420px] text-xs leading-5"><code>{{ manualConfig }}</code></pre>
              </template>
            </article>
          </div>
        </section>

        <section id="test" class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
          <SectionHeader index="04" title="测试" :description="selectedClient.testDescription" />
          <div class="mt-5 grid gap-4 lg:grid-cols-[minmax(0,1fr)_300px]">
            <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
              <CommandBlock :label="selectedClient.testLabel" :code="selectedClient.testCommand" @copy="copy" />
            </div>
            <div class="rounded-xl bg-emerald-50 p-4 dark:bg-emerald-900/20">
              <div class="flex items-center gap-2 font-semibold text-emerald-800 dark:text-emerald-200">
                <Icon name="checkCircle" size="sm" />
                成功检查
              </div>
              <ul class="mt-3 space-y-2 text-sm text-emerald-800 dark:text-emerald-100">
                <li v-for="item in successChecks" :key="item" class="flex gap-2">
                  <Icon name="check" size="xs" class="mt-1 shrink-0" />
                  <span>{{ item }}</span>
                </li>
              </ul>
            </div>
          </div>
        </section>

        <section id="troubleshooting" class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
          <SectionHeader index="05" title="错误排查" description="如果接入失败，先从 Endpoint、API Key、客户端代理和模型名称四个方向检查。" />
          <div class="mt-5 grid gap-4 lg:grid-cols-3">
            <article v-for="item in checks" :key="item.title" class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
              <div class="flex items-center gap-2 font-semibold text-gray-950 dark:text-white">
                <Icon :name="item.icon" size="sm" />
                {{ item.title }}
              </div>
              <p class="mt-3 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ item.description }}</p>
            </article>
          </div>

          <div class="mt-5 space-y-3">
            <details v-for="item in troubleItems" :key="item.title" class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
              <summary class="cursor-pointer select-none font-medium text-gray-900 dark:text-white">{{ item.title }}</summary>
              <p class="mt-3 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ item.description }}</p>
            </details>
          </div>

          <div class="mt-5 rounded-xl bg-gray-50 p-4 dark:bg-dark-900/60">
            <div class="font-semibold text-gray-950 dark:text-white">联系反馈时请带上这些信息</div>
            <ul class="mt-3 grid gap-2 text-sm text-gray-600 dark:text-dark-300 sm:grid-cols-2">
              <li v-for="item in feedbackItems" :key="item" class="flex gap-2">
                <Icon name="chevronRight" size="xs" class="mt-1 shrink-0 text-primary-500" />
                <span>{{ item }}</span>
              </li>
            </ul>
          </div>
        </section>
      </main>

      <aside class="hidden xl:block">
        <div class="sticky top-20 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="text-sm font-semibold text-gray-950 dark:text-white">快速跳转</div>
          <nav class="mt-3 space-y-1">
            <a
              v-for="item in quickLinks"
              :key="item.href"
              :href="item.href"
              class="flex items-center gap-2 rounded-lg px-2 py-2 text-sm text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-950 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white"
            >
              <Icon name="chevronRight" size="xs" />
              {{ item.label }}
            </a>
          </nav>
        </div>
      </aside>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { keysAPI } from '@/api'
import { useClipboard } from '@/composables/useClipboard'
import { maskApiKey } from '@/utils/maskApiKey'
import { getPublicOrigin, normalizePublicEndpoint } from '@/utils/publicEndpoint'
import type { ApiKey } from '@/types'

type IconName =
  | 'book'
  | 'copy'
  | 'download'
  | 'externalLink'
  | 'cog'
  | 'checkCircle'
  | 'check'
  | 'chat'
  | 'terminal'
  | 'chatBubble'
  | 'globe'
  | 'key'
  | 'cpu'
  | 'sync'
  | 'refresh'
  | 'document'
type ClientId = 'codex-app' | 'codex-cli' | 'claude-desktop' | 'claude-code' | 'grok-build'

interface ClientGuide {
  id: ClientId
  name: string
  icon: IconName
  description: string
  badge: string
  recommended: boolean
  installTitle: string
  installDescription: string
  installLink: string
  commands: Array<{ label: string; code: string }>
  manualSupported: boolean
  manualTitle: string
  manualHint: string
  defaultModel: string
  testLabel: string
  testCommand: string
  testDescription: string
}

const SectionHeader = defineComponent({
  props: {
    index: { type: String, required: true },
    title: { type: String, required: true },
    description: { type: String, required: true },
  },
  setup(props) {
    return () => h('div', { class: 'flex flex-col gap-2 sm:flex-row sm:items-start sm:gap-4' }, [
      h('span', { class: 'inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-sm font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' }, props.index),
      h('div', { class: 'min-w-0' }, [
        h('h3', { class: 'text-xl font-semibold text-gray-950 dark:text-white' }, props.title),
        h('p', { class: 'mt-1 text-sm leading-6 text-gray-600 dark:text-dark-300' }, props.description),
      ]),
    ])
  },
})

const CommandBlock = defineComponent({
  props: {
    label: { type: String, required: true },
    code: { type: String, required: true },
  },
  emits: ['copy'],
  setup(props, { emit }) {
    return () => h('div', { class: 'min-w-0' }, [
      h('div', { class: 'mb-2 flex items-center justify-between gap-3' }, [
        h('div', { class: 'text-xs font-medium text-gray-500 dark:text-dark-400' }, props.label),
        h('button', {
          type: 'button',
          class: 'btn btn-secondary btn-sm px-2',
          title: '复制',
          onClick: () => emit('copy', props.code),
        }, [h(Icon, { name: 'copy', size: 'xs' })]),
      ]),
      h('pre', { class: 'code-block text-xs leading-5' }, [h('code', props.code)]),
    ])
  },
})

const CopyField = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
  },
  emits: ['copy'],
  setup(props, { emit }) {
    return () => h('div', { class: 'min-w-0 rounded-xl bg-gray-50 p-3 dark:bg-dark-900/60' }, [
      h('dt', { class: 'text-xs font-medium text-gray-500 dark:text-dark-400' }, props.label),
      h('dd', { class: 'mt-2 flex min-w-0 items-center gap-2' }, [
        h('code', { class: 'min-w-0 flex-1 truncate rounded bg-white px-2 py-1 font-mono text-xs text-gray-700 dark:bg-dark-800 dark:text-dark-200' }, props.value),
        h('button', {
          type: 'button',
          class: 'btn btn-secondary btn-sm shrink-0 px-2',
          title: '复制',
          onClick: () => emit('copy', props.value),
        }, [h(Icon, { name: 'copy', size: 'xs' })]),
      ]),
    ])
  },
})

const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const selectedClientId = ref<ClientId>('codex-app')
const configMethod = ref<'cc-switch' | 'manual'>('cc-switch')
const apiKeys = ref<ApiKey[]>([])
const selectedApiKeyId = ref<number | null>(null)
const manualApiKey = ref('')
const endpointDraft = ref('')
const loadingKeys = ref(false)

const quickLinks = [
  { href: '#client', label: '选择客户端' },
  { href: '#install', label: '安装' },
  { href: '#config', label: '配置' },
  { href: '#test', label: '测试' },
  { href: '#troubleshooting', label: '错误排查' },
]

const clients: ClientGuide[] = [
  {
    id: 'codex-app',
    name: 'ChatGPT',
    icon: 'chat',
    description: '内置 Codex 的桌面应用，无需终端，适合首次使用。',
    badge: '新手推荐',
    recommended: true,
    installTitle: '下载 ChatGPT 桌面应用',
    installDescription: 'Codex 现已集成到 ChatGPT 桌面应用。下载安装后，在 ChatGPT 顶部切换器中选择 Codex，再继续往下配置。',
    installLink: 'https://learn.chatgpt.com/docs/app',
    commands: [],
    manualSupported: true,
    manualTitle: 'Codex 配置文件',
    manualHint: '保存到 Codex 使用的 config.toml，再重启 ChatGPT 桌面应用。',
    defaultModel: 'gpt-5.6-sol',
    testLabel: '桌面应用测试',
    testCommand: '在 ChatGPT 顶部切换器选择 Codex，发送：用一句话回复 OK',
    testDescription: '保存配置后回到 ChatGPT 桌面应用，选择 Codex 并发起一个简单任务。',
  },
  {
    id: 'codex-cli',
    name: 'Codex CLI',
    icon: 'terminal',
    description: '在终端中使用 Codex，适合熟悉命令行的用户。',
    badge: '需要终端',
    recommended: false,
    installTitle: '安装 Codex CLI',
    installDescription: '根据系统选择安装命令，安装完成后确认 codex 命令可用。',
    installLink: 'https://developers.openai.com/codex/cli',
    commands: [
      { label: 'macOS / Linux', code: 'curl -fsSL https://chatgpt.com/codex/install.sh | sh\ncodex --version' },
      { label: 'Windows PowerShell', code: 'irm https://chatgpt.com/codex/install.ps1 | iex\ncodex --version' },
    ],
    manualSupported: true,
    manualTitle: '写入 ~/.codex/config.toml',
    manualHint: '复制下面内容到 Codex CLI 的配置文件。',
    defaultModel: 'gpt-5.6-sol',
    testLabel: '终端测试',
    testCommand: 'codex -p "用一句话回复 OK"',
    testDescription: '配置完成后在终端运行测试命令，确认 Codex CLI 能正常请求。',
  },
  {
    id: 'claude-desktop',
    name: 'Claude Desktop',
    icon: 'chatBubble',
    description: 'Claude 桌面应用，无需终端，适合首次使用。',
    badge: '新手推荐',
    recommended: true,
    installTitle: '下载 Claude Desktop',
    installDescription: '下载安装 Claude Desktop 后，使用 CC Switch 完成 Provider 配置。',
    installLink: 'https://claude.ai/download',
    commands: [],
    manualSupported: false,
    manualTitle: 'Claude Desktop 使用图形配置',
    manualHint: 'Claude Desktop 请通过 CC Switch 配置。',
    defaultModel: 'claude-sonnet-4-5',
    testLabel: '桌面应用测试',
    testCommand: '打开 Claude Desktop，选择 CC Switch 中配置好的 Provider，发送：用一句话回复 OK',
    testDescription: '保存 CC Switch 配置后重启 Claude Desktop，并用简单消息确认连通。',
  },
  {
    id: 'claude-code',
    name: 'Claude Code CLI',
    icon: 'terminal',
    description: '在终端中使用 Claude Code，适合熟悉命令行的用户。',
    badge: '需要终端',
    recommended: false,
    installTitle: '安装 Claude Code CLI',
    installDescription: '根据系统选择安装命令，安装完成后确认 claude 命令可用。',
    installLink: 'https://code.claude.com/docs/en/overview#terminal',
    commands: [
      { label: 'macOS / Linux', code: 'curl -fsSL https://claude.ai/install.sh | bash\nclaude --version' },
      { label: 'Windows PowerShell', code: 'irm https://claude.ai/install.ps1 | iex\nclaude --version' },
      { label: 'Windows CMD', code: 'curl -fsSL https://claude.ai/install.cmd -o install.cmd && install.cmd && del install.cmd\nclaude --version' },
    ],
    manualSupported: true,
    manualTitle: 'Claude Code 环境变量',
    manualHint: '复制并在当前 shell 中执行，或写入你的 shell profile。',
    defaultModel: 'claude-sonnet-4-5',
    testLabel: '终端测试',
    testCommand: 'claude "用一句话回复 OK"',
    testDescription: '配置完成后在终端运行测试命令，确认 Claude Code 能正常请求。',
  },
  {
    id: 'grok-build',
    name: 'Grok Build CLI',
    icon: 'terminal',
    description: '在终端中使用 Grok Build，适合熟悉命令行的用户。',
    badge: '需要终端',
    recommended: false,
    installTitle: '安装 Grok Build CLI',
    installDescription: '根据系统选择安装命令，安装完成后确认 grok 命令可用。',
    installLink: 'https://docs.x.ai/build/overview',
    commands: [
      { label: 'macOS / Linux', code: 'curl -fsSL https://x.ai/cli/install.sh | bash\ngrok --version' },
      { label: 'Windows PowerShell', code: 'irm https://x.ai/cli/install.ps1 | iex\ngrok --version' },
    ],
    manualSupported: true,
    manualTitle: 'Grok Build 环境变量',
    manualHint: '复制并在当前 shell 中执行，或写入你的 shell profile。',
    defaultModel: 'grok-4.6',
    testLabel: '终端测试',
    testCommand: 'grok "用一句话回复 OK"',
    testDescription: '配置完成后在终端运行测试命令，确认 Grok Build 能正常请求。',
  },
]

const checks = [
  { title: 'Endpoint', icon: 'globe' as IconName, description: '确认 Base URL 没有多余空格，协议是 http 或 https，路径与后台展示的 API Endpoint 一致。' },
  { title: 'API Key', icon: 'key' as IconName, description: '确认密钥处于启用状态，没有过期，也没有触发余额、额度或 IP 限制。' },
  { title: '模型名称', icon: 'cpu' as IconName, description: '如果客户端指定了模型，确认模型名称存在于当前服务可用模型列表中。' },
  { title: '代理与网络', icon: 'sync' as IconName, description: '客户端如果走系统代理，请确认代理没有拦截 HTTPS 或改写请求头。' },
  { title: '配置生效', icon: 'refresh' as IconName, description: '修改配置后重启客户端或终端会话，避免仍然使用旧配置。' },
  { title: '错误日志', icon: 'document' as IconName, description: '保留客户端报错、请求时间和 Request ID，便于定位问题。' },
]

const troubleItems = [
  { title: '401 / Unauthorized', description: '通常是 API Key 填错、密钥已禁用、复制时漏字符，或客户端没有把 Authorization 头发出去。' },
  { title: '404 / model not found', description: '通常是模型名称不存在。先使用默认模型测试，通过后再切换到目标模型。' },
  { title: '429 / rate limit', description: '通常是并发、窗口限速或上游限速。降低并发后重试，必要时查看用量记录。' },
  { title: '连接超时或 DNS 错误', description: '检查 Endpoint 域名是否能从本机访问，必要时在浏览器打开 Endpoint 做基础连通性确认。' },
]

const feedbackItems = [
  '使用的客户端名称和版本',
  'Endpoint，不要发送完整 API Key',
  '大致请求时间和所在时区',
  '完整错误消息或截图',
  '使用的模型名称',
  '是否通过代理或公司网络访问',
]

const selectedClient = computed(() => {
  return clients.find((client) => client.id === selectedClientId.value) ?? clients[0]
})

const endpointBase = computed(() => {
  const value = normalizePublicEndpoint(endpointDraft.value)
  if (value) return value
  const configured = normalizePublicEndpoint(appStore.apiBaseUrl || '')
  if (configured) return configured
  const origin = getPublicOrigin()
  if (origin) return origin
  return ''
})

const selectedApiKey = computed(() => {
  return apiKeys.value.find((item) => item.id === selectedApiKeyId.value) ?? null
})

const apiKeyValue = computed(() => selectedApiKey.value?.key || manualApiKey.value.trim())

const selectedApiKeyPreview = computed(() => {
  if (selectedApiKey.value) return maskApiKey(selectedApiKey.value.key)
  return apiKeyValue.value ? maskApiKey(apiKeyValue.value) : '请选择或填写 API Key'
})

const apiKeysHint = computed(() => {
  if (loadingKeys.value) return '正在加载你的 API Key...'
  if (apiKeys.value.length) return '已自动加载你的 API Key，也可以到 API 密钥页面创建新的密钥。'
  return '没有加载到 API Key，可以先手动填写，或到 API 密钥页面创建。'
})

const providerName = computed(() => {
  if (selectedClientId.value === 'grok-build') return 'Grok 4.6 via Happy Code'
  if (selectedClientId.value === 'claude-desktop' || selectedClientId.value === 'claude-code') return 'Claude via Happy Code'
  return 'Codex via Happy Code'
})

const manualConfig = computed(() => {
  const endpoint = endpointBase.value || 'https://your-domain.example'
  const key = apiKeyValue.value || 'sk-...'
  if (selectedClientId.value === 'claude-code') {
    return `export ANTHROPIC_AUTH_TOKEN=${JSON.stringify(key)}
export ANTHROPIC_BASE_URL=${JSON.stringify(endpoint)}
export ANTHROPIC_MODEL=${JSON.stringify(selectedClient.value.defaultModel)}`
  }
  if (selectedClientId.value === 'grok-build') {
    return `export XAI_API_KEY=${JSON.stringify(key)}
export XAI_BASE_URL=${JSON.stringify(endpoint)}
export XAI_MODEL=${JSON.stringify(selectedClient.value.defaultModel)}`
  }
  return buildCodexConfig(endpoint, key)
})

const successChecks = computed(() => [
  '客户端能返回正常文本，不再提示认证失败。',
  `当前使用的 Endpoint 是 ${endpointBase.value || '你的服务地址'}。`,
  `默认模型为 ${selectedClient.value.defaultModel}，后续可以按需切换。`,
])

const copyLabel = '复制'

watch(selectedClientId, () => {
  if (!selectedClient.value.manualSupported) {
    configMethod.value = 'cc-switch'
  }
})

function buildCodexConfig(origin: string, apiKey: string, authMode: 'api-key' | 'openai' = 'api-key') {
  const auth = authMode === 'api-key'
    ? `requires_openai_auth = false
experimental_bearer_token = ${JSON.stringify(apiKey)}
http_headers = { "x-openai-actor-authorization" = "local-image-extension" }`
    : 'requires_openai_auth = true'

  return `model_provider = "code"
model = "gpt-5.6-sol"
model_reasoning_effort = "high"

[model_providers.code]
name = "code"
base_url = ${JSON.stringify(origin)}
wire_api = "responses"
${auth}`
}

async function copy(text: string) {
  await copyToClipboard(text, '已复制')
}

async function loadApiKeys() {
  loadingKeys.value = true
  try {
    const response = await keysAPI.list(1, 100, { status: 'active', sort_by: 'created_at', sort_order: 'desc' })
    apiKeys.value = response.items
    selectedApiKeyId.value = response.items[0]?.id ?? null
  } catch (error) {
    console.error('Failed to load API keys for help center:', error)
  } finally {
    loadingKeys.value = false
  }
}

onMounted(() => {
  endpointDraft.value = normalizePublicEndpoint(appStore.apiBaseUrl || '', getPublicOrigin())
  loadApiKeys()
})
</script>
