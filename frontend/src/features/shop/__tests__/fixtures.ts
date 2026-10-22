import type { ShopOrder, ShopProduct } from '../api'
export function productFixture(): ShopProduct {
  return { id: '88000000-0000-4000-8000-000000000002', brand: 'ChatGPT', name: 'Plus', spec: 'Monthly', description: 'Official product subscription', requirements: 'An eligible account', purchase_notice: 'First condition\nSecond condition', delivery_hours: 24, price_cents: 12300, payment_url: 'https://shop.example.com/plus', reference_amount: 20, reference_currency: 'USD', reference_url: 'https://chatgpt.com/pricing', reference_date: '2026-10-22', active: true, sort: 20, version: 3 }
}
export function orderFixture(): ShopOrder {
  return { id: 'SOtest', user_id: 42, applicant_email: 'member@example.com', target_email: 'target@example.com', contact_email: 'member@example.com', product: productFixture(), amount_cents: 12300, payment_status: 'PENDING', fulfillment_status: 'WAITING', external_reference: '', note: '', delivery_note: '', created_at: '2026-10-22T04:00:00Z', valid_until: '', version: 1, events: [{ actor_id: 42, action: 'created', note: '', at: '2026-10-22T04:00:00Z' }] }
}
