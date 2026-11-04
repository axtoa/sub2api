-- Account access tokens are stored only as authenticated ciphertext, never in
-- wallet config, operation responses, or audit JSON.
ALTER TABLE upstream_wallets ADD COLUMN IF NOT EXISTS access_token_encrypted TEXT NOT NULL DEFAULT '';
