import type { ExternalToolKind } from './api'

export const resourceGroups = [
  { id: 'quality', kinds: ['channel', 'ranking', 'benchmark', 'status', 'pricing'] },
  { id: 'business', kinds: ['upstream', 'payment', 'email', 'infrastructure', 'operations'] },
  { id: 'product', kinds: ['marketplace', 'creative'] },
] as const satisfies ReadonlyArray<{ id: string; kinds: readonly ExternalToolKind[] }>

export type ResourceGroup = 'all' | typeof resourceGroups[number]['id']
export const resourceKinds = resourceGroups.flatMap(group => [...group.kinds])

export function resourceGroup(kind: ExternalToolKind): ResourceGroup {
  return resourceGroups.find(group => (group.kinds as readonly string[]).includes(kind))?.id ?? 'all'
}

export function safeResourceURL(value: string) {
  try {
    const url = new URL(value)
    return ['http:', 'https:'].includes(url.protocol) && !!url.hostname && !url.username && !url.password && !/[\r\n\t]/.test(value)
  } catch { return false }
}
