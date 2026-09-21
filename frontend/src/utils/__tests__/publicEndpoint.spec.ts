import { describe, expect, it } from 'vitest'
import { normalizePublicEndpoint } from '@/utils/publicEndpoint'

describe('publicEndpoint utils', () => {
  it('converts public HTTP endpoints to HTTPS and removes trailing slashes', () => {
    expect(normalizePublicEndpoint('  http://api.example.com/v1/// ')).toBe(
      'https://api.example.com/v1'
    )
  })

  it('keeps HTTPS endpoints and their path intact', () => {
    expect(normalizePublicEndpoint('https://api.example.com/v1/')).toBe(
      'https://api.example.com/v1'
    )
  })

  it('uses and normalizes the fallback when the configured value is empty', () => {
    expect(normalizePublicEndpoint('', 'http://api.example.com/')).toBe(
      'https://api.example.com'
    )
  })
})
