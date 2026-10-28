# Subscription Shop

This module sells assisted subscriptions for users' own ChatGPT and Anthropic
accounts. It is independent of gateway plans, balances, token usage and billing.
There is no payment callback, automatic activation, session collection, anonymous
checkout or transfer of gateway credit.

## Routes and Ownership

- `/shop`: catalogue and checkout behind the existing login. The homepage keeps
  its compact public product cards; its read-only catalogue API stays public.
- `/shop/orders` and `/shop/orders/:id`: the authenticated user's own orders.
- `/admin/shop`: all shop orders, behind the existing administrator guard.
- `/admin/shop/products`: product configuration, behind the same guard.
- APIs use `/api/v1/shop/*` and `/api/v1/admin/shop/*` respectively.

Backend logic lives in `backend/internal/shop`, frontend pages and styles in
`frontend/src/features/shop`. Integration points are route registration, handler
assembly, locales, the homepage and sidebar entries. Existing gateway subscription
APIs are not reused or changed.

Homepage product cards open a new tab with an expiring, tab-local purchasing
intent. The existing password, registration/email verification, two-factor and
OAuth flows keep their authentication logic; the router returns this tab to the
selected product after successful authentication. A one-time, non-sensitive
completion signal asks only the originating homepage tab to enter the console.
That tab verifies the shared session against the server first; it never trusts a
notification as authentication. Ordinary logins and other open tabs are unaffected.
Leaving the authentication journey cancels the intent. A missing/closed original
tab does not prevent checkout. Failure to preserve/notify this optional intent
does not change the existing authentication requirements.

## Database and Configuration

Migration `242_subscription_shop.sql` adds `shop_products` and `shop_orders` to
the existing PostgreSQL database. Startup applies it through the normal migration
runner. No existing migration is edited. Product and order payloads use JSONB,
with separate ownership, creation time, request-key and version columns.

Seed products have no selling price, reference price or external payment URL.
They appear as coming soon until an administrator supplies a positive CNY price
and HTTPS checkout URL. Configure account requirements, purchase/after-sales terms
and delivery estimates before sale. Reference prices additionally require a source
URL and check date; verify the actual plans and regional prices before publishing.

The server reads the applicant's email from the authenticated user record.
An empty target or contact email defaults to that account email. Orders retain
the product, price, terms and external checkout URL from the time of ordering.
Product edits or hiding a product do not alter previous orders.

The switch at the top of shop management persists `official_shop_enabled` in
the existing `settings` table. Migration `250_shop_entries_default_closed.sql`
closes the on-site, external, and homepage shop entries on upgrade. Missing
settings also resolve to disabled. Admin-only GET/PUT
`/api/v1/admin/shop/settings` reads/changes it. Disabling hides the on-site
customer sidebar entry and blocks public catalogue/new order requests. The
homepage promotion has its own switch and appears only when at least one shop
channel is enabled.
Existing orders, payment references, manual fulfillment and idempotent order
retries remain available; administrator management stays accessible to reopen.
New-order transactions hold a shared lock on the switch until commit, so closing
waits for already-started creations. Setting read failures block new orders;
failed writes preserve the previous state. A successful update invalidates the
existing HTML settings cache. Browser tabs
refresh public settings on focus or a local cross-tab change notification.

## Manual Payment and Delivery

1. The user creates an order after accepting that product's terms.
2. The user opens external checkout and can submit its external order reference.
3. Staff checks the external payment, amount, account and duplicate receipts.
4. Staff explicitly confirms payment, starts processing and records delivery.

Submitting a reference or opening checkout never marks an order as paid. An
external payment arriving after local cancellation can be reconciled by an
administrator. Refund actions only record refunds already made externally; they
never move money. Terminal orders are kept as history, not physically deleted.

Creation uses a per-user transaction advisory lock and unique request key.
Retries with the same key and same normalized payload return the original order.
A changed payload, stale product version or unfinished duplicate order is rejected.
Product and order actions use version checks; clients must refresh and review a
conflict rather than silently overwrite concurrent changes.

## Verification

Run frontend feature, router, homepage, sidebar and locale tests with Vitest,
then typecheck and build. Run backend shop, route and server tests with `go test`.
The PostgreSQL integration test is opt-in:

```sh
SHOP_TEST_DATABASE_URL='postgres://user:password@127.0.0.1:5432/shop_test?sslmode=disable' \
  go test -tags integration ./internal/shop -run TestPostgresShopMigrationAndConcurrentOrders -count=1
```

Use a dedicated test database. The test creates and removes a uniquely named
schema and exercises the actual migration, concurrent retries, ownership checks,
price snapshots, manual fulfillment and preservation of gateway credit.

## Before Release

- Temporary local administrator preview bootstrap, mock API adapter and fixtures
  have been removed. All environments use the existing real authentication.
- Verify a production build contains no administrator preview identifiers or fixtures.
- Verify unauthenticated order access and non-admin management access are denied.
- Review actual product prices, terms and external links with the operator.
- Back up the database before deployment. Rolling back the binary need not remove
  the additional shop tables; never delete orders as part of a rollback.

Browser visual verification remains separate from unit and database tests. Review
the homepage cards, standalone pages and forms in light/dark and mobile layouts.
