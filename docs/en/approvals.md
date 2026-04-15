# Approval System

### USD Thresholds

Each wallet can have a single USD-denominated approval policy. When a transfer or contract call exceeds the threshold, it enters a pending state requiring Passkey approval.

```bash
curl -s -X PUT ${TEE_WALLET_URL}/api/wallets/WALLET_ID/policy \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "threshold_usd": "100",
    "daily_limit_usd": "5000",
    "enabled": true
  }'
```

- `threshold_usd`: Transactions above this USD value require Passkey approval.
- `daily_limit_usd` (optional): Cumulative USD spend per UTC calendar day. Transfers that would exceed this limit are hard-blocked (no approval path).
- `enabled`: Enable or disable the policy.

**Price conversion:**
- Native currencies (ETH, SOL, BNB, etc.) are priced via CoinGecko API with a 10-second cache.
- Stablecoins (USDC, USDT, DAI, BUSD) are hardcoded at $1.
- ERC-20 tokens are priced via CoinGecko token price API by contract address.
- Solana SPL tokens use CoinGecko first, then fall back to Jupiter Price API.
- If a price is unavailable, the transaction requires approval (fail-closed).
- View current prices: `GET /api/prices`.

**View the current policy:**

```bash
curl -s ${TEE_WALLET_URL}/api/wallets/WALLET_ID/policy \
  -H "Authorization: Bearer ${API_KEY}"
```

**Delete a policy** (Passkey only):

```bash
curl -s -X DELETE ${TEE_WALLET_URL}/api/wallets/WALLET_ID/policy \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}"
```

### Daily Limits

Daily limits use a pre-deduction (auth/capture) pattern:

1. Before signing, the wallet pre-deducts the USD amount from the daily budget.
2. If signing or broadcast fails, the pre-deduction is rolled back.
3. This prevents phantom spend from failed transactions while ensuring the limit is enforced even under concurrent requests.

The daily limit resets at UTC midnight.

**Check current daily spend:**

```bash
curl -s ${TEE_WALLET_URL}/api/wallets/WALLET_ID/daily-spent \
  -H "Authorization: Bearer ${API_KEY}"
```

Response:

```json
{
  "daily_spent_usd": "235.50",
  "daily_limit_usd": "5000",
  "remaining_usd": "4764.50",
  "reset_at": "2026-03-27T00:00:00Z"
}
```

If no policy is set, all fields return empty strings except `daily_spent_usd` which returns `"0"`.

### Approval Flow

When a request triggers an approval:

1. The API returns `"status": "pending_approval"` with an `approval_id`.
2. The human owner opens the approval link in the web UI.
3. The web UI prompts for a fresh Passkey assertion (hardware-bound -- a stolen session token cannot approve).
4. After approval, the wallet completes the transaction (signs and broadcasts).

**List pending approvals:**

```bash
curl -s ${TEE_WALLET_URL}/api/approvals/pending \
  -H "Authorization: Bearer ${API_KEY}"
```

**Get approval details:**

```bash
curl -s ${TEE_WALLET_URL}/api/approvals/APPROVAL_ID \
  -H "Authorization: Bearer ${API_KEY}"
```

**Approve a request** (Passkey only):

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/approvals/APPROVAL_ID/approve \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}"
```

**Reject a request** (Passkey only):

```bash
curl -s -X POST ${TEE_WALLET_URL}/api/approvals/APPROVAL_ID/reject \
  -H "Authorization: Bearer ps_${SESSION_TOKEN}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}"
```

Pending approvals expire after the configured timeout (default: 24 hours).

**Approval types:**

| Type | Trigger |
|------|---------|
| `transfer` | Transfer exceeds USD threshold |
| `sign` | Sign request exceeds threshold |
| `contract_call` | Contract call via API key (all contract calls require approval) |
| `policy_change` | Policy set via API key |
| `contract_add` | Contract whitelist addition via API key |
| `contract_update` | Contract whitelist update via API key |
| `wrap_sol` | Wrap SOL exceeds threshold |
| `unwrap_sol` | Unwrap SOL exceeds threshold |
| `addressbook_add` | Address book entry addition via API key |
| `addressbook_update` | Address book entry update via API key |

### Contract Call Approvals

All contract calls made via API key require Passkey approval. This is by design -- contract interactions carry higher risk than simple transfers, and the wallet enforces human-in-the-loop confirmation for every contract call initiated by an agent.

Contract calls made via a Passkey session execute immediately (the human is already present).

The convenience endpoints `approve-token` and `revoke-approval` also always require Passkey approval because they grant or revoke third-party spending access.
