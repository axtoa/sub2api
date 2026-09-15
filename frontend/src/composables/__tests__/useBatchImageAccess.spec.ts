import { describe, expect, it } from 'vitest'
import type { ApiKey, GroupPlatform } from '@/types'
import { keyAllowsBatchImage, keyAllowsCreativeVideo } from '../useBatchImageAccess'

function makeKey(
  platform: GroupPlatform,
  options: { status?: ApiKey['status']; allowBatchImage?: boolean } = {},
): ApiKey {
  return {
    status: options.status ?? 'active',
    group: {
      platform,
      allow_batch_image_generation: options.allowBatchImage ?? false,
    },
  } as ApiKey
}

describe('creative studio access predicates', () => {
  it('keeps batch image access restricted to active Gemini groups', () => {
    expect(keyAllowsBatchImage(makeKey('gemini', { allowBatchImage: true }))).toBe(true)
    expect(keyAllowsBatchImage(makeKey('grok', { allowBatchImage: true }))).toBe(false)
    expect(keyAllowsBatchImage(makeKey('gemini', { status: 'inactive', allowBatchImage: true }))).toBe(false)
  })

  it('allows the studio for active Grok and Composite groups', () => {
    expect(keyAllowsCreativeVideo(makeKey('grok'))).toBe(true)
    expect(keyAllowsCreativeVideo(makeKey('composite'))).toBe(true)
    expect(keyAllowsCreativeVideo(makeKey('gemini'))).toBe(false)
    expect(keyAllowsCreativeVideo(makeKey('grok', { status: 'inactive' }))).toBe(false)
  })
})
