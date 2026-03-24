# Authentication

TEENet Wallet uses a dual authentication model. Every protected endpoint accepts either type of credential in the `Authorization` header.

### API Keys

API keys are intended for AI agents, bots, and automated pipelines. They are prefixed with `ocw_`.

**Generating a key** (requires Passkey session):

```bash
curl -s -X POST http://localhost:8080/api/auth/apikey/generate \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck" \
  -H "Content-Type: application/json" \
  -d '{"label": "production-agent"}'
```

**Using a key:**

```bash
curl -s http://localhost:8080/api/wallets \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Listing keys:**

```bash
curl -s http://localhost:8080/api/auth/apikey/list \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

**Revoking a key:**

```bash
curl -s -X DELETE http://localhost:8080/api/auth/apikey \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck" \
  -H "Content-Type: application/json" \
  -d '{"key_id": "KEY_ID_HERE"}'
```

API keys can perform most operations directly. However, certain sensitive operations (wallet deletion, policy deletion, contract removal, approval/reject actions) require a Passkey session. When an API key attempts to set a policy or add a contract to the whitelist, the request creates a pending approval that the Passkey owner must confirm.

**Rate limiting:** Each API key is limited to a configurable number of requests per minute (default: 200). Wallet creation has a separate, lower limit (default: 5 per minute) because TEE distributed key generation is computationally expensive.

### Passkey Sessions

Passkey sessions use the WebAuthn standard for hardware-bound authentication. They are prefixed with `ps_`.

**Registration flow:**

1. `POST /api/auth/passkey/register/begin` -- begin open registration (returns a challenge)
2. Complete the WebAuthn ceremony in the browser
3. `POST /api/auth/passkey/register/verify` -- submit the attestation

**Login flow:**

1. `GET /api/auth/passkey/options` -- get a login challenge
2. Complete the WebAuthn assertion in the browser
3. `POST /api/auth/passkey/verify` -- submit the assertion and receive a session token

Passkey sessions are required for:
- Generating, listing, and revoking API keys
- Deleting wallets
- Deleting approval policies
- Removing contracts from the whitelist
- Approving or rejecting pending approval requests
- Deleting the user account
- Adding or removing custom chains

### CSRF Protection

All state-changing requests made with a Passkey session must include the `X-CSRF-Token` header. Any non-empty value is accepted (e.g., `nocheck`). This prevents cross-site request forgery attacks against browser-based sessions.

```bash
curl -s -X POST http://localhost:8080/api/wallets \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck" \
  -H "Content-Type: application/json" \
  -d '{"chain": "ethereum", "label": "My Wallet"}'
```

API key requests do not require the CSRF header.

---
[Previous: Configuration](configuration.md) | [Next: Wallet Management](wallets.md)
