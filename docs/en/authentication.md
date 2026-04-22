# Authentication

TEENet Wallet uses a dual authentication model. Many wallet endpoints accept either an API key or a Passkey session in the `Authorization` header, while sensitive account-management and destructive operations are Passkey-only.

### API Keys

API keys are intended for AI agents, bots, and automated pipelines. They are prefixed with `ocw_`.

**Generating a key** (requires Passkey session):

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/auth/apikey/generate \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"label": "production-agent"}'
```

**Using a key:**

```bash
curl -s ${TEE_WALLET_URL}/api/wallets \
  -H "Authorization: Bearer ${API_KEY}"
```

**Listing keys:**

```bash
curl -s ${TEE_WALLET_URL}/api/auth/apikey/list \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}"
```

**Renaming a key:**

```bash
curl -s -X PATCH ${TEE_WALLET_URL}/api/auth/apikey \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"prefix": "ocw_a1b2c3d4", "label": "new-label"}'
```

**Revoking a key:**

```bash
curl -s -X DELETE ${TEE_WALLET_URL}/api/auth/apikey?prefix=ocw_a1b2c3d4 \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}"
```

API keys can perform most operations directly. However, certain sensitive operations (wallet deletion, policy deletion, contract removal, approval/reject actions) require a Passkey session. When an API key attempts to set a policy or add a contract to the whitelist, the request creates a pending approval that the Passkey owner must confirm.

**Rate limiting:** Each API key is limited to a configurable number of requests per minute (default: 100). Wallet creation has a separate, lower limit (default: 5 per minute) because TEE distributed key generation is computationally expensive. Endpoints that hit upstream RPC share a per-user cap of 50/min (`RPC_RATE_LIMIT`) covering both reads (`/call-read`, `/balance`) and fund-moving ops (`/transfer`, `/contract-call`, `/approve-token`, `/revoke-approval`, `/wrap-sol`, `/unwrap-sol`).

### Passkey Sessions

Passkey sessions use the WebAuthn standard for hardware-bound authentication. They are prefixed with `ps_`.

**Registration flow:**

1. `POST /api/auth/passkey/register/begin` -- begin open registration (returns a challenge)
2. Complete the WebAuthn ceremony in the browser
3. `POST /api/auth/passkey/register/verify` -- submit the attestation

**Login flow:**

1. `GET /api/auth/passkey/options` -- get a login challenge
2. Complete the WebAuthn assertion in the browser
3. `POST /api/auth/passkey/verify` -- submit the assertion and receive a session token plus `csrf_token`

Passkey sessions are required for:
- Generating, renaming, and revoking API keys
- Deleting wallets
- Deleting approval policies
- Removing contracts from the whitelist
- Approving or rejecting pending approval requests
- Inviting new users (admin deployments)
- Deleting the user account
- Deleting address book entries
- Adding or removing custom chains

### CSRF Protection

All state-changing requests made with a Passkey session must include the `X-CSRF-Token` header with the exact `csrf_token` returned at login. This prevents cross-site request forgery attacks against browser-based sessions.

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/wallets \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"chain": "ethereum", "label": "My Wallet"}'
```

API key requests do not require the CSRF header.
