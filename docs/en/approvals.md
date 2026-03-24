# Approval System

### USD Thresholds

Each wallet can have a single USD-denominated approval policy. When a transfer or contract call exceeds the threshold, it enters a pending state requiring Passkey approval.

```bash
curl -s -X PUT http://localhost:8080/api/wallets/WALLET_ID/policy \
  -H "Authorization: Bearer ocw_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "threshold_usd": "100",
    "daily_limit_usd": "5000",
    "enabled": true
  }'
```

- `threshold_usd`: Transactions above this USD value require Passkey approval.
- `daily_limit_usd` (optional): Cumulative USD spend per UTC calendar day. Transfers that would exceed this limit are hard-blocked (no approval path).
- The policy covers all currencies on the wallet (ETH, SOL, tokens). Real-time prices from CoinGecko are used for conversion; stablecoins are pegged at $1.

**View the current policy:**

```bash
curl -s http://localhost:8080/api/wallets/WALLET_ID/policy \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Delete a policy** (Passkey only):

```bash
curl -s -X DELETE http://localhost:8080/api/wallets/WALLET_ID/policy \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

### Daily Limits

Daily limits use a pre-deduction (auth/capture) pattern:

1. Before signing, the wallet pre-deducts the USD amount from the daily budget.
2. If signing or broadcast fails, the pre-deduction is rolled back.
3. This prevents phantom spend from failed transactions while ensuring the limit is enforced even under concurrent requests.

The daily limit resets at UTC midnight.

### Approval Flow

When a request triggers an approval:

1. The API returns `"status": "pending_approval"` with an `approval_id`.
2. The human owner opens the approval link in the web UI.
3. The web UI prompts for a fresh Passkey assertion (hardware-bound -- a stolen session token cannot approve).
4. After approval, the wallet completes the transaction (signs and broadcasts).

**List pending approvals:**

```bash
curl -s http://localhost:8080/api/approvals/pending \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Get approval details:**

```bash
curl -s http://localhost:8080/api/approvals/APPROVAL_ID \
  -H "Authorization: Bearer ocw_YOUR_API_KEY"
```

**Approve a request** (Passkey only):

```bash
curl -s -X POST http://localhost:8080/api/approvals/APPROVAL_ID/approve \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

**Reject a request** (Passkey only):

```bash
curl -s -X POST http://localhost:8080/api/approvals/APPROVAL_ID/reject \
  -H "Authorization: Bearer ps_YOUR_SESSION_TOKEN" \
  -H "X-CSRF-Token: nocheck"
```

Pending approvals expire after the configured timeout (default: 30 minutes).

**Approval types:**

| Type | Trigger |
|------|---------|
| `transfer` | Transfer exceeds USD threshold |
| `sign` | Sign request exceeds threshold |
| `contract_call` | Contract call exceeds threshold or is a high-risk method |
| `policy_change` | Policy set via API key |
| `contract_add` | Contract whitelist addition via API key |
| `contract_update` | Contract whitelist update via API key |
| `wrap_sol` | Wrap SOL exceeds threshold |
| `unwrap_sol` | Unwrap SOL exceeds threshold |

### High-Risk Methods

The following EVM methods always require Passkey approval, regardless of auto-approve settings or threshold policies:

- `approve`
- `transferFrom`
- `increaseAllowance`
- `setApprovalForAll`
- `safeTransferFrom`

These methods grant third parties access to wallet funds and are therefore treated as unconditionally sensitive.

---
[Previous: Smart Contracts](smart-contracts.md) | [Next: AI Agent Integration](agent-integration.md)
