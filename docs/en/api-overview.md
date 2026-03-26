# API Reference

> Full OpenAPI specification is available at [openapi.yaml](../api/openapi.yaml)

## Supported Chains

The following chains are supported out of the box via `chains.json`:

| Chain | Name (API) | Currency | Protocol | Curve | Family |
|-------|------------|----------|----------|-------|--------|
| Ethereum Mainnet | `ethereum` | ETH | ECDSA | secp256k1 | EVM |
| Optimism Mainnet | `optimism` | ETH | ECDSA | secp256k1 | EVM |
| Sepolia Testnet | `sepolia` | ETH | ECDSA | secp256k1 | EVM |
| Holesky Testnet | `holesky` | ETH | ECDSA | secp256k1 | EVM |
| Base Sepolia Testnet | `base-sepolia` | ETH | ECDSA | secp256k1 | EVM |
| BSC Testnet | `bsc-testnet` | tBNB | ECDSA | secp256k1 | EVM |
| Solana Mainnet | `solana` | SOL | Schnorr | ed25519 | Solana |
| Solana Devnet | `solana-devnet` | SOL | Schnorr | ed25519 | Solana |

Any EVM-compatible chain can be added at runtime via `POST /api/chains` by providing a name, RPC URL, currency, and optional chain ID. Custom chains use ECDSA on secp256k1.

**Common ERC-20 token addresses:**

Ethereum Mainnet:

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48` | 6 |
| USDT | `0xdac17f958d2ee523a2206206994597c13d831ec7` | 6 |
| WETH | `0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2` | 18 |
| DAI | `0x6b175474e89094c44da98b954eedeac495271d0f` | 18 |

Sepolia Testnet:

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238` | 6 |
| WETH | `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14` | 18 |
| LINK | `0x779877A7B0D9E8603169DdbD7836e478b4624789` | 18 |

Base Sepolia Testnet:

| Token | Contract | Decimals |
|-------|----------|----------|
| USDC | `0x036CbD53842c5426634e7929541eC2318f3dCF7e` | 6 |
| WETH | `0x4200000000000000000000000000000000000006` | 18 |

---

## Error Reference

### Common Errors and Solutions

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
| `rate limit exceeded` | Too many requests in the current time window | Wait and retry. Default limits: 200 requests/min per API key, 5 wallet creations/min, 10 registrations/min per IP. |
| `CSRF token required` | A Passkey session request is missing the `X-CSRF-Token` header | Add `X-CSRF-Token: nocheck` (or any non-empty value) to state-changing requests. |
| `passkey session required` | The operation requires Passkey auth but was called with an API key | Use a Passkey session for this operation (wallet deletion, policy deletion, contract removal, approve/reject). |
| `max wallets reached` | User has reached the `MAX_WALLETS_PER_USER` limit | Delete unused wallets or increase the limit in the server configuration. |

### HTTP Status Codes

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

---

## Audit Log

All wallet operations are recorded in an audit log. Query it with:

```bash
curl -s "${TEE_WALLET_URL}/api/audit/logs?page=1&limit=20" \
  -H "Authorization: Bearer ${API_KEY}"
```

**Query parameters:**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page` | `1` | Page number |
| `limit` | `20` | Results per page (max: 100) |
| `action` | _(all)_ | Filter by action type |
| `wallet_id` | _(all)_ | Filter by wallet |

**Action types:**

| Action | Description |
|--------|-------------|
| `login` | Passkey login |
| `wallet_create` | Wallet created |
| `wallet_delete` | Wallet deleted |
| `transfer` | Transfer sent or pending |
| `sign` | Message signed or pending |
| `policy_update` | Approval policy set or pending |
| `approval_approve` | Approval request approved |
| `approval_reject` | Approval request rejected |
| `contract_add` | Contract added to whitelist or pending |
| `wrap_sol` | SOL wrapped into wSOL |
| `unwrap_sol` | wSOL unwrapped to SOL |
| `apikey_generate` | API key generated |
| `apikey_revoke` | API key revoked |

---

## Security Summary

| Layer | Mechanism |
|-------|-----------|
| Key storage | Private keys split across 3-5 TEE nodes via threshold cryptography (FROST/GG20). No single node holds the full key. |
| Signing | M-of-N threshold signing. The private key is never reconstructed. |
| Human approval | High-value transactions and sensitive operations require fresh WebAuthn/Passkey hardware assertion. |
| Contract security | Address whitelist + mandatory Passkey approval for all contract calls via API key. |
| Spend control | USD-denominated thresholds and daily limits with auth/capture pattern. |
| API protection | Per-key rate limiting, CSRF protection for browser sessions, invite-based registration. |
| Transport | Mutual TLS between wallet service and TEE-DAO cluster. |
| Data | SQLite with WAL mode, structured audit logging, Content Security Policy headers. |

---
[Previous: AI Agent Integration](agent-integration.md)
