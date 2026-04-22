# Error Codes & Status Codes

## Common Errors and Solutions

| Error Message | Cause | Solution |
|---------------|-------|----------|
| `insufficient funds` | Wallet balance too low for the transfer plus gas | Check balance with `GET /api/wallets/:id/balance`. For ETH transfers, allow approximately 0.0005 ETH for gas. |
| `daily spend limit exceeded` | Cumulative USD spend for the day has reached the daily limit | Wait until UTC midnight for the limit to reset, or adjust the policy via Passkey. |
| `contract not whitelisted` | The contract address, token mint, or program ID is not in the wallet's whitelist | Add it via `POST /api/wallets/:id/contracts` (API key creates a pending approval) or through the web UI for instant approval. |
| `contract operations require passkey approval` | Contract call via API key requires human confirmation | The wallet owner must approve the pending request via Passkey in the web UI. |
| `wallet is not ready` | The wallet is still in the `creating` state (DKG in progress) | Wait 1-2 minutes for ECDSA key generation to complete, then retry. |
| `invalid API key` | The provided API key is not valid or has been revoked | Verify the `Authorization` header value. Generate a new key if needed. |
| `approval has expired` | The pending approval was not acted on within the expiry window (default: 24 hours) | Initiate the operation again to create a new approval request. |
| `cannot overwrite a built-in chain` | Attempted to create a custom chain with the same name as a built-in chain | Choose a different name for the custom chain. |
| `chain has existing wallets; delete them first` | Attempted to delete a custom chain that still has wallets | Delete all wallets on the chain before removing it. |
| `rate limit exceeded` | Too many requests in the current time window | Wait and retry. Default limits: 100 requests/min per API key, 50 total RPC calls/min per user (reads + writes share this bucket), 5 wallet creations/min, 10 registrations/min per IP. |
| `invalid CSRF token` | The request used a missing, stale, or incorrect `X-CSRF-Token` for the current Passkey session | Reuse the exact `csrf_token` returned by login in the `X-CSRF-Token` header on state-changing Passkey requests. |
| `passkey session required` | The operation requires Passkey auth but was called with an API key | Use a Passkey session for this operation (wallet deletion, policy deletion, contract removal, approve/reject). |
| `max wallets reached` | User has reached the `MAX_WALLETS_PER_USER` limit | Delete unused wallets or increase the limit in the server configuration. |

## HTTP Status Codes

| Code | Meaning |
|------|---------|
| `200` | Success |
| `201` | Resource created |
| `202` | Request accepted; pending Passkey approval |
| `400` | Invalid request (missing fields, bad format) |
| `401` | Authentication required or invalid credentials |
| `403` | Forbidden (e.g., attempting to delete a built-in chain) |
| `404` | Resource not found |
| `409` | Conflict (e.g., duplicate chain name, wallets exist on chain) |
| `429` | Rate limit exceeded |
| `500` | Internal server error |
