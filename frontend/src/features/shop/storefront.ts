import { safeExternalLink } from '@/utils/externalLinks'

export type StorefrontProduct = {
  id: string
  name: string
  spec: string
  description: string
  instructions: string
  price_cents: number
  // Best AI balance rewards use USD cents; price_cents uses CNY cents.
  reward_cents: number
  stock: number
}

export function formatStorefrontReward(cents: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency', currency: 'USD', minimumFractionDigits: 0, maximumFractionDigits: 2,
  }).format(cents / 100)
}

// An optional dedicated storefront URL; same-site installations work without it.
export function storefrontLink(product?: string, lookup = false): string {
  const base = safeExternalLink(import.meta.env.VITE_STOREFRONT_URL, '') || '/shop'
  const url = new URL(base, window.location.origin)
  if (product) url.searchParams.set('product', product)
  if (lookup) {
    url.searchParams.set('lookup', '1')
    url.hash = 'shop-order-lookup'
  }
  return base.startsWith('/') ? `${url.pathname}${url.search}${url.hash}` : url.href
}

export async function fetchStorefrontCatalog(signal?: AbortSignal): Promise<{
  products: StorefrontProduct[]
  enabled: boolean
}> {
  const [catalog, status] = await Promise.all([
    fetch('/api/v1/cdkshop/products', { cache: 'no-store', signal }),
    fetch('/api/v1/cdkshop/status', { cache: 'no-store', signal }),
  ])
  if (!catalog.ok || !status.ok) throw new Error('catalog_unavailable')
  const [items, availability] = await Promise.all([catalog.json(), status.json()])
  return { products: items.items ?? [], enabled: availability.enabled === true }
}
